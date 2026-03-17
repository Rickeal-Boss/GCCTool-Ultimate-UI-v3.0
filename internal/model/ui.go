package model

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const maxLogLines = 500

// UIComponents UI组件集合
type UIComponents struct {
	// 输入框
	UsernameEntry    *widget.Entry
	PasswordEntry    *widget.Entry
	NodeSelect       *widget.Select
	AgentEntry       *widget.Entry
	HourEntry        *widget.Entry
	MinuteEntry      *widget.Entry
	AdvanceEntry     *widget.Entry
	ThreadEntry      *widget.Entry
	CourseTypeRadio  *widget.RadioGroup
	CourseNameEntry  *widget.Entry
	TeacherEntry     *widget.Entry
	CourseNumEntry   *widget.Entry
	MinCreditEntry   *widget.Entry

	// 复选框（课程分类）
	CategoryChecks  []*widget.Check

	// 按钮
	StartBtn        *widget.Button
	StopBtn         *widget.Button
	CopyLogBtn      *widget.Button

	// 日志
	LogLabel        *widget.Label
	LogScroll       *container.Scroll
	logLines        []string // 内部切片，限制行数

	// 课程列表
	CourseList      *widget.List
	CourseData      []*Course
}

// NewUIComponents 创建UI组件
func NewUIComponents() *UIComponents {
	ui := &UIComponents{
		UsernameEntry:   widget.NewEntry(),
		PasswordEntry:   widget.NewPasswordEntry(),
		NodeSelect:      widget.NewSelect(nil, nil),
		AgentEntry:      widget.NewEntry(),
		HourEntry:       widget.NewEntry(),
		MinuteEntry:     widget.NewEntry(),
		AdvanceEntry:    widget.NewEntry(),
		ThreadEntry:     widget.NewEntry(),
		CourseTypeRadio: widget.NewRadioGroup(nil, nil),
		CourseNameEntry: widget.NewEntry(),
		TeacherEntry:    widget.NewEntry(),
		CourseNumEntry:  widget.NewEntry(),
		MinCreditEntry:  widget.NewEntry(),
		CategoryChecks:  make([]*widget.Check, 9),
		StartBtn:       widget.NewButton("启动", nil),
		StopBtn:        widget.NewButton("停止", nil),
		CopyLogBtn:     widget.NewButton("拷贝日志", nil),
		CourseData:     make([]*Course, 0),

		// 关键修复：使用同一个Label对象
		LogLabel: widget.NewLabel(""),
		logLines: make([]string, 0, maxLogLines),
	}

	// LogScroll直接包装LogLabel
	ui.LogScroll = container.NewScroll(ui.LogLabel)

	// 设置占位符
	ui.UsernameEntry.SetPlaceHolder("请输入学号")
	ui.PasswordEntry.SetPlaceHolder("请输入密码")
	ui.AgentEntry.SetPlaceHolder("http://example.com:port")
	ui.CourseNameEntry.SetPlaceHolder("例如: 羽毛球")
	ui.TeacherEntry.SetPlaceHolder("例如: 张三")
	ui.CourseNumEntry.SetPlaceHolder("例如: 0200200200,0200200201")

	return ui
}

// GetConfig 从UI获取配置
func (ui *UIComponents) GetConfig() *Config {
	cfg := NewConfig()

	cfg.Username = ui.UsernameEntry.Text
	cfg.Password = ui.PasswordEntry.Text
	cfg.NodeURL = ui.NodeSelect.Selected
	cfg.Agent = ui.AgentEntry.Text

	if h := ui.HourEntry.Text; h != "" {
		cfg.Hour = parseInt(h)
	}
	if m := ui.MinuteEntry.Text; m != "" {
		cfg.Minute = parseInt(m)
	}
	if a := ui.AdvanceEntry.Text; a != "" {
		cfg.Advance = parseInt(a)
	}
	if t := ui.ThreadEntry.Text; t != "" {
		cfg.Threads = parseInt(t)
	}

	cfg.CourseType = ui.CourseTypeRadio.Selected
	cfg.CourseName = ui.CourseNameEntry.Text
	cfg.TeacherName = ui.TeacherEntry.Text
	cfg.CourseNumber = ui.CourseNumEntry.Text
	if c := ui.MinCreditEntry.Text; c != "" {
		cfg.MinCredit = parseInt(c)
	}

	// 解析课程分类
	for i, check := range ui.CategoryChecks {
		if check.Checked {
			cfg.Categories[getCategoryLabel(i)] = true
		}
	}

	return cfg
}

// SetConfig 设置UI配置
func (ui *UIComponents) SetConfig(cfg *Config) {
	ui.UsernameEntry.SetText(cfg.Username)
	ui.PasswordEntry.SetText(cfg.Password)
	ui.NodeSelect.SetSelected(cfg.NodeURL)
	ui.AgentEntry.SetText(cfg.Agent)
	ui.HourEntry.SetText(toString(cfg.Hour))
	ui.MinuteEntry.SetText(toString(cfg.Minute))
	ui.AdvanceEntry.SetText(toString(cfg.Advance))
	ui.ThreadEntry.SetText(toString(cfg.Threads))
	ui.CourseTypeRadio.SetSelected(cfg.CourseType)
	ui.CourseNameEntry.SetText(cfg.CourseName)
	ui.TeacherEntry.SetText(cfg.TeacherName)
	ui.CourseNumEntry.SetText(cfg.CourseNumber)
	ui.MinCreditEntry.SetText(toString(cfg.MinCredit))
}

// UpdateCourseList 更新课程列表
func (ui *UIComponents) UpdateCourseList(courses []*Course) {
	ui.CourseData = courses
	if ui.CourseList != nil {
		ui.CourseList.Refresh()
	}
}

// AppendLog 追加日志（v2.5 线程安全说明）
//
// Fyne v2.5 对所有 canvas 写操作（SetText、Refresh 等）内部使用容器锁保护，
// 可以在任意 goroutine 中直接调用，无需额外同步。
// （v2.6 移除了容器锁并引入 fyne.Do；本项目锁定 v2.5.3，不使用 fyne.Do。）
//
// 日志行数超过 maxLogLines 时丢弃最旧的 1/4，防止内存无限增长。
func (ui *UIComponents) AppendLog(message string) {
	ui.logLines = append(ui.logLines, message)
	if len(ui.logLines) > maxLogLines {
		keep := maxLogLines * 3 / 4
		ui.logLines = ui.logLines[len(ui.logLines)-keep:]
	}
	ui.LogLabel.SetText(strings.Join(ui.logLines, "\n"))
	ui.LogScroll.ScrollToBottom()
}

// ClearLog 清空日志
func (ui *UIComponents) ClearLog() {
	ui.logLines = ui.logLines[:0]
	ui.LogLabel.SetText("")
}

func parseInt(s string) int {
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

func toString(i int) string {
	return fmt.Sprintf("%d", i)
}

func getCategoryLabel(index int) string {
	labels := []string{
		"科技类", "人文类", "经营类",
		"体育类", "创新创业类", "艺术类",
		"自然科学类", "思政类", "其他类",
	}
	if index < len(labels) {
		return labels[index]
	}
	return ""
}
