package client

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
)

func newTestClient() *Client {
	return NewClientWithProxy("节点1（推荐）", "")
}

// ── parseSelectResult ────────────────────────────────────────────────────────
// V3.0 用 `Flag string` 接收，正方返回数字 1/-1 时 Unmarshal 直接失败 →
// 即使真的选上了也判失败并无限重提。以下用例锁住"数字/字符串/布尔都能识别"。

func TestParseSelectResult_FlagTypes(t *testing.T) {
	c := newTestClient()

	cases := []struct {
		name      string
		resp      string
		wantOK    bool
		wantRetry bool
	}{
		{"数字 1", `{"flag":1,"message":"选课成功"}`, true, false},
		{"字符串 1", `{"flag":"1","message":"ok"}`, true, false},
		{"布尔 true", `{"flag":true}`, true, false},
		{"数字 -1 待重试", `{"flag":-1,"message":"请稍后重试"}`, false, true},
		{"字符串 -1 待重试", `{"flag":"-1"}`, false, true},
		{"数字 0 失败", `{"flag":0,"message":"课程已满"}`, false, false},
		{"缺少 flag", `{"message":"未知"}`, false, false},
		{"HTML 响应", `<html><body>登录</body></html>`, false, false},
		{"空响应", ``, false, false},
	}

	for _, tc := range cases {
		err := c.parseSelectResult(tc.resp)
		if tc.wantOK {
			if err != nil {
				t.Errorf("%s: 期望成功，got %v", tc.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: 期望失败，got nil", tc.name)
			continue
		}
		if got := errors.Is(err, ErrSelectRetry); got != tc.wantRetry {
			t.Errorf("%s: ErrSelectRetry=%v, want %v (err=%v)", tc.name, got, tc.wantRetry, err)
		}
	}
}

func TestParseSelectResult_MessageKeys(t *testing.T) {
	c := newTestClient()
	// message 缺失时应回落到 msg（不同部署字段名不同）
	err := c.parseSelectResult(`{"flag":0,"msg":"容量已满"}`)
	if err == nil || !strings.Contains(err.Error(), "容量已满") {
		t.Errorf("应能识别 msg 字段, got %v", err)
	}
}

// ── buildSelectParams ────────────────────────────────────────────────────────
// V3.0 只提交 5 个字段，缺 xkkz_id 等页面下发的初始化参数。
// 现在应"整体回填页面参数 + 用课程自身字段覆盖"。

func TestBuildSelectParams_MergesInitParamsAndOverrides(t *testing.T) {
	c := newTestClient()
	c.SetSelectInitParams(map[string]string{
		"xkkz_id":   "XK-1",
		"rwlx":      "1",
		"kklxdm":    "10", // 页面值：应被课程自身的 kklxdm 覆盖
		"kcgs_list": "01,02",
	})

	course := &model.Course{
		ID: "K1", Name: "羽毛球", Type: "20", ClassID: "SHORT",
		Extra: &model.CourseExtra{DoJxbID: "LONG-ID"},
	}
	p := c.buildSelectParams(course)

	if p["jxb_ids"] != "LONG-ID" {
		t.Errorf("应优先使用 do_jxb_id, got %q", p["jxb_ids"])
	}
	if p["kklxdm"] != "20" {
		t.Errorf("课程自身类别应覆盖页面值, got %q", p["kklxdm"])
	}
	if p["xkkz_id"] != "XK-1" {
		t.Errorf("页面下发的初始化参数必须保留, got %q", p["xkkz_id"])
	}
	if p["rwlx"] != "1" {
		t.Errorf("页面下发的开关参数必须保留, got %q", p["rwlx"])
	}
	if p["kch_id"] != "K1" || p["kcmc"] != "羽毛球" {
		t.Errorf("课程字段未正确写入: %+v", p)
	}
	if p["gnmkdm"] != gnmkdmSelect {
		t.Errorf("gnmkdm = %q, want %q", p["gnmkdm"], gnmkdmSelect)
	}
}

func TestBuildSelectParams_DegradesToShortJxbID(t *testing.T) {
	c := newTestClient()

	// Extra 缺失（详情接口失败）→ 降级用短 jxb_id，而不是放弃这门课
	if got := c.buildSelectParams(&model.Course{ID: "K2", ClassID: "SHORT"})["jxb_ids"]; got != "SHORT" {
		t.Errorf("jxb_ids = %q, want SHORT", got)
	}
	// Extra 存在但 DoJxbID 为空 → 同样降级
	course := &model.Course{ID: "K3", ClassID: "S3", Extra: &model.CourseExtra{}}
	if got := c.buildSelectParams(course)["jxb_ids"]; got != "S3" {
		t.Errorf("jxb_ids = %q, want S3", got)
	}
}

func TestBuildSelectParams_NoCachedParamsShouldNotPanic(t *testing.T) {
	c := newTestClient() // 未调用 SetSelectInitParams
	p := c.buildSelectParams(&model.Course{ID: "K4", ClassID: "S4"})
	if p["jxb_ids"] != "S4" || p["kch_id"] != "K4" {
		t.Errorf("缓存为空时仍应能构建最小参数集, got %+v", p)
	}
}

func TestSelectInitParamsIsCopy(t *testing.T) {
	c := newTestClient()
	src := map[string]string{"a": "1"}
	c.SetSelectInitParams(src)
	src["a"] = "mutated" // 修改外部 map 不应影响缓存

	if got := c.SelectInitParams()["a"]; got != "1" {
		t.Errorf("缓存应做深拷贝, got %q", got)
	}
	// 取出的副本被修改也不应影响缓存
	out := c.SelectInitParams()
	out["a"] = "mutated2"
	if got := c.SelectInitParams()["a"]; got != "1" {
		t.Errorf("返回值应为副本, got %q", got)
	}

	keys := c.SelectInitParamKeys()
	if len(keys) != 1 || keys[0] != "a" {
		t.Errorf("SelectInitParamKeys = %v", keys)
	}
}
