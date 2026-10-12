package robber

import (
	"testing"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/client"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/pkg/logger"
)

// newFilterTestRobber 构造仅用于 filterCourses 测试的 Robber（不发任何请求）
func newFilterTestRobber(cfg *model.Config) *Robber {
	return &Robber{
		client: client.NewClient("节点1（推荐）"),
		logger: logger.NewLogger(nil),
		config: cfg,
	}
}

// TypeRelaxed：所选课程类型无开放轮次时（客户端已回落到实际开放轮次），
// 本地匹配必须放宽类型过滤 —— 否则该轮次所有课程被滤掉，用户一门课都看不到（死路）。
// 实机场景：选了"网课"(kklxdm=10)，2026-10 第 1 轮只开放 kklxdm=11 其他特殊课程。
func TestFilterCourses_TypeRelaxed(t *testing.T) {
	r := newFilterTestRobber(&model.Config{CourseType: model.CourseTypeOnline, CourseName: "产品创新"})

	list := &model.CourseList{
		Total:       2,
		TypeRelaxed: true,
		Items: []*model.Course{
			{Type: "11", Name: "数据驱动的产品创新", Credit: 2},
			{Type: "10", Name: "网课A", Credit: 2},
		},
	}

	matched := r.filterCourses(list)
	// 类型过滤放宽后：kklxdm=11 的课也应命中（课程名匹配），
	// 而非被"不是网课"滤掉导致 0 候选
	if len(matched) != 1 || matched[0].Name != "数据驱动的产品创新" {
		t.Fatalf("TypeRelaxed 时应放宽类型过滤, got %+v", matched)
	}

	// 未放宽时（TypeRelaxed=false）：kklxdm=11 仍按网课类型被过滤
	list.TypeRelaxed = false
	if got := r.filterCourses(list); len(got) != 0 {
		t.Errorf("未回落时应按配置类型过滤, got %+v", got)
	}
}

// readyCoursesFirst：详情就绪（有 do_jxb_id）的课程排前面 ——
// 每轮提交名额有限，不筛就可能全打在注定被拒的短 jxb_id 课程上。
func TestReadyCoursesFirst(t *testing.T) {
	courses := []*model.Course{
		{Name: "A"}, // 无详情
		{Name: "B", Extra: &model.CourseExtra{DoJxbID: "LONG-B"}},
		{Name: "C"}, // 无详情
		{Name: "D", Extra: &model.CourseExtra{DoJxbID: "LONG-D"}},
	}
	got := readyCoursesFirst(courses)
	if len(got) != 4 {
		t.Fatalf("元素不应丢失: %d", len(got))
	}
	if got[0].Name != "B" || got[1].Name != "D" {
		t.Errorf("详情就绪的课程应排前面, got %s,%s", got[0].Name, got[1].Name)
	}
	if got[2].Name != "A" || got[3].Name != "C" {
		t.Errorf("无详情课程应保持原相对顺序, got %s,%s", got[2].Name, got[3].Name)
	}
}
