package model

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Course 课程信息
type Course struct {
	// 基本信息
	ID     string `json:"kch_id"`
	Name   string `json:"kcmc"`
	Number string `json:"kch"`
	Credit int    `json:"xf"`
	// CreditRaw 学分的原始字符串形式（如 "2.0"）。
	// 选课提交的 kcmc 需要按浏览器原样格式回显学分（"(kch)名-2.0学分"），
	// 只存 int 会丢掉 "2.0" 的小数位。
	CreditRaw string
	Type     string `json:"kklxdm"` // 课程类型代码

	// 教学班信息
	ClassID   string `json:"jxb_id"`
	ClassName string `json:"jxbmc"`

	// 选课状态
	Selected int `json:"yxzrs"` // 已选人数
	Capacity int `json:"zrs"`   // 总人数

	// 上课信息
	Teacher  string `json:"jsm"`
	Room     string `json:"jsmc"`
	WeekTime string `json:"sksj"` // 上课时间（周几第几节）

	// 其他
	RowNum int `json:"kcrow"` // 行号

	// 扩展字段（从课程详情获取）
	Extra *CourseExtra
}

// CourseExtra 课程扩展信息（由 GetClassInfo 获取）
type CourseExtra struct {
	ClassInfo string // 教室名称
	ExamInfo  string // 上课时间
	Remark    string // 课程备注
	// DoJxbID：正方 V9 的加密教学班长 ID，选课时必须使用此值（非短 jxb_id）
	// 字段名在接口响应中为 do_jxb_id
	DoJxbID string
	// Teacher：详情 jsxx 解析出的教师名（课程列表接口不返回教师名，
	// 教师只在详情里 —— 实机 288/288 门课无 jsm/jsxx 字段）
	Teacher string
	// Capacity / Selected：详情下发的容量与已选人数（jxbrl / jxbrs）。
	// 容量以服务端实时值为准，本地缓存不得用于阻止提交。
	Capacity  int
	Selected  int
}

// CourseList 课程列表
type CourseList struct {
	Total int
	Items []*Course

	// Warning 服务端返回课程但存在需要用户知晓的偏差时给出说明
	//（如"所选课程类型当前无开放轮次，已回落到教务实际开放的轮次"）。
	Warning string
	// TypeRelaxed 本次查询未按配置的课程类型过滤（回落到了其他开放轮次）。
	// robber 据此用"无类型偏好"的配置副本做本地匹配，避免把该轮次全部课程
	// 滤掉导致用户一门课都看不到（死路）。
	TypeRelaxed bool
}

// Clone 拷贝课程列表（Course 按值复制，Extra 指针置空）
//
// 用途：课程列表短 TTL 缓存需要把同一份查询结果分发给多个并发 Worker，
// 而 robCourse 会写入 course.Extra（详情接口返回值）。若共享同一批指针，
// 多 Worker 并发写 Extra 会产生数据竞争；Clone 让每个 Worker 拿到独立副本。
func (cl *CourseList) Clone() *CourseList {
	if cl == nil {
		return nil
	}
	out := &CourseList{
		Total:       cl.Total,
		Items:       make([]*Course, 0, len(cl.Items)),
		Warning:     cl.Warning,
		TypeRelaxed: cl.TypeRelaxed,
	}
	for _, c := range cl.Items {
		if c == nil {
			continue
		}
		cp := *c
		cp.Extra = nil
		out.Items = append(out.Items, &cp)
	}
	return out
}

