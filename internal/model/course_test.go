package model

import (
	"strings"
	"testing"
)

// ── ParseCourseList ──────────────────────────────────────────────────────────
// 核心契约：「查到 0 门课」是业务结果，不是错误。
// V3.0 的原始 bug 是把空 tmpList 当成"初始化课表失败"，用户看到的
// "获取课表失败"其实是没排课。

func TestParseCourseList_EmptyListIsNotAnError(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"空数组", `{"tmpList":[]}`},
		{"null", `{"tmpList":null}`},
		{"字段缺失", `{}`},
		{"附带其它字段", `{"sfxsjc":"1","tmpList":[]}`},
	}
	for _, c := range cases {
		list, err := ParseCourseList([]byte(c.raw))
		if err != nil {
			t.Errorf("%s: 0 门课不应报错, got %v", c.name, err)
			continue
		}
		if list == nil || list.Total != 0 || len(list.Items) != 0 {
			t.Errorf("%s: want Total=0/len=0, got %+v", c.name, list)
		}
		if list.Items == nil {
			t.Errorf("%s: Items 应为非 nil 空切片（便于调用方直接遍历）", c.name)
		}
	}
}

func TestParseCourseList_StringTypedNumbers(t *testing.T) {
	// 正方各部署字段类型不统一：学分/容量常以字符串下发。
	// V3.0 直接 .(float64) 断言 → 字符串时静默归零 → 课程被 MinCredit 全过滤。
	raw := `{"tmpList":[{
		"kch_id":"C1","kcmc":"羽毛球","kch":"0200200200",
		"xf":"2.0","zrs":"60","yxzrs":"12","kklxdm":"20",
		"jxb_id":"J1","jsm":"张三","sksj":"周一1-2"
	}]}`
	list, err := ParseCourseList([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("Total = %d, want 1", list.Total)
	}
	c := list.Items[0]
	if c.Credit != 2 {
		t.Errorf("Credit = %d, want 2（字符串 \"2.0\" 应被解析）", c.Credit)
	}
	if c.Capacity != 60 || c.Selected != 12 {
		t.Errorf("容量解析错误: %d/%d, want 12/60", c.Selected, c.Capacity)
	}
	if c.Type != "20" {
		t.Errorf("Type = %q, want \"20\"", c.Type)
	}
	if c.Name != "羽毛球" || c.Teacher != "张三" {
		t.Errorf("基础字段解析错误: %+v", c)
	}
}

func TestParseCourseList_NumberTypedNumbers(t *testing.T) {
	raw := `{"tmpList":[{"kcmc":"A","xf":3,"zrs":40,"yxzrs":40,"kcrow":2}]}`
	list, err := ParseCourseList([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	c := list.Items[0]
	if c.Credit != 3 || c.Capacity != 40 || c.RowNum != 2 {
		t.Errorf("数字型字段解析错误: %+v", c)
	}
}

func TestParseCourseList_ArrayRoot(t *testing.T) {
	// 部分接口（如已选课程）直接返回数组
	list, err := ParseCourseList([]byte(`[{"kcmc":"A"},{"kcmc":"B"}]`))
	if err != nil {
		t.Fatalf("数组根节点应可解析: %v", err)
	}
	if list.Total != 2 {
		t.Fatalf("Total = %d, want 2", list.Total)
	}
}

func TestParseCourseList_HTMLErrorPage(t *testing.T) {
	_, err := ParseCourseList([]byte("<html><body>系统正在维护，请稍后再试</body></html>"))
	if err == nil {
		t.Fatal("HTML 响应应返回可读错误，而不是 JSON 解析器的 invalid character")
	}
	if !strings.Contains(err.Error(), "HTML") {
		t.Errorf("错误信息应明确指出返回了 HTML, got %v", err)
	}
}

func TestParseCourseList_EmptyBody(t *testing.T) {
	if _, err := ParseCourseList([]byte("   ")); err == nil {
		t.Fatal("空响应应报错")
	}
}

// ── Match ────────────────────────────────────────────────────────────────────

func TestCourseMatch_CourseTypeInChineseAndEnglish(t *testing.T) {
	pe := &Course{Type: "20", Name: "羽毛球", Credit: 2}
	online := &Course{Type: "10", Name: "网课A", Credit: 2}
	normal := &Course{Type: "30", Name: "普通课A", Credit: 2}

	// 中文标签（历史 bug：中文永远匹配不上）
	if !pe.Match(&Config{CourseType: "体育课"}) {
		t.Error("中文「体育课」应匹配 kklxdm=20")
	}
	if online.Match(&Config{CourseType: "体育课"}) {
		t.Error("中文「体育课」不应匹配 kklxdm=10")
	}
	if !online.Match(&Config{CourseType: "普通网课"}) {
		t.Error("中文「普通网课」应匹配 kklxdm=10")
	}

	// 英文规范代码
	if !pe.Match(&Config{CourseType: CourseTypePE}) {
		t.Error("pe 应匹配 kklxdm=20")
	}

	// 普通课 = 既不是网课也不是体育课
	if pe.Match(&Config{CourseType: CourseTypeNormal}) || online.Match(&Config{CourseType: CourseTypeNormal}) {
		t.Error("普通课不应匹配 10/20")
	}
	if !normal.Match(&Config{CourseType: CourseTypeNormal}) {
		t.Error("普通课应匹配 kklxdm=30")
	}
}

func TestCourseMatch_OtherFilters(t *testing.T) {
	c := &Course{Type: "10", Name: "羽毛球提高班", Number: "0200200200", Teacher: "张三", Credit: 2}

	if c.Match(&Config{CourseType: CourseTypeOnline, CourseName: "羽毛球提高"}) == false {
		t.Error("课程名应支持子串匹配")
	}
	if c.Match(&Config{CourseType: CourseTypeOnline, CourseName: "篮球"}) {
		t.Error("不匹配的课程名应被过滤")
	}
	if c.Match(&Config{CourseType: CourseTypeOnline, TeacherName: "李四"}) {
		t.Error("教师名应精确匹配")
	}
	if !c.Match(&Config{CourseType: CourseTypeOnline, TeacherName: "张三"}) {
		t.Error("相同教师名应通过")
	}
	if c.Match(&Config{CourseType: CourseTypeOnline, MinCredit: 3}) {
		t.Error("学分 2 < MinCredit 3，应被过滤")
	}
	if !c.Match(&Config{CourseType: CourseTypeOnline, MinCredit: 2}) {
		t.Error("学分等于阈值应通过")
	}
}

func TestCourseIsFull(t *testing.T) {
	if !(&Course{Selected: 60, Capacity: 60}).IsFull() {
		t.Error("已选=容量 应判满")
	}
	if (&Course{Selected: 10, Capacity: 60}).IsFull() {
		t.Error("未满不应判满")
	}
	// 容量未知（0）时不判满，避免因数据缺失漏掉课程
	if (&Course{Selected: 999, Capacity: 0}).IsFull() {
		t.Error("容量未知时不应判满")
	}
}

func TestCourseListCloneIsIndependent(t *testing.T) {
	src := &CourseList{Total: 1, Items: []*Course{{Name: "A", ClassID: "J1"}}}
	cp := src.Clone()

	if cp.Total != 1 || len(cp.Items) != 1 {
		t.Fatalf("Clone 结果不正确: %+v", cp)
	}
	if cp.Items[0] == src.Items[0] {
		t.Fatal("Clone 必须返回独立的 Course 指针（否则多 Worker 并发写 Extra 会数据竞争）")
	}

	cp.Items[0].Extra = &CourseExtra{DoJxbID: "LONG"}
	if src.Items[0].Extra != nil {
		t.Fatal("修改副本不应影响原对象")
	}

	if (*CourseList)(nil).Clone() != nil {
		t.Error("nil 列表的 Clone 应返回 nil")
	}
}
