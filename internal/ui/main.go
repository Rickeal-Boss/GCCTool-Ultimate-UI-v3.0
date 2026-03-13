package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"gcctool/internal/client"
	"gcctool/internal/model"
	"gcctool/internal/robber"
	"gcctool/pkg/logger"
)

// App 应用程序
type App struct {
	app     fyne.App
	window  fyne.Window
	ui      *model.UIComponents
	client  *client.Client
	robber  *robber.Robber
	logger  *logger.Logger
}

// NewApp 创建应用
func NewApp() *App {
	a := &App{
		app: app.New(),
	}

	a.initWindow()
	a.initComponents()
	a.initLogger()
	a.initClient()
	a.initRobber()
	a.buildUI()

	return a
}

// initWindow 初始化窗口
func (a *App) initWindow() {
	a.window = a.app.NewWindow("GCC课程选课助手 V3.0")
	a.window.Resize(fyne.NewSize(900, 700))
	a.window.CenterOnScreen()
	a.window.SetIcon(theme.ComputerIcon())
}

// initComponents 初始化UI组件
func (a *App) initComponents() {
	a.ui = model.NewUIComponents()

	// 初始化节点选择
	a.ui.NodeSelect.Options = []string{
		"节点1（推荐）",
		"节点2（推荐）",
		"节点3（推荐）",
		"节点4（外网）",
		"节点5（外网）",
		"节点6（内网）",
		"节点7（内网）",
	}

	// 初始化课程类型
	a.ui.CourseTypeRadio.Options = []string{"普通网课", "体育课", "普通课"}

	// 初始化分类复选框
	labels := []string{
		"科技类", "人文类", "经营类",
		"体育类", "创新创业类", "艺术类",
		"自然科学类", "思政类", "其他类",
	}
	for i, label := range labels {
		a.ui.CategoryChecks[i] = widget.NewCheck(label, nil)
	}

	// 初始化按钮
	a.initButtons()

	// 设置默认值
	setDefaults(a.ui)
}

// initButtons 初始化按钮
func (a *App) initButtons() {
	a.ui.StartBtn = widget.NewButtonWithIcon("启动", theme.MediaPlayIcon(), func() {
		a.onStartClicked()
	})
	a.ui.StartBtn.Importance = widget.HighImportance

	a.ui.StopBtn = widget.NewButtonWithIcon("停止", theme.MediaStopIcon(), func() {
		a.onStopClicked()
	})
	a.ui.StopBtn.Importance = widget.DangerImportance
	a.ui.StopBtn.Disable()

	a.ui.CopyLogBtn = widget.NewButtonWithIcon("拷贝日志", theme.ContentCopyIcon(), func() {
		if a.logger.Copy() {
			dialog.ShowInformation("提示", "日志已复制到剪贴板", a.window)
		} else {
			dialog.ShowInformation("错误", "日志复制失败", a.window)
		}
	})
}

// initLogger 初始化日志
func (a *App) initLogger() {
	a.logger = logger.NewLogger(a.ui)
}

// initClient 初始化客户端
func (a *App) initClient() {
	a.client = client.NewClient(a.ui.NodeSelect.Selected)
}

// initRobber 初始化抢课调度器
func (a *App) initRobber() {
	a.robber = robber.NewRobber(a.client, a.logger)
}

// buildUI 构建UI
func (a *App) buildUI() {
	content := a.buildMainLayout()
	a.window.SetContent(content)
}

// buildMainLayout 构建主布局
func (a *App) buildMainLayout() *fyne.Container {
	// 标题卡片
	titleCard := a.buildTitleCard()

	// Tab容器
	tabs := container.NewAppTabs(
		container.NewTabItem("基础配置", a.buildConfigTab()),
		container.NewTabItem("高级设置", a.buildAdvancedTab()),
		container.NewTabItem("运行日志", a.buildLogTab()),
	)

	// 底部按钮栏
	buttonBar := a.buildButtonBar()

	return container.NewBorder(titleCard, buttonBar, nil, nil, tabs)
}

