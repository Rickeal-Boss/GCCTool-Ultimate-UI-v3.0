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
	Type   string `json:"kklxdm"` // 课程类型代码

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
}

// CourseList 课程列表
type CourseList struct {
	Total int
	Items []*Course
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
		Total: cl.Total,
		Items: make([]*Course, 0, len(cl.Items)),
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
func (c *Course) Match(cfg *Config) bool {
	// 检查课程类型
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

	// 检查课程名称（不区分大小写）
	if cfg.CourseName != "" && !strings.Contains(strings.ToLower(c.Name), strings.ToLower(cfg.CourseName)) {
		return false
	}

	// 检查老师（精确匹配）
	if cfg.TeacherName != "" && c.Teacher != cfg.TeacherName {
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
