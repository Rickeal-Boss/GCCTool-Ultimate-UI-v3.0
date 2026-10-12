package client

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/stealth"
)

// ─────────────────────────────────────────────────────────────────────────────
// GetClassList 全链路测试（httptest 模拟正方 V9 四个端点）
//
// 覆盖 2026-09-15 / 2026-10-12 实机结论：
//  1. Display 必须浏览器同款 7 参数；
//  2. PartDisplay 必须带 xkkz_xh + 仅白名单字段（整包参数池被拒）；
//  3. sfkxk=0 时 Display 返回 911 → 容忍并用首页参数继续；
//  4. 参数池 90s 复用：第二次查询只发 1 个 PartDisplay；
//  5. {"flag":"0","msg":"..."} 错误信封不得被当成"0 门课"。
// ─────────────────────────────────────────────────────────────────────────────

// newChainTestClient 构造指向测试服务器的客户端（与 NewClient 等价，但 baseURL 可指定）
func newChainTestClient(baseURL string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		httpClient: &http.Client{
			Timeout:       10 * time.Second,
			Jar:           jar,
			CheckRedirect: safeRedirectPolicy,
		},
		baseURL:         baseURL,
		cookieJar:       jar,
		circuitBreaker:  stealth.NewCircuitBreaker("test"),
		backoffStrategy: stealth.NewBackoffStrategy(5*time.Second, 60*time.Second, 2.0, true),
	}
}

// indexPageWithRounds 构造带两个页签的选课首页（queryCourse + tab_kklx_ 锚文本 + firstXxx）
func indexPageWithRounds() string {
	return `<!DOCTYPE html><html><body>
	<input type="hidden" name="firstKklxdm" value="10">
	<input type="hidden" name="firstKklxmc" value="通识选修课">
	<input type="hidden" name="firstXkkzId" value="XK-FIRST">
	<input type="hidden" name="firstXkkzXh" value="HEX-FIRST-248CHARS">
	<input type="hidden" name="firstNjdmId" value="2023">
	<input type="hidden" name="firstZyhId" value="0801">
	<input type="hidden" name="xkkz_id" value="XK-PAGE">
	<input type="hidden" name="njdm_id" value="2023">
	<input type="hidden" name="zyh_id" value="0801">
	<input type="hidden" name="xszxzt" value="1">
	<input type="hidden" name="xkxnm" value="2026">
	<input type="hidden" name="xkxqm" value="1">
	<input type="hidden" name="rwlx" value="2">
	<input type="hidden" name="csrftoken" value="LOGIN-FORM-FIELD">
	<input type="hidden" name="iskxk" value="1">
	<ul class="nav">
	<li id="tab_kklx_10"><a href="javascript:void(0);" onclick="queryCourse(this,'10','XK-A','2023','0801','HEX-A-248')">通识选修课</a></li>
	<li id="tab_kklx_20"><a href="javascript:void(0);" onclick="queryCourse(this,'20','XK-B','2023','0801','HEX-B-248')">体育课</a></li>
	</ul>
	</body></html>`
}

// chainServer 记录每个端点收到的请求，并按脚本返回响应
type chainServer struct {
	mu sync.Mutex

	displayParams  url.Values  // Display 收到的参数
	partParams     url.Values  // PartDisplay 收到的参数
	displayHits    int
	partHits       int
	indexHits      int

	displayStatus  int    // Display 返回的状态码（默认 200）
	displayBody    string // Display 返回的响应体
	partBody       string // PartDisplay 返回的响应体
}

