package model

import "fmt"

// Config 用户配置
type Config struct {
	// 账号信息
	Username string
	Password string

	// 节点配置
	NodeURL string
	Agent   string

	// 选课时间
	Hour    int
	Minute  int
	Advance int // 提前多少分钟开始

	// 并发配置
	Threads int

	// 课程筛选
	CourseType   string // 规范代码：online / pe / normal（中文标签会在 GetConfig 时归一化）
	CourseName   string
	TeacherName  string
	CourseNumber string
	MinCredit    int

	// 课程分类（多选）
	Categories map[string]bool
}

// 并发线程数上下限（过高会触发服务端限流/封禁）
const (
	MinThreads = 1
	MaxThreads = 50
)

// NewConfig 创建默认配置
func NewConfig() *Config {
	return &Config{
		NodeURL:    "节点1（推荐）",
		Hour:       12,
		Minute:     30,
		Advance:    1,
		Threads:    10,
		CourseType: CourseTypeOnline,
		MinCredit:  2,
		Categories: make(map[string]bool),
	}
}

// Validate 校验配置的合法范围。
//
// 历史 Bug（V3.0）：parseInt 解析失败静默返回 0，线程数会被写成 0，
// startWorkers 里 wg.Add(0) 启动 0 个 goroutine —— 界面显示"抢课中"，
// 实际一个请求都不发。这里在启动前做显式校验，把问题暴露给用户而不是静默失效。
func (c *Config) Validate() error {
	if c.Hour < 0 || c.Hour > 23 {
		return fmt.Errorf("选课时间的小时需在 0~23 之间（当前: %d）", c.Hour)
	}
	if c.Minute < 0 || c.Minute > 59 {
		return fmt.Errorf("选课时间的分钟需在 0~59 之间（当前: %d）", c.Minute)
	}
	if c.Advance < 0 {
		return fmt.Errorf("提前开抢时间不能为负数（当前: %d）", c.Advance)
	}
	if c.Threads < MinThreads || c.Threads > MaxThreads {
		return fmt.Errorf("并发线程数需在 %d~%d 之间（当前: %d）", MinThreads, MaxThreads, c.Threads)
	}
	if c.MinCredit < 0 {
		return fmt.Errorf("最低学分不能为负数（当前: %d）", c.MinCredit)
	}
	return nil
}

// Normalize 把可容错的字段归一化（中文课程类型 → 规范代码），便于后续逻辑直接使用。
func (c *Config) Normalize() {
	c.CourseType = NormalizeCourseType(c.CourseType)
}