// buildTitleCard 构建标题卡片
func (a *App) buildTitleCard() *widget.Card {
	title := widget.NewLabelWithStyle(
		"GCC课程选课助手 V3.0",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	subtitle := widget.NewLabelWithStyle(
		"自动化选课工具 - 仅供学习研究使用",
		fyne.TextAlignCenter,
		fyne.TextStyle{},
	)

	separator := canvas.NewRectangle(theme.DisabledColor())
	separator.SetMinSize(fyne.NewSize(0, 2))

	return widget.NewCard("", "",
		container.NewVBox(title, subtitle, separator),
	)
}

// buildConfigTab 构建配置Tab
func (a *App) buildConfigTab() *fyne.Container {
	return container.NewPadded(
		container.NewVBox(
			a.buildAuthCard(),
			a.buildNodeCard(),
			a.buildTimeCard(),
		),
	)
}

// buildAdvancedTab 构建高级设置Tab
func (a *App) buildAdvancedTab() *fyne.Container {
	return container.NewPadded(
		container.NewVBox(
			a.buildCourseTypeCard(),
			a.buildCategoryCard(),
			a.buildFilterCard(),
		),
	)
}

// buildLogTab 构建日志Tab
func (a *App) buildLogTab() *fyne.Container {
	return container.NewPadded(
		widget.NewCard("运行日志", "", a.ui.LogScroll),
	)
}

// buildAuthCard 构建账号卡片
func (a *App) buildAuthCard() *widget.Card {
	return widget.NewCard("账号信息", "",
		container.NewGridWithColumns(2,
			widget.NewLabel("账号:"), a.ui.UsernameEntry,
			widget.NewLabel("密码:"), a.ui.PasswordEntry,
		),
	)
}

// buildNodeCard 构建节点卡片
func (a *App) buildNodeCard() *widget.Card {
	return widget.NewCard("网络配置", "",
		container.NewVBox(
			widget.NewLabel("选择节点:"), a.ui.NodeSelect,
			widget.NewSeparator(),
			widget.NewLabel("代理地址:"), a.ui.AgentEntry,
		),
	)
}

// buildTimeCard 构建时间卡片
func (a *App) buildTimeCard() *widget.Card {
	return widget.NewCard("时间设置", "",
		container.NewVBox(
			container.NewHBox(
				widget.NewLabel("系统选课时间:"),
				a.ui.HourEntry, widget.NewLabel("时"),
				a.ui.MinuteEntry, widget.NewLabel("分"),
			),
			container.NewHBox(
				widget.NewLabel("提前开抢:"),
				a.ui.AdvanceEntry, widget.NewLabel("分钟"),
			),
			container.NewHBox(
				widget.NewLabel("线程数:"),
				a.ui.ThreadEntry, widget.NewLabel("个"),
			),
		),
	)
}

// buildCourseTypeCard 构建课程类型卡片
func (a *App) buildCourseTypeCard() *widget.Card {
	return widget.NewCard("课程类型", "", a.ui.CourseTypeRadio)
}

// buildCategoryCard 构建分类卡片
func (a *App) buildCategoryCard() *widget.Card {
	checks := make([]fyne.CanvasObject, 9)
	for i, check := range a.ui.CategoryChecks {
		checks[i] = check
	}

	return widget.NewCard("课程分类", "",
		container.NewGridWithColumns(3, checks...),
	)
}

// buildFilterCard 构建筛选卡片
func (a *App) buildFilterCard() *widget.Card {
	return widget.NewCard("筛选条件", "",
		container.NewVBox(
			container.NewHBox(
				widget.NewLabel("学分限制:"),
				a.ui.MinCreditEntry, widget.NewLabel("分"),
			),
			widget.NewSeparator(),
			widget.NewLabel("课程名称:"), a.ui.CourseNameEntry,
			widget.NewSeparator(),
			widget.NewLabel("老师姓名:"), a.ui.TeacherEntry,
			widget.NewSeparator(),
			widget.NewLabel("课程编号:"), a.ui.CourseNumEntry,
		),
	)
}

// buildButtonBar 构建按钮栏
func (a *App) buildButtonBar() *fyne.Container {
	separator := canvas.NewRectangle(theme.DisabledColor())
	separator.SetMinSize(fyne.NewSize(0, 2))

	status := widget.NewLabelWithStyle(
		"就绪",
		fyne.TextAlignCenter,
		fyne.TextStyle{Italic: true},
	)

	buttons := container.NewHBox(
		a.ui.StartBtn,
		a.ui.StopBtn,
		widget.NewSeparator(),
		a.ui.CopyLogBtn,
	)

	return container.NewBorder(
		nil, nil, nil, status,
		container.NewVBox(separator, buttons),
	)
}

// onStartClicked 启动按钮点击
func (a *App) onStartClicked() {
	cfg := a.ui.GetConfig()

	// 验证配置
	if cfg.Username == "" || cfg.Password == "" {
		dialog.ShowError(fmt.Errorf("请输入账号和密码"), a.window)
		return
	}

	// 禁用输入
	disableInputs(a.ui, true)

	// 启用停止按钮
	a.ui.StopBtn.Enable()
	a.ui.StartBtn.Disable()

	// 清空日志
	a.logger.Clear()

	a.logger.Info("开始抢课任务...")

	// 启动抢课
	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.logger.Error(fmt.Sprintf("抢课任务异常: %v", r))
				disableInputs(a.ui, false)
				a.ui.StopBtn.Disable()
				a.ui.StartBtn.Enable()
			}
		}()

		// 重新创建客户端（使用当前节点）
		a.client = client.NewClient(cfg.NodeURL)
		a.robber = robber.NewRobber(a.client, a.logger)

		// 开始抢课
		if err := a.robber.Start(cfg); err != nil {
			a.logger.Error(fmt.Sprintf("启动失败: %v", err))
			disableInputs(a.ui, false)
			a.ui.StopBtn.Disable()
			a.ui.StartBtn.Enable()
		}
	}()
}

