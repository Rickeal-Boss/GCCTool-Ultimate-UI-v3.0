package model

// Config 用户配置
type Config struct {
	// 账号信息
	Username string
	Password string

	// 节点配置
	NodeURL string
	Agent   string

	// 选课时间
	Hour   int
	Minute int
	Advance int // 提前多少分钟开始

	// 并发配置
	Threads int

	// 课程筛选
	CourseType     string // "online", "pe", "normal"
	CourseName     string
	TeacherName    string
	CourseNumber   string
	MinCredit      int

	// 课程分类（多选）
	Categories map[string]bool

	// 其他
	KkkzId   string
	Kklxdm   string
	FirstXkkzId string
	FirstKklxdm string
}

// NewConfig 创建默认配置
func NewConfig() *Config {
	return &Config{
		NodeURL:    "节点1（推荐）",
		Hour:       12,
		Minute:     30,
		Advance:    1,
		Threads:    10,
		CourseType: "online",
		MinCredit:  2,
		Categories: make(map[string]bool),
	}
}
