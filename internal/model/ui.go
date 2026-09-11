package model

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const maxLogLines = 500

// UIComponents UI组件集合
type UIComponents struct {
	// 输入框
	UsernameEntry   *widget.Entry
	PasswordEntry   *widget.Entry
	NodeSelect      *widget.Select
	AgentEntry      *widget.Entry
	HourEntry       *widget.Entry
	MinuteEntry     *widget.Entry
	AdvanceEntry    *widget.Entry
	ThreadEntry     *widget.Entry
	CourseTypeRadio *widget.RadioGroup
	CourseNameEntry *widget.Entry
	TeacherEntry    *widget.Entry
	CourseNumEntry  *widget.Entry
	MinCreditEntry  *widget.Entry

	// 复选框（课程分类）
	CategoryChecks []*widget.Check

	// 按钮
	StartBtn   *widget.Button
	StopBtn    *widget.Button
	CopyLogBtn *widget.Button

	// 日志
	LogLabel *widget.Label
	LogScroll *container.Scroll
	logLines []string // 内部切片，限制行数

	// 课程列表
	CourseList *widget.List
	CourseData []*Course
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
		StartBtn:        widget.NewButton("启动", nil),
		StopBtn:         widget.NewButton("停止", nil),
		CopyLogBtn:      widget.NewButton("拷贝日志", nil),
		CourseData:      make([]*Course, 0),

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
//
// 与 V3.0 的关键差异：数字输入框非法时返回错误，而不是静默回退成 0。
// 历史 Bug：fmt.Sscanf 解析失败返回 0，线程数被写成 0 → wg.Add(0) 启动 0 个
// goroutine → 界面显示"抢课中"但一个请求都不发，用户完全无法察觉。
func (ui *UIComponents) GetConfig() (*Config, error) {
	cfg := NewConfig()

	cfg.Username = strings.TrimSpace(ui.UsernameEntry.Text)
	cfg.Password = ui.PasswordEntry.Text
	cfg.NodeURL = ui.NodeSelect.Selected
	cfg.Agent = strings.TrimSpace(ui.AgentEntry.Text)

	var err error
	if cfg.Hour, err = parseIntField("选课时间（时）", ui.HourEntry.Text, cfg.Hour); err != nil {
		return nil, err
	}
	if cfg.Minute, err = parseIntField("选课时间（分）", ui.MinuteEntry.Text, cfg.Minute); err != nil {
		return nil, err
	}
	if cfg.Advance, err = parseIntField("提前开抢（分钟）", ui.AdvanceEntry.Text, cfg.Advance); err != nil {
		return nil, err
	}
	if cfg.Threads, err = parseIntField("并发线程数", ui.ThreadEntry.Text, cfg.Threads); err != nil {
		return nil, err
	}
	if cfg.MinCredit, err = parseIntField("最低学分", ui.MinCreditEntry.Text, cfg.MinCredit); err != nil {
		return nil, err
	}

	// UI 展示的是中文标签，统一归一化为规范代码（online/pe/normal）
	cfg.CourseType = NormalizeCourseType(ui.CourseTypeRadio.Selected)
	cfg.CourseName = strings.TrimSpace(ui.CourseNameEntry.Text)
	cfg.TeacherName = strings.TrimSpace(ui.TeacherEntry.Text)
	cfg.CourseNumber = strings.TrimSpace(ui.CourseNumEntry.Text)

	// 解析课程分类
	for i, check := range ui.CategoryChecks {
		if check != nil && check.Checked {
			cfg.Categories[getCategoryLabel(i)] = true
		}
	}

	return cfg, nil
}

// SetConfig 设置UI配置
func (ui *UIComponents) SetConfig(cfg *Config) {
	ui.UsernameEntry.SetText(cfg.Username)
	ui.PasswordEntry.SetText(cfg.Password)
	ui.NodeSelect.SetSelected(cfg.NodeURL)
	ui.AgentEntry.SetText(cfg.Agent)
	ui.HourEntry.SetText(strconv.Itoa(cfg.Hour))
	ui.MinuteEntry.SetText(strconv.Itoa(cfg.Minute))
	ui.AdvanceEntry.SetText(strconv.Itoa(cfg.Advance))
	ui.ThreadEntry.SetText(strconv.Itoa(cfg.Threads))
	// 规范代码 → 中文标签，避免 SetSelected 传入英文代码导致单选组无选中项
	ui.CourseTypeRadio.SetSelected(CourseTypeLabel(cfg.CourseType))
	ui.CourseNameEntry.SetText(cfg.CourseName)
	ui.TeacherEntry.SetText(cfg.TeacherName)
	ui.CourseNumEntry.SetText(cfg.CourseNumber)
	ui.MinCreditEntry.SetText(strconv.Itoa(cfg.MinCredit))
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

// parseIntField 解析整数字段。
//
// 空值 → 返回默认值（用户没填就用默认，不报错）；
// 非法值 → 返回带字段名的错误，由调用方提示用户。
func parseIntField(name, raw string, def int) (int, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return def, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("「%s」需为整数（当前输入: %q）", name, raw)
	}
	return v, nil
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