func (s *chainServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/xsxk/zzxkyzb_cxZzxkYzbIndex.html", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.indexHits++
		s.mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(indexPageWithRounds()))
	})
	mux.HandleFunc("/xsxk/zzxkyzb_cxZzxkYzbDisplay.html", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm() // PostForm 需显式解析后才可用
		s.mu.Lock()
		s.displayHits++
		s.displayParams = r.PostForm
		status, body := s.displayStatus, s.displayBody
		s.mu.Unlock()
		if status == 0 {
			status = 200
		}
		if body == "" {
			body = `<input type="hidden" name="rwlx" value="2">
			<input type="hidden" name="xklc" value="8">
			<input type="hidden" name="rlkz" value="1">
			<input type="hidden" name="rlzlkz" value="1">
			<input type="hidden" name="cdrlkz" value="1">
			<input type="hidden" name="xkly" value="1">
			<input type="hidden" name="xkkz_id" value="XK-DISPLAY">`
		}
		w.WriteHeader(status)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(body))
	})
	mux.HandleFunc("/xsxk/zzxkyzb_cxZzxkYzbPartDisplay.html", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm() // PostForm 需显式解析后才可用
		s.mu.Lock()
		s.partHits++
		s.partParams = r.PostForm
		body := s.partBody
		s.mu.Unlock()
		if body == "" {
			body = `{"tmpList":[{"kch_id":"T0000002","kcmc":"测试课程甲","kch":"T0000002","xf":"2.0","kklxdm":"10","jxb_id":"J1","yxzrs":"90","zrs":"90"}]}`
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})
	return mux
}

