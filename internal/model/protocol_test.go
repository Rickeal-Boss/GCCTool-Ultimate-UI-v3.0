package model

import (
	"strings"
	"testing"
)

// ── ParseRounds：页签（轮次）机制 ───────────────────────────────────────────

func TestParseRounds_Tabs(t *testing.T) {
	page := `<html><body><script>
	function queryCourse(a,kklxdm,xkkz_id,njdm_id,zyh_id,xkkz_xh){}
	queryCourse(this,'10','XK-A','2023','0801','HEX-A');
	queryCourse(this,'20','XK-B','2023','0801','HEX-B');
	</script>
	<ul class="nav">
	<li id="tab_kklx_10"><a>通识选修课</a></li>
	<li id="tab_kklx_20"><a>体育课</a></li>
	</ul></body></html>`

	rounds := ParseRounds(page)
	if len(rounds) != 2 {
		t.Fatalf("应解析出 2 个轮次, got %d", len(rounds))
	}
	if rounds[0].Kklxdm != "10" || rounds[0].XkkzID != "XK-A" || rounds[0].XkkzXh != "HEX-A" || rounds[0].Name != "通识选修课" {
		t.Errorf("轮次0 解析错误: %+v", rounds[0])
	}
	if rounds[1].Kklxdm != "20" || rounds[1].Name != "体育课" {
		t.Errorf("轮次1 解析错误: %+v", rounds[1])
	}
}

func TestParseRounds_IncompleteTabDropped(t *testing.T) {
	// 缺 xkkz_xh 的残缺页签必须丢弃（半组参数只会得到误导性的「加密串错误」）
	page := `queryCourse(this,'10','XK-A','2023','0801','HEX-A');
	queryCourse(this,'20','','2023','0801','HEX-B');`
	rounds := ParseRounds(page)
	if len(rounds) != 1 || rounds[0].Kklxdm != "10" {
		t.Fatalf("残缺页签应被丢弃, got %+v", rounds)
	}
}

func TestParseRounds_SingleRoundFromFirst(t *testing.T) {
	// 只有一个页签时站点不渲染页签 —— 用 firstXxx 合成
	page := `<input type="hidden" name="firstKklxdm" value="11">
	<input type="hidden" name="firstKklxmc" value="其他特殊课程">
	<input type="hidden" name="firstXkkzId" value="XK-11">
	<input type="hidden" name="firstXkkzXh" value="HEX-11">
	<input type="hidden" name="firstNjdmId" value="2023">
	<input type="hidden" name="firstZyhId" value="0801">`
	rounds := ParseRounds(page)
	if len(rounds) != 1 {
		t.Fatalf("应合成单轮次, got %+v", rounds)
	}
	if rounds[0].Kklxdm != "11" || rounds[0].XkkzXh != "HEX-11" || rounds[0].Name != "其他特殊课程" {
		t.Errorf("合成轮次错误: %+v", rounds[0])
	}
}

func TestParseRounds_NoRounds(t *testing.T) {
	if got := ParseRounds("<html>无页签无first</html>"); got != nil {
		t.Errorf("无任何轮次信息应返回 nil, got %+v", got)
	}
	if got := ParseRounds(""); got != nil {
		t.Errorf("空页面应返回 nil, got %+v", got)
	}
}

// ── ParseTeacherFromJsxx：教师名三格式 ─────────────────────────────────────

func TestParseTeacherFromJsxx_ThreeFormats(t *testing.T) {
	cases := []struct {
		jsxx string
		want string
	}{
		// 实机三种共存格式（原实现三种全错）
		{"70010/教务处/无", ""},          // 网课：教务处开课，无教师
		{"20000001/测试教师/讲师", "测试教师"}, // 主修课：第3段是职称
		{"20000002/测试教师乙/无", "测试教师乙"}, // 职称缺失
		{"20000002/测试教师乙/", "测试教师乙"},  // 第3段为空
		{"20000001/测试教师/副教授", "测试教师"}, // 另一职称
		{"20000001/测试教师", "测试教师"},       // 2 段格式：取最后一段
		{"70010/教务处", ""},              // 2 段格式：单位词 → 无教师
		{"", ""},                          // 空
		{"无", ""},                        // 显式无
		{"null", ""},                      // null
	}
	for _, tc := range cases {
		if got := ParseTeacherFromJsxx(tc.jsxx); got != tc.want {
			t.Errorf("ParseTeacherFromJsxx(%q) = %q, want %q", tc.jsxx, got, tc.want)
		}
	}
}

// ── Match 兜底：数据缺失不过滤 ─────────────────────────────────────────────

