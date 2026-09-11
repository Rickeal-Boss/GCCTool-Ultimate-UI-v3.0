package client

import (
	"testing"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
)

// ── parseCourseExtra ─────────────────────────────────────────────────────────
// V3.0 固定按 JSON 对象解析，而同一接口在不同部署下可能返回数组。
// 形态不符时 Unmarshal 失败 → robber 侧 continue 跳过该课 → 整轮颗粒无收。

func TestParseCourseExtra_ArrayAndObject(t *testing.T) {
	arr := `[{"kcmc":"羽毛球","do_jxb_id":"LONG","jsmc":"体育馆","sksj":"周一1-2","kcbj":"备注"}]`
	e := parseCourseExtra(arr)
	if e == nil {
		t.Fatal("数组形态应能解析")
	}
	if e.DoJxbID != "LONG" || e.ClassInfo != "体育馆" || e.ExamInfo != "周一1-2" || e.Remark != "备注" {
		t.Errorf("数组形态字段解析错误: %+v", e)
	}

	obj := `{"kcmc":"羽毛球","do_jxb_id":"LONG2"}`
	e = parseCourseExtra(obj)
	if e == nil || e.DoJxbID != "LONG2" {
		t.Fatalf("对象形态应能解析: %+v", e)
	}
}

func TestParseCourseExtra_SkipsEmptyArrayElements(t *testing.T) {
	// 数组首个元素为空对象时，应继续找下一个可用元素
	e := parseCourseExtra(`[{},{"kcmc":"B","do_jxb_id":"ID-B"}]`)
	if e == nil || e.DoJxbID != "ID-B" {
		t.Fatalf("应跳过空元素: %+v", e)
	}
}

func TestParseCourseExtra_KeyNormalization(t *testing.T) {
	// 不同部署可能写成 doJxbId 而非 do_jxb_id
	e := parseCourseExtra(`{"kcmc":"A","doJxbId":"X"}`)
	if e == nil || e.DoJxbID != "X" {
		t.Fatalf("key 归一化失败: %+v", e)
	}
}

func TestParseCourseExtra_NoUsableContent(t *testing.T) {
	// 空数组 / 空对象 / 空串 → nil，由调用方降级用短 jxb_id 重试
	for _, s := range []string{"[]", "{}", "", "   ", `[{}]`, `{"foo":"bar"}`} {
		if got := parseCourseExtra(s); got != nil {
			t.Errorf("parseCourseExtra(%q) = %+v, want nil", s, got)
		}
	}
}

// ── looksLikeLoginPage ───────────────────────────────────────────────────────
// 会话失效在 AJAX 链路里原本完全隐形：http.Client 跟随 302 后拿到 200 + 登录页，
// 表现为"JSON 解析失败"，无法触发重新登录。

func TestLooksLikeLoginPage(t *testing.T) {
	login := `<html><form action="/xtgl/login_slogin.html">` +
		`<input type="password" name="mm"></form></html>`
	if !looksLikeLoginPage(login) {
		t.Error("登录页应被识别")
	}
	// 单引号写法同样识别
	if !looksLikeLoginPage(`login_slogin <input type='password'>`) {
		t.Error("单引号 password 也应被识别")
	}
	// 正常 JSON / 仅提到 login_slogin 的文本，不应误判
	if looksLikeLoginPage(`{"tmpList":[]}`) {
		t.Error("正常 JSON 不应被判为登录页")
	}
	if looksLikeLoginPage("refer to login_slogin.html in docs") {
		t.Error("只有路径片段、没有密码框时不应误判")
	}
}

// ── parseCategoryOptions / categoryCodes ─────────────────────────────────────
// V3.0 的 cfg.Categories 只被收集、从未参与查询：勾选分类完全无效且无提示。

func TestParseCategoryOptions(t *testing.T) {
	page := `<html><select name="kcgs_list">` +
		`<option value="01">科技类</option>` +
		`<option value="">人文类</option>` +
		`</select></html>`
	opts := parseCategoryOptions(page)

	if opts["科技类"] != "01" {
		t.Errorf("科技类 -> %q, want 01", opts["科技类"])
	}
	// value 缺失时退化为选项文本
	if opts["人文类"] != "人文类" {
		t.Errorf("人文类 -> %q, want 人文类", opts["人文类"])
	}
}