// TestGetClassList_FullChain 完整链路：Display 7 参 / PartDisplay 白名单+xkkz_xh / 轮次整组取用
func TestGetClassList_FullChain(t *testing.T) {
	srv := &chainServer{}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	c := newChainTestClient(ts.URL)
	list, err := c.GetClassList(&model.Config{CourseType: "体育课"})
	if err != nil {
		t.Fatalf("GetClassList 失败: %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("应解析出 1 门课, got %d", list.Total)
	}

	// ── Display：浏览器同款 7 参数，且 kklxdm 取所选类型的轮次（体育课=20 → XK-B/HEX-B-248）
	dp := srv.displayParams
	if dp.Get("xkkz_id") != "XK-B" {
		t.Errorf("Display xkkz_id = %q, want XK-B（轮次整组取用，不得用 first 轮次）", dp.Get("xkkz_id"))
	}
	if dp.Get("kklxdm") != "20" {
		t.Errorf("Display kklxdm = %q, want 20", dp.Get("kklxdm"))
	}
	if dp.Get("xszxzt") != "1" {
		t.Errorf("Display xszxzt = %q, want 1", dp.Get("xszxzt"))
	}
	if dp.Get("kspage") != "0" || dp.Get("jspage") != "0" {
		t.Errorf("Display 分页参数应固定为 0: kspage=%q jspage=%q", dp.Get("kspage"), dp.Get("jspage"))
	}
	if _, exists := dp["csrftoken"]; exists {
		t.Error("Display 请求不得携带登录页 form 的字段（csrftoken）")
	}

	// ── PartDisplay：xkkz_xh 必需 + 仅白名单字段
	pp := srv.partParams
	if pp.Get("xkkz_xh") != "HEX-B-248" {
		t.Errorf("PartDisplay xkkz_xh = %q, want HEX-B-248（缺它服务端回「加密串错误」）", pp.Get("xkkz_xh"))
	}
	if pp.Get("kklxdm") != "20" {
		t.Errorf("PartDisplay kklxdm = %q, want 20", pp.Get("kklxdm"))
	}
	for _, forbidden := range []string{"csrftoken", "iskxk", "firstKklxdm", "firstXkkzXh", "zzxkyzb_junk"} {
		if _, exists := pp[forbidden]; exists {
			t.Errorf("PartDisplay 不得携带白名单外字段 %q（整包参数池会被服务端拒绝）", forbidden)
		}
	}
}

// TestGetClassList_ToleratesDisplay911 sfkxk=0 时 Display 返回 911 ——
// 这是正常业务分支，必须容忍并仅用首页参数继续查列表。
func TestGetClassList_ToleratesDisplay911(t *testing.T) {
	srv := &chainServer{displayStatus: 911}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	c := newChainTestClient(ts.URL)
	list, err := c.GetClassList(&model.Config{CourseType: "体育课"})
	if err != nil {
		t.Fatalf("Display 911 时应容忍并继续，got %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("应仅用首页参数查到课程, got %d", list.Total)
	}
	if !strings.Contains(list.Warning, "911") {
		t.Errorf("Warning 应说明 Display 911 的成因: %q", list.Warning)
	}
	// PartDisplay 仍必须带 xkkz_xh（来自首页 firstXkkzXh / 轮次）
	if srv.partParams.Get("xkkz_xh") == "" {
		t.Error("Display 911 后 PartDisplay 仍必须带 xkkz_xh")
	}
}

// TestGetClassList_ErrorEnvelopeNotZeroCourses {"flag":"0","msg":"加密串错误"}
// 是协议错误，不得被 ParseCourseList 解析成"0 门课"（那会把故障伪装成业务结果）。
func TestGetClassList_ErrorEnvelopeNotZeroCourses(t *testing.T) {
	srv := &chainServer{partBody: `{"flag":"0","msg":"加密串错误"}`}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	c := newChainTestClient(ts.URL)
	_, err := c.GetClassList(&model.Config{CourseType: "体育课"})
	if err == nil {
		t.Fatal("错误信封必须返回 error，不能当成 0 门课")
	}
	if !strings.Contains(err.Error(), "加密串错误") {
		t.Errorf("错误信息应透传服务端原因: %v", err)
	}
}

// TestGetClassList_PoolReuse 参数池 90s 复用：列表缓存过期后第二次查询
// 只发 1 个 PartDisplay（不发探测/首页/Display）。
func TestGetClassList_PoolReuse(t *testing.T) {
	srv := &chainServer{}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	c := newChainTestClient(ts.URL)
	cfg := &model.Config{CourseType: "体育课"}
	if _, err := c.GetClassList(cfg); err != nil {
		t.Fatalf("首次查询失败: %v", err)
	}
	// index 计数含"不跟随重定向的探测 GET"（同一 URL，共 2 次）
	firstIndex, firstDisplay, firstPart := srv.indexHits, srv.displayHits, srv.partHits
	if firstIndex != 2 || firstDisplay != 1 || firstPart != 1 {
		t.Fatalf("首次应为完整链路(探测+首页+Display+PartDisplay): index=%d display=%d part=%d", firstIndex, firstDisplay, firstPart)
	}

	// 等列表缓存（1s TTL）过期，但参数池（90s）仍有效
	time.Sleep(1100 * time.Millisecond)
	if _, err := c.GetClassList(cfg); err != nil {
		t.Fatalf("第二次查询失败: %v", err)
	}
	if srv.indexHits != firstIndex || srv.displayHits != firstDisplay {
		t.Errorf("参数池复用窗口内不应再打探测/首页/Display: index=%d(+%d) display=%d(+%d)",
			srv.indexHits, srv.indexHits-firstIndex, srv.displayHits, srv.displayHits-firstDisplay)
	}
	if srv.partHits != firstPart+1 {
		t.Errorf("第二次查询应只发 1 个 PartDisplay: part=%d(+%d)", srv.partHits, srv.partHits-firstPart)
	}

	// 清会话绑定状态后，参数池作废 → 重新走完整链路（+探测+首页）
	c.ClearSessionBoundState()
	if _, err := c.GetClassList(cfg); err != nil {
		t.Fatalf("第三次查询失败: %v", err)
	}
	if srv.indexHits != firstIndex+2 {
		t.Errorf("ClearSessionBoundState 后应重新走完整链路: index=%d(+%d)", srv.indexHits, srv.indexHits-firstIndex)
	}
}

// TestGetClassList_TypeRelaxedWhenTypeNotOpen 所选类型无开放轮次时：
// 回落首个开放轮次 + TypeRelaxed + Warning（否则该轮次课程被全部滤掉 → 死路）。
func TestGetClassList_TypeRelaxedWhenTypeNotOpen(t *testing.T) {
	srv := &chainServer{}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	c := newChainTestClient(ts.URL)
	// 首页只有 kklxdm=10/20 两个轮次；选"普通课"(候选 01/30) → 未命中 → 回落
	list, err := c.GetClassList(&model.Config{CourseType: "普通课"})
	if err != nil {
		t.Fatalf("GetClassList 失败: %v", err)
	}
	if !list.TypeRelaxed {
		t.Error("所选类型无开放轮次时应置 TypeRelaxed")
	}
	if list.Warning == "" {
		t.Error("所选类型无开放轮次时应给出 Warning 说明回落")
	}
	// 查询用回落轮次（首个开放轮次 = 10）
	if srv.partParams.Get("kklxdm") != "10" {
		t.Errorf("应回落到首个开放轮次 kklxdm=10, got %q", srv.partParams.Get("kklxdm"))
	}
}

// TestGetClassInfo_BareZeroResponse sfkxk=0 时详情接口返回裸 "0"（HTTP 200）——
// 必须给出可操作说明，而不是难懂的 unmarshal 错误。
func TestGetClassInfo_BareZeroResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`"0"`))
	}))
	defer ts.Close()

	c := newChainTestClient(ts.URL)
	c.SetSelectInitParams(map[string]string{"xkkz_id": "XK", "rlkz": "1"})
	_, err := c.GetClassInfo(&model.Course{ID: "K1", Type: "10"})
	if err == nil {
		t.Fatal("裸 \"0\" 响应必须返回错误")
	}
	if !strings.Contains(err.Error(), "sfkxk=0") {
		t.Errorf("错误信息应说明真实原因(sfxk=0)，got: %v", err)
	}
}