// Match 检查课程是否匹配筛选条件
//
// 课程类型比较使用 model 层的统一映射：cfg.CourseType 允许是中文标签或英文代码，
// 内部一律归一化为规范代码后再比较，避免"展示值当业务值"导致过滤失效。
//
// 两条兜底（防坑守则 #1：过滤/匹配必须对数据缺失兜底，未知不过滤）：
//  1. cfg.CourseType 为空 → 不做类型过滤（TypeRelaxed 回落场景）；
//  2. c.Type 为空 → 课程数据缺类型字段，不过滤（避免因字段缺失滤掉全部课）。
func (c *Course) Match(cfg *Config) bool {
	// 检查课程类型
	if cfg.CourseType != "" && c.Type != "" {
		switch NormalizeCourseType(cfg.CourseType) {
		case CourseTypeOnline:
			if c.Type != CourseTypeCodeKklxdm(CourseTypeOnline) {
				return false
			}
		case CourseTypePE:
			if c.Type != CourseTypeCodeKklxdm(CourseTypePE) {
				return false
			}
		case CourseTypeNormal:
			// 普通课：排除网课与体育课对应的类别码
			if c.Type == CourseTypeCodeKklxdm(CourseTypeOnline) || c.Type == CourseTypeCodeKklxdm(CourseTypePE) {
				return false
			}
		}
	}

	// 检查课程名称（不区分大小写）
	if cfg.CourseName != "" && !strings.Contains(strings.ToLower(c.Name), strings.ToLower(cfg.CourseName)) {
		return false
	}

	// 检查老师（包含匹配）
	// 修复（V3.0 bug）：原实现是精确匹配（c.Teacher != cfg.TeacherName），
	// 而课程列表接口根本不返回教师名（教师只在详情 jsxx 里），用户填了教师
	// 筛选就会把全部课程静默滤掉。现在：列表无教师字段时不过滤；
	// 有教师字段时按包含匹配（"测试教师" 能命中 "测试教师 讲师" 这类展示值）。
	if cfg.TeacherName != "" && c.Teacher != "" &&
		!strings.Contains(strings.ToLower(c.Teacher), strings.ToLower(cfg.TeacherName)) {
		return false
	}

	// 检查课程编号（不区分大小写）
	if cfg.CourseNumber != "" && !strings.Contains(strings.ToLower(c.Number), strings.ToLower(cfg.CourseNumber)) {
		return false
	}

	// 检查学分
	if cfg.MinCredit > 0 && c.Credit < cfg.MinCredit {
		return false
	}

	return true
}

// IsFull 检查是否满员
func (c *Course) IsFull() bool {
	// Capacity 为 0 时视为未知容量，不过滤（避免因数据缺失漏掉课程）。
	if c.Capacity <= 0 {
		return false
	}
	return c.Selected >= c.Capacity
}

// ─────────────────────────────────────────────────────────────────────────────
// 教师名解析（详情 jsxx 字段）
//
// 实机抓包结论（2026-09-15）：jsxx 有三种共存格式，原实现三种全错：
//   - "70010/教务处/无"      （网课，教务处开课）→ 正确解析为 ""（无教师）
//   - "20000001/测试教师/讲师"（主修课）        → 正确解析为 "测试教师"
//   - "20000002/测试教师乙/无"（职称缺失）      → 正确解析为 "测试教师乙"
// 判别规则：第 3 段是职称 → 取第 2 段；第 3 段为空/"无" → 看第 2 段是否
// 单位词（处/学院/中心…），是单位词则无教师，否则是姓名。
// 兼容 2 段格式（工号/姓名）：取最后一段。
// ─────────────────────────────────────────────────────────────────────────────

// teacherTitleKeywords 职称关键词（命中即认为该段是职称而非姓名）
var teacherTitleKeywords = []string{
	"教授", "讲师", "助教", "工程师", "实验师", "教练", "研究员",
	"导师", "教师", "技师", "医师", "馆员", "编审", "记者", "正高", "副高",
}

// orgNameKeywords 单位词关键词（命中即认为该段是开课单位而非教师姓名）
var orgNameKeywords = []string{
	"处", "学院", "中心", "部", "系", "组", "室", "园", "校区", "办公室", "联盟",
}

