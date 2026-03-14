package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/client"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/robber"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/pkg/logger"
)

// App 应用程序
type App struct {
	app     fyne.App
	window  fyne.Window
	ui      *model.UIComponents
	client  *client.Client
	robber  *robber.Robber
	logger  *logger.Logger

	// 液态玻璃按钮（替代 ui.StartBtn / ui.StopBtn / ui.CopyLogBtn）
	startLiquid *LiquidButton
	stopLiquid  *LiquidButton
	copyLiquid  *LiquidButton
}

// NewApp 创建应用
func NewApp() *App {
	a := &App{
		app: app.New(),
	}

	// 应用黄色背景 Material 主题
	a.app.Settings().SetTheme(&materialYellowTheme{})

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
	a.window.SetFixedSize(false)
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

// initButtons 初始化液态玻璃风格按钮
func (a *App) initButtons() {
	// 启动按钮：绿色强调
	a.startLiquid = NewLiquidButtonWithAccent(
		"启动",
		theme.MediaPlayIcon(),
		color.NRGBA{R: 0x43, G: 0xA0, B: 0x47, A: 0xFF},
		func() { a.onStartClicked() },
	)

	// 停止按钮：红色强调，初始禁用
	a.stopLiquid = NewLiquidButtonWithAccent(
		"停止",
		theme.MediaStopIcon(),
		color.NRGBA{R: 0xE5, G: 0x39, B: 0x35, A: 0xFF},
		func() { a.onStopClicked() },
	)
	a.stopLiquid.Disable()

	// 拷贝日志按钮：蓝色强调
	a.copyLiquid = NewLiquidButtonWithAccent(
		"拷贝日志",
		theme.ContentCopyIcon(),
		color.NRGBA{R: 0x19, G: 0x76, B: 0xD2, A: 0xFF},
		func() {
			if a.logger.Copy() {
				dialog.ShowInformation("提示", "日志已复制到剪贴板", a.window)
			} else {
				dialog.ShowInformation("错误", "日志复制失败", a.window)
			}
		},
	)

	// 同步到 UIComponents（model 包保留字段，但实际 UI 使用 LiquidButton）
	// widget.Button 仅作占位，不参与实际渲染
	a.ui.StartBtn = widget.NewButton("", nil)
	a.ui.StopBtn = widget.NewButton("", nil)
	a.ui.CopyLogBtn = widget.NewButton("", nil)
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
	// 标题横幅
	titleBanner := a.buildTitleCard()

	// Tab容器
	tabs := container.NewAppTabs(
		container.NewTabItem("基础配置", a.buildConfigTab()),
		container.NewTabItem("高级设置", a.buildAdvancedTab()),
		container.NewTabItem("运行日志", a.buildLogTab()),
	)

	// 底部按钮栏
	buttonBar := a.buildButtonBar()

	return container.NewBorder(titleBanner, buttonBar, nil, nil, tabs)
}

// buildTitleCard 构建标题横幅（Material 风格）
func (a *App) buildTitleCard() fyne.CanvasObject {
	return buildTitleBanner()
}

// buildConfigTab 构建配置Tab（可上下滚动，解决内容溢出布局混乱）
func (a *App) buildConfigTab() *fyne.Container {
	content := container.NewVBox(
		a.buildAuthCard(),
		a.buildNodeCard(),
		a.buildTimeCard(),
	)
	scroll := container.NewVScroll(content)
	return container.NewPadded(scroll)
}

// buildAdvancedTab 构建高级设置Tab（可上下滚动，解决布局冲突）
func (a *App) buildAdvancedTab() *fyne.Container {
	content := container.NewVBox(
		a.buildCourseTypeCard(),
		a.buildCategoryCard(),
		a.buildFilterCard(),
	)
	scroll := container.NewVScroll(content)
	return container.NewPadded(scroll)
}

// buildLogTab 构建日志Tab（Material 风格，日志区填满剩余空间）
//
// 不使用 materialCard 的 VBox 内容包装（VBox 不会拉伸子元素），
// 改为 container.NewBorder：标题行固定在顶，LogScroll 填充所有剩余高度。
func (a *App) buildLogTab() *fyne.Container {
	return container.NewPadded(buildLogPanel("运行日志", 6, a.ui.LogScroll))
}

// buildAuthCard 构建账号卡片（Material 风格）
func (a *App) buildAuthCard() fyne.CanvasObject {
	content := container.NewVBox(
		mdFieldRow("账号", a.ui.UsernameEntry),
		mdSectionDivider(),
		mdFieldRow("密码", a.ui.PasswordEntry),
	)
	return materialCard("账号信息", 0, content)
}

// buildNodeCard 构建节点卡片（Material 风格）
func (a *App) buildNodeCard() fyne.CanvasObject {
	content := container.NewVBox(
		mdFieldRow("选择节点", a.ui.NodeSelect),
		mdSectionDivider(),
		mdFieldRow("代理地址", a.ui.AgentEntry),
	)
	return materialCard("网络配置", 1, content)
}

// buildTimeCard 构建时间卡片（Material 风格）
func (a *App) buildTimeCard() fyne.CanvasObject {
	timeRow := container.NewHBox(
		a.ui.HourEntry, widget.NewLabel("时"),
		a.ui.MinuteEntry, widget.NewLabel("分"),
	)
	advRow := container.NewHBox(a.ui.AdvanceEntry, widget.NewLabel("分钟"))
	threadRow := container.NewHBox(a.ui.ThreadEntry, widget.NewLabel("个"))

	content := container.NewVBox(
		mdFieldRow("系统选课时间", timeRow),
		mdSectionDivider(),
		mdFieldRow("提前开抢", advRow),
		mdSectionDivider(),
		mdFieldRow("线程数", threadRow),
	)
	return materialCard("时间设置", 2, content)
}

// buildCourseTypeCard 构建课程类型卡片（Material 风格）
func (a *App) buildCourseTypeCard() fyne.CanvasObject {
	return materialCard("课程类型", 3, a.ui.CourseTypeRadio)
}

// buildCategoryCard 构建分类卡片（Material 风格）
func (a *App) buildCategoryCard() fyne.CanvasObject {
	checks := make([]fyne.CanvasObject, 9)
	for i, check := range a.ui.CategoryChecks {
		checks[i] = check
	}
	return materialCard("课程分类", 4, container.NewGridWithColumns(3, checks...))
}

// buildFilterCard 构建筛选卡片（Material 风格）
func (a *App) buildFilterCard() fyne.CanvasObject {
	creditRow := container.NewHBox(a.ui.MinCreditEntry, widget.NewLabel("分"))
	content := container.NewVBox(
		mdFieldRow("最低学分", creditRow),
		mdSectionDivider(),
		mdFieldRow("课程名称", a.ui.CourseNameEntry),
		mdSectionDivider(),
		mdFieldRow("老师姓名", a.ui.TeacherEntry),
		mdSectionDivider(),
		mdFieldRow("课程编号", a.ui.CourseNumEntry),
	)
	return materialCard("筛选条件", 5, content)
}

// buildButtonBar 构建底部液态玻璃操作栏
func (a *App) buildButtonBar() fyne.CanvasObject {
	return liquidButtonBar(a.startLiquid, a.stopLiquid, a.copyLiquid, "就绪")
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

	// 启用停止按钮，禁用启动按钮
	a.stopLiquid.Enable()
	a.startLiquid.Disable()

	// 清空日志
	a.logger.Clear()

	a.logger.Info("开始抢课任务...")

	// 启动抢课
	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.logger.Error(fmt.Sprintf("抢课任务异常: %v", r))
				disableInputs(a.ui, false)
				a.stopLiquid.Disable()
				a.startLiquid.Enable()
			}
		}()

		// 重新创建客户端（使用当前节点）
		a.client = client.NewClient(cfg.NodeURL)
		a.robber = robber.NewRobber(a.client, a.logger)

		// 开始抢课
		if err := a.robber.Start(cfg); err != nil {
			a.logger.Error(fmt.Sprintf("启动失败: %v", err))
			disableInputs(a.ui, false)
			a.stopLiquid.Disable()
			a.startLiquid.Enable()
		}
	}()
}

// onStopClicked 停止按钮点击
func (a *App) onStopClicked() {
	a.logger.Info("正在停止抢课...")
	a.robber.Stop()

	disableInputs(a.ui, false)
	a.stopLiquid.Disable()
	a.startLiquid.Enable()
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