// TestParseRoundsFromIndexPage 轮次解析：多页签整组取用、残缺页签丢弃、无页签用 firstXxx
func TestParseRoundsFromIndexPage(t *testing.T) {
	rounds := model.ParseRounds(indexPageWithRounds())
	if len(rounds) != 2 {
		t.Fatalf("应解析出 2 个轮次, got %d: %+v", len(rounds), rounds)
	}
	if rounds[0].Kklxdm != "10" || rounds[0].XkkzID != "XK-A" || rounds[0].XkkzXh != "HEX-A-248" {
		t.Errorf("第一个轮次参数错误: %+v", rounds[0])
	}
	if rounds[0].Name != "通识选修课" {
		t.Errorf("页签名解析错误: %q", rounds[0].Name)
	}
	if rounds[1].Kklxdm != "20" || rounds[1].XkkzID != "XK-B" {
		t.Errorf("第二个轮次参数错误: %+v", rounds[1])
	}

	// 残缺页签（缺 xkkz_xh）必须丢弃
	broken := `<html><body><script>
	queryCourse(this,'10','XK-A','2023','0801','HEX-A-248');
	queryCourse(this,'20','XK-B','2023','0801','');
	</script></body></html>`
	if got := model.ParseRounds(broken); len(got) != 1 || got[0].Kklxdm != "10" {
		t.Errorf("残缺页签应被丢弃, got %+v", got)
	}

	// 无页签（单轮次开放）→ 用 firstXxx 合成
	single := `<html><body>
	<input type="hidden" name="firstKklxdm" value="11">
	<input type="hidden" name="firstKklxmc" value="其他特殊课程">
	<input type="hidden" name="firstXkkzId" value="XK-11">
	<input type="hidden" name="firstXkkzXh" value="HEX-11">
	<input type="hidden" name="firstNjdmId" value="2023">
	<input type="hidden" name="firstZyhId" value="0801">
	</body></html>`
	got := model.ParseRounds(single)
	if len(got) != 1 || got[0].Kklxdm != "11" || got[0].Name != "其他特殊课程" || got[0].XkkzXh != "HEX-11" {
		t.Errorf("无页签时应由 firstXxx 合成单轮次, got %+v", got)
	}

	// 空页面 → nil
	if got := model.ParseRounds(""); got != nil {
		t.Errorf("空页面应返回 nil, got %+v", got)
	}
}
