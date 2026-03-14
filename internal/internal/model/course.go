package model

import (
	"encoding/json"
	"strings"
)

// Course 课程信息
type Course struct {
	// 基本信息
	ID          string `json:"kch_id"`
	Name        string `json:"kcmc"`
	Number      string `json:"kch"`
	Credit      int    `json:"xf"`
	Type        string `json:"kklxdm"` // 课程类型代码

	// 教学班信息
	ClassID     string `json:"jxb_id"`
	ClassName   string `json:"jxbmc"`

	// 选课状态
	Selected    int    `json:"yxzrs"` // 已选人数
	Capacity    int    `json:"zrs"`   // 总人数

	// 上课信息
	Teacher     string `json:"jsm"`
	Room        string `json:"jsmc"`
	WeekTime    string `json:"sksj"`   // 上课时间（周几第几节）

	// 其他
	RowNum      int    `json:"kcrow"`  // 行号

	// 扩展字段（从课程详情获取）
	Extra       *CourseExtra
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

// Match 检查课程是否匹配筛选条件
func (c *Course) Match(cfg *Config) bool {
	// 检查课程类型
	if cfg.CourseType == "online" && c.Type != "10" {
		return false
	}
	if cfg.CourseType == "pe" && c.Type != "20" {
		return false
	}
	if cfg.CourseType == "normal" && (c.Type == "10" || c.Type == "20") {
		return false
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
	return c.Selected >= c.Capacity
}

// String 格式化课程信息
func (c *Course) String() string {
	return c.Name + " " + c.Teacher + " " + c.WeekTime
}

// ParseCourseList 解析课程列表JSON
func ParseCourseList(data []byte) (*CourseList, error) {
	var result struct {
		TmpList []map[string]interface{} `json:"tmpList"`
		Sfxsjc  string                   `json:"sfxsjc"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	list := &CourseList{
		Items: make([]*Course, 0, len(result.TmpList)),
	}

	for _, item := range result.TmpList {
		course := &Course{}

		// 解析基础字段
		if v, ok := item["kch_id"].(string); ok {
			course.ID = v
		}
		if v, ok := item["kcmc"].(string); ok {
			course.Name = v
		}
		if v, ok := item["kch"].(string); ok {
			course.Number = v
		}
		if v, ok := item["xf"].(float64); ok {
			course.Credit = int(v)
		}
		if v, ok := item["kklxdm"].(string); ok {
			course.Type = v
		}
		if v, ok := item["jxb_id"].(string); ok {
			course.ClassID = v
		}
		if v, ok := item["jxbmc"].(string); ok {
			course.ClassName = v
		}
		if v, ok := item["yxzrs"].(float64); ok {
			course.Selected = int(v)
		}
		if v, ok := item["kcrow"].(float64); ok {
			course.RowNum = int(v)
		}

		list.Items = append(list.Items, course)
	}

	list.Total = len(list.Items)
	return list, nil
}