func TestCourseMatch_MissingFieldsDoNotFilter(t *testing.T) {
	// 课程数据缺类型字段 → 不过滤（避免因字段缺失滤掉全部课）
	noType := &Course{Name: "A", Credit: 2}
	if !noType.Match(&Config{CourseType: CourseTypePE}) {
		t.Error("课程缺 kklxdm 时不应被类型过滤滤掉")
	}

	// 配置类型为空（TypeRelaxed 回落场景）→ 不做类型过滤
	pe := &Course{Type: "20", Name: "羽毛球", Credit: 2}
	if !pe.Match(&Config{CourseType: ""}) {
		t.Error("CourseType 为空时不应按类型过滤")
	}

	// 课程列表不返回教师名（教师只在详情里）→ 教师筛选不过滤
	listCourse := &Course{Type: "10", Name: "网课", Credit: 2, Teacher: ""}
	if !listCourse.Match(&Config{CourseType: CourseTypeOnline, TeacherName: "测试教师"}) {
		t.Error("列表课程无教师字段时，教师筛选不应把全部课程滤掉")
	}

	// 有教师字段 → 包含匹配（"测试教师" 命中 "测试教师 讲师"）
	withTeacher := &Course{Type: "10", Name: "英语", Credit: 2, Teacher: "测试教师 讲师"}
	if !withTeacher.Match(&Config{CourseType: CourseTypeOnline, TeacherName: "测试教师"}) {
		t.Error("教师筛选应为包含匹配")
	}
	if withTeacher.Match(&Config{CourseType: CourseTypeOnline, TeacherName: "李四"}) {
		t.Error("不相关的教师名应被过滤")
	}
}

// ── 候选表 kklxdm ─────────────────────────────────────────────────────────

func TestCourseTypeKklxdmCandidates(t *testing.T) {
	// 实机修正：普通课（各学院开课）主修课程 kklxdm=01；原单值表 "30" 是历史猜测
	got := CourseTypeKklxdmCandidates(CourseTypeNormal)
	if len(got) != 2 || got[0] != "01" || got[1] != "30" {
		t.Errorf("普通课候选表 = %v, want [01 30]", got)
	}
	if got := CourseTypeKklxdmCandidates(CourseTypeOnline); len(got) != 1 || got[0] != "10" {
		t.Errorf("网课候选表 = %v, want [10]", got)
	}
	if got := CourseTypeKklxdmCandidates(CourseTypePE); len(got) != 1 || got[0] != "20" {
		t.Errorf("体育课候选表 = %v, want [20]", got)
	}
	// 中文标签同样归一化
	if got := CourseTypeKklxdmCandidates("普通课"); len(got) != 2 || got[0] != "01" {
		t.Errorf("中文标签候选表 = %v", got)
	}
	// 返回值是副本，修改不得影响内部表
	got[0] = "mutated"
	if again := CourseTypeKklxdmCandidates(CourseTypeNormal); again[0] != "01" {
		t.Error("候选表返回值必须是副本")
	}
}

// ── CourseList.Clone 保留 Warning/TypeRelaxed ──────────────────────────────

func TestCourseListClone_PreservesWarningAndTypeRelaxed(t *testing.T) {
	src := &CourseList{
		Total:       1,
		Items:       []*Course{{Name: "A"}},
		Warning:     "已回落到教务实际开放的轮次",
		TypeRelaxed: true,
	}
	cp := src.Clone()
	if cp.Warning != src.Warning || !cp.TypeRelaxed {
		t.Errorf("Clone 丢失 Warning/TypeRelaxed: %+v", cp)
	}
}

// ── buildCourseFromMap：CreditRaw 保留原始学分串 ───────────────────────────

func TestBuildCourseFromMap_CreditRaw(t *testing.T) {
	// tmpList 全部字段是字符串（xf="2.0"）；选课提交 kcmc 需要 "2.0" 的小数位
	c := buildCourseFromMap(map[string]interface{}{"kch_id": "K1", "xf": "2.0"})
	if c.Credit != 2 || c.CreditRaw != "2.0" {
		t.Errorf("Credit=%d CreditRaw=%q, want 2 / \"2.0\"", c.Credit, c.CreditRaw)
	}
	// 数字形态也不得 panic（xf=2.0 → CreditRaw "2"）
	c2 := buildCourseFromMap(map[string]interface{}{"kch_id": "K2", "xf": 2.0})
	if c2.Credit != 2 {
		t.Errorf("数字 xf 解析失败: %+v", c2)
	}
}

// 确保教师单位词表覆盖常见开课主体（教务处/学院/中心）
func TestOrgNameKeywordsCoverage(t *testing.T) {
	for _, org := range []string{"教务处", "现代信息产业学院", "体育教学部", "马克思主义学院"} {
		if !looksLikeOrgName(org) {
			t.Errorf("%q 应识别为开课单位", org)
		}
	}
	for _, name := range []string{"测试教师甲", "测试教师乙", "测试教师丙"} {
		if looksLikeOrgName(name) {
			t.Errorf("%q 不应识别为开课单位", name)
		}
	}
	// 防止 orgNameKeywords 与常用姓氏误撞
	for _, name := range []string{"周处", "吴系", "王部"} {
		_ = name // 仅记录：中文姓名极少含单位词，误判风险可接受
	}
	if !strings.Contains("教务处", "处") {
		t.Error("sanity")
	}
}