// ParseTeacherFromJsxx 从课程详情的 jsxx 字段解析教师名。
// 解析不出教师名（网课/教务处开课）时返回空字符串 —— 这是正确业务结果，
// 调用方不得把空字符串当作"解析失败"。
func ParseTeacherFromJsxx(jsxx string) string {
	s := strings.TrimSpace(jsxx)
	if s == "" || s == "无" || s == "null" {
		return ""
	}

	parts := strings.Split(s, "/")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	switch {
	case len(parts) >= 3:
		// 三段：[工号/姓名/职称]
		name, title := parts[1], parts[2]
		if containsAnyKeyword(title, teacherTitleKeywords) {
			return name // 第 3 段是职称 → 第 2 段是姓名
		}
		if looksLikeOrgName(name) {
			return "" // 教务处开课的网课：无教师
		}
		return name
	case len(parts) == 2:
		// 两段：[工号/姓名]，取最后一段
		last := parts[1]
		if looksLikeOrgName(last) {
			return ""
		}
		return last
	default:
		// 单段：可能直接就是姓名
		if looksLikeOrgName(s) {
			return ""
		}
		return s
	}
}

// containsAnyKeyword s 包含任一词表关键词则为 true
func containsAnyKeyword(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// looksLikeOrgName s 看起来是开课单位（而非教师姓名）则为 true
func looksLikeOrgName(s string) bool {
	return s != "" && containsAnyKeyword(s, orgNameKeywords)
}

// String 格式化课程信息
func (c *Course) String() string {
	return c.Name + " " + c.Teacher + " " + c.WeekTime
}

// ─────────────────────────────────────────────────────────────────────────────
// 字段容错解析
//
// 正方 V9 各部署对同一字段的类型并不统一：学分可能是 2.0（数字）也可能是 "2.0"
// （字符串），选课人数同理。V3.0 直接 `item["xf"].(float64)` 断言，一旦是字符串就
// 静默归零，后果是：
//   - Credit=0 → 被 MinCredit 过滤掉 → 永远"没有符合条件的课程"
//   - Capacity=0 → IsFull() 恒 false → 反复去撞满员的课
//   - Selected=0 → 容量显示 0/0
// 因此统一用下面两个函数取值，兼容 float64 / string / json.Number / bool。
// ─────────────────────────────────────────────────────────────────────────────

// ToInt 把任意 JSON 值转换为 int，无法解析时返回 (0, false)
func ToInt(v interface{}) (int, bool) {
	switch t := v.(type) {
	case nil:
		return 0, false
	case float64:
		return int(t), true
	case float32:
		return int(t), true
	case int:
		return t, true
	case int64:
		return int(t), true
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i), true
		}
		if f, err := t.Float64(); err == nil {
			return int(f), true
		}
		return 0, false
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		if i, err := strconv.Atoi(s); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int(f), true
		}
		return 0, false
	case bool:
		if t {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// ToString 把任意 JSON 值转换为 string，无法解析时返回 ("", false)
func ToString(v interface{}) (string, bool) {
	switch t := v.(type) {
	case nil:
		return "", false
	case string:
		return t, true
	case float64:
		if t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10), true
		}
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case json.Number:
		return t.String(), true
	case bool:
		return strconv.FormatBool(t), true
	}
	return "", false
}

// previewText 截取字符串前 n 个字符，用于错误信息中展示原始响应片段
func previewText(s string, n int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= n {
		return string(runes)
	}
	return string(runes[:n]) + "..."
}

// extractHTMLText 从 HTML 中提取可读的纯文本（去除标签、压缩空白）
// 用于在服务端返回 HTML 错误页时给出人类可读的错误提示
func extractHTMLText(html string) string {
	// 去掉 <script>...</script> 和 <style>...</style> 块
	for _, tag := range []string{"script", "style"} {
		for {
			open := strings.Index(strings.ToLower(html), "<"+tag)
			if open < 0 {
				break
			}
			close := strings.Index(strings.ToLower(html[open:]), "</"+tag+">")
			if close < 0 {
				break
			}
			html = html[:open] + " " + html[open+close+len("</"+tag+">"):]
		}
	}
	// 去掉所有 HTML 标签
	inTag := false
	var buf strings.Builder
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			buf.WriteByte(' ')
		case !inTag:
			buf.WriteRune(r)
		}
	}
	// 压缩连续空白，截取前 120 字符
	text := strings.Join(strings.Fields(buf.String()), " ")
	if len([]rune(text)) > 120 {
		runes := []rune(text)
		text = string(runes[:120]) + "..."
	}
	return text
}