// onStopClicked 停止按钮点击
func (a *App) onStopClicked() {
	a.logger.Info("正在停止抢课...")
	a.robber.Stop()

	disableInputs(a.ui, false)
	a.ui.StopBtn.Disable()
	a.ui.StartBtn.Enable()
}

// Run 运行应用
func (a *App) Run() {
	a.window.ShowAndRun()
}

// setDefaults 设置默认值
func setDefaults(ui *model.UIComponents) {
	ui.HourEntry.SetText("12")
	ui.MinuteEntry.SetText("30")
	ui.AdvanceEntry.SetText("1")
	ui.ThreadEntry.SetText("10")
	ui.CourseTypeRadio.SetSelected("普通网课")
	ui.NodeSelect.SetSelectedIndex(0)
	ui.MinCreditEntry.SetText("2")
}

// disableInputs 禁用/启用输入框
func disableInputs(ui *model.UIComponents, disabled bool) {
	entries := []interface {
		Enable()
		Disable()
	}{
		ui.UsernameEntry,
		ui.PasswordEntry,
		ui.NodeSelect,
		ui.AgentEntry,
		ui.HourEntry,
		ui.MinuteEntry,
		ui.AdvanceEntry,
		ui.ThreadEntry,
		ui.CourseTypeRadio,
		ui.CourseNameEntry,
		ui.TeacherEntry,
		ui.CourseNumEntry,
		ui.MinCreditEntry,
	}

	for _, e := range entries {
		if disabled {
			e.Disable()
		} else {
			e.Enable()
		}
	}

	for _, check := range ui.CategoryChecks {
		if disabled {
			check.Disable()
		} else {
			check.Enable()
		}
	}
}
