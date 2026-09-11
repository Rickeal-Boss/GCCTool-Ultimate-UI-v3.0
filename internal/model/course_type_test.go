package model

import "testing"

// 课程类型是本项目最严重的历史 bug 所在：
// V3.0 把 UI 的中文标签直接当业务值，导致 Match() 的三个 if 全部不成立
// （类型过滤失效）且查询类别码恒为 10。以下用例锁住"任一输入形态都能正确归一化"。

func TestNormalizeCourseType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", CourseTypeOnline},
		{"   ", CourseTypeOnline},
		{"online", CourseTypeOnline},
		{"pe", CourseTypePE},
		{"normal", CourseTypeNormal},
		{"普通网课", CourseTypeOnline},
		{"网课", CourseTypeOnline},
		{"体育课", CourseTypePE},
		{"普通课", CourseTypeNormal},
		{"不认识的类型", CourseTypeOnline}, // 未知值回落网课，避免空查询
	}
	for _, c := range cases {
		if got := NormalizeCourseType(c.in); got != c.want {
			t.Errorf("NormalizeCourseType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCourseTypeLabelRoundTrip(t *testing.T) {
	for _, code := range []string{CourseTypeOnline, CourseTypePE, CourseTypeNormal} {
		label := CourseTypeLabel(code)
		if got := NormalizeCourseType(label); got != code {
			t.Errorf("往返失败: %q -> %q -> %q", code, label, got)
		}
	}
	// 中文标签输入也应被接受（用于回填 UI 后再次取值）
	if got := CourseTypeLabel("体育课"); got != "体育课" {
		t.Errorf("CourseTypeLabel(体育课) = %q, want 体育课", got)
	}
}

func TestCourseTypeLabelsAreStable(t *testing.T) {
	labels := CourseTypeLabels()
	if len(labels) != 3 {
		t.Fatalf("want 3 labels, got %d", len(labels))
	}
	// 返回副本：外部修改不应影响内部状态
	labels[0] = "伪造"
	if CourseTypeLabels()[0] == "伪造" {
		t.Error("CourseTypeLabels 应返回副本")
	}
}

func TestCourseTypeCodeKklxdm(t *testing.T) {
	cases := map[string]string{
		CourseTypeOnline: "10",
		CourseTypePE:     "20",
		CourseTypeNormal: "30",
		"普通网课":           "10", // 中文必须走对映射，而不是 default
		"体育课":            "20",
		"普通课":            "30",
		"":               "10",
	}
	for in, want := range cases {
		if got := CourseTypeCodeKklxdm(in); got != want {
			t.Errorf("CourseTypeCodeKklxdm(%q) = %q, want %q", in, got, want)
		}
	}
}