// buildCourseFromMap 从单个 JSON 对象构建 Course（字段级容错）
func buildCourseFromMap(item map[string]interface{}) *Course {
	course := &Course{}

	if v, ok := ToString(item["kch_id"]); ok {
		course.ID = v
	}
	if v, ok := ToString(item["kcmc"]); ok {
		course.Name = v
	}
	if v, ok := ToString(item["kch"]); ok {
		course.Number = v
	}
	if v, ok := ToString(item["xf"]); ok {
		course.CreditRaw = v
	}
	if v, ok := ToInt(item["xf"]); ok {
		course.Credit = v
	}
	if v, ok := ToString(item["kklxdm"]); ok {
		course.Type = v
	}
	if v, ok := ToString(item["jxb_id"]); ok {
		course.ClassID = v
	}
	if v, ok := ToString(item["jxbmc"]); ok {
		course.ClassName = v
	}
	if v, ok := ToInt(item["yxzrs"]); ok {
		course.Selected = v
	}
	// 总人数（容量）——V3.0 曾遗漏此字段，导致 IsFull() 恒为 false
	if v, ok := ToInt(item["zrs"]); ok {
		course.Capacity = v
	}
	if v, ok := ToString(item["jsm"]); ok {
		course.Teacher = v
	}
	if v, ok := ToString(item["jsmc"]); ok {
		course.Room = v
	}
	if v, ok := ToString(item["sksj"]); ok {
		course.WeekTime = v
	}
	if v, ok := ToInt(item["kcrow"]); ok {
		course.RowNum = v
	}

	return course
}

// buildCourseList 由若干 JSON 对象构建 CourseList
func buildCourseList(items []map[string]interface{}) *CourseList {
	list := &CourseList{Items: make([]*Course, 0, len(items))}
	for _, item := range items {
		list.Items = append(list.Items, buildCourseFromMap(item))
	}
	list.Total = len(list.Items)
	return list
}

// ParseCourseList 解析课程列表JSON
//
// 兼容三种响应形态，避免"0 门课"被当作解析失败：
//  1. {"tmpList":[...]}  —— 选课列表接口的标准形态
//  2. [...]              —— 部分接口（如已选课程）直接返回数组
//  3. HTML 错误页         —— 提前识别并给出人类可读提示
func ParseCourseList(data []byte) (*CourseList, error) {
	// 服务端在选课未开放、会话失效等情况下会返回 HTML 而非 JSON
	// 提前检测，给出可读错误，避免 JSON 解析器报 "invalid character '<'"
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("服务端返回了空响应（可能会话已过期或网络中断）")
	}
	if strings.HasPrefix(trimmed, "<") {
		hint := extractHTMLText(trimmed)
		if hint == "" {
			hint = "（无法提取页面文字）"
		}
		return nil, fmt.Errorf("服务端返回了HTML而非课程数据（可能选课未开放或会话已过期）: %s", hint)
	}

	// 形态 2：根节点直接是数组
	if strings.HasPrefix(trimmed, "[") {
		var arr []map[string]interface{}
		if err := json.Unmarshal(data, &arr); err == nil {
			return buildCourseList(arr), nil
		}
	}

	// 形态 1：{"tmpList":[...]}
	var result struct {
		TmpList []map[string]interface{} `json:"tmpList"`
		Sfxsjc  string                   `json:"sfxsjc"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析课程列表JSON失败: %w（响应前120字符: %s）", err, previewText(trimmed, 120))
	}

	// tmpList 缺失/为 null 时 TmpList 为 nil → 由 buildCourseList 归一为长度 0 的切片。
	// 重点：0 门课是合法的业务结果，不是错误。
	return buildCourseList(result.TmpList), nil
}