func TestParseCategoryOptions_IgnoresUnrelatedSelects(t *testing.T) {
	page := `<select name="xkxnm"><option value="2026">2026</option></select>`
	if len(parseCategoryOptions(page)) != 0 {
		t.Error("非课程归属类下拉框不应被采集")
	}
}

func TestCategoryCodesAndMatchCategories(t *testing.T) {
	c := newTestClient()
	c.SetCategoryOptions(map[string]string{"科技类": "01", "艺术类": "06"})

	// 命中 → 产出真实代码
	codes := c.categoryCodes(&model.Config{Categories: map[string]bool{"科技类": true, "艺术类": true}})
	if len(codes) != 2 || codes[0] != "01" || codes[1] != "06" {
		t.Errorf("categoryCodes = %v, want [01 06]", codes)
	}

	// 页面没有该选项 → 不臆造代码，交给上层告警
	c2 := newTestClient()
	c2.SetCategoryOptions(map[string]string{"科技类": "01"})
	if got := c2.categoryCodes(&model.Config{Categories: map[string]bool{"体育类": true}}); len(got) != 0 {
		t.Errorf("未匹配的分类不应产出代码, got %v", got)
	}

	applied, unmatched := c2.MatchCategories(map[string]bool{"科技类": true, "体育类": true})
	if len(applied) != 1 || applied[0] != "科技类" {
		t.Errorf("applied = %v", applied)
	}
	if len(unmatched) != 1 || unmatched[0] != "体育类" {
		t.Errorf("unmatched = %v", unmatched)
	}
}

func TestBuildCourseQueryParams(t *testing.T) {
	c := newTestClient()
	c.SetCategoryOptions(map[string]string{"科技类": "01"})

	cfg := &model.Config{
		CourseType: "体育课", // 中文也应归一化
		Categories: map[string]bool{"科技类": true},
	}
	p := c.buildCourseQueryParams(map[string]string{"xkkz_id": "XK"}, cfg)

	if p["kklxdm"] != "20" {
		t.Errorf("kklxdm = %q, want 20（中文「体育课」应映射为 20）", p["kklxdm"])
	}
	if p["kcgs_list"] != "01" {
		t.Errorf("kcgs_list = %q, want 01", p["kcgs_list"])
	}
	if p["xkkz_id"] != "XK" {
		t.Errorf("基础参数应被保留, got %q", p["xkkz_id"])
	}
}

func TestCourseListCacheKey(t *testing.T) {
	c := newTestClient()

	k1 := c.courseListCacheKey(&model.Config{CourseType: "普通网课"})
	k2 := c.courseListCacheKey(&model.Config{CourseType: "online"})
	if k1 != k2 {
		t.Errorf("中文与英文代码应产生同一个缓存键: %q vs %q", k1, k2)
	}
	k3 := c.courseListCacheKey(&model.Config{CourseType: "体育课"})
	if k1 == k3 {
		t.Error("不同课程类型必须产生不同的缓存键")
	}

	k4 := c.courseListCacheKey(&model.Config{CourseType: "online", Categories: map[string]bool{"科技类": true}})
	if k1 == k4 {
		t.Error("勾选分类后缓存键应变化")
	}
}

// ── 课程列表短 TTL 缓存 ──────────────────────────────────────────────────────

func TestCourseListCache_CloneOnHit(t *testing.T) {
	c := newTestClient()
	src := &model.CourseList{Total: 1, Items: []*model.Course{{Name: "A"}}}

	if got := c.CachedCourseList("k"); got != nil {
		t.Error("未写入缓存时应返回 nil")
	}

	c.CacheCourseList("k", src)
	hit := c.CachedCourseList("k")
	if hit == nil || hit.Total != 1 {
		t.Fatalf("缓存命中失败: %+v", hit)
	}
	if hit.Items[0] == src.Items[0] {
		t.Error("缓存命中必须返回副本，避免多 Worker 共享 Course 指针写 Extra")
	}

	// 键不同 → 不命中
	if c.CachedCourseList("other") != nil {
		t.Error("不同键不应命中缓存")
	}

	// 主动作废（选课成功后调用）
	c.InvalidateCourseListCache()
	if c.CachedCourseList("k") != nil {
		t.Error("作废后不应命中缓存")
	}
}
