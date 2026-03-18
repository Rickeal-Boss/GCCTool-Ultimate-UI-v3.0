package ui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/client"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/robber"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/stealth"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/pkg/logger"
)

// App 应用程序
type App struct {
	app    fyne.App
	window fyne.Window
	ui     *model.UIComponents
	client *client.Client
	robber *robber.Robber
	logger *logger.Logger

	// 上下文管理（用于优雅关闭所有 goroutine）
	ctx    context.Context
	cancel  context.CancelFunc

	// 液态玻璃操作按钮
	startLiquid *LiquidButton
	stopLiquid  *LiquidButton
	copyLiquid  *LiquidButton

	// 状态芯片文字（底部栏右侧）
	statusLabel *canvas.Text
}

// NewApp 创建并初始化应用
func NewApp() *App {
	a := &App{
		app: app.New(),
	}
	a.app.Settings().SetTheme(&materialYellowTheme{})

	// 初始化上下文管理
	a.ctx, a.cancel = context.WithCancel(context.Background())

	a.initWindow()
	a.initComponents()
	a.initLogger()
	a.initClient()
	a.initRobber()
	a.buildUI()

	// 自动加载上次保存的配置（若存在）
	a.loadSavedConfig()

	// 启动遥测摘要定时刷新（每 30s 输出一次到日志）
	go a.startTelemetryLoop()

	return a
}

// ── 初始化 ────────────────────────────────────────────────────────────────────

func (a *App) initWindow() {
	a.window = a.app.NewWindow("GCC 课程选课助手  V3.1")
	// 960×700：横向更宽，Tab 内字段行有足够空间展开
	a.window.Resize(fyne.NewSize(960, 700))
	a.window.SetFixedSize(false)
	a.window.CenterOnScreen()
	a.window.SetIcon(theme.ComputerIcon())
}

func (a *App) initComponents() {
	a.ui = model.NewUIComponents()

	// ── 节点列表 ──────────────────────────────────────────────────────────
	// 节点1-5：外网 HTTPS（推荐）；节点6-13：校园内网 HTTP（172.22.14.1~8）
	a.ui.NodeSelect.Options = []string{
		"节点1（推荐）",
		"节点2（推荐）",
		"节点3（推荐）",
		"节点4（外网）",
		"节点5（外网）",
		"节点6（内网）",
		"节点7（内网）",
		"节点8（内网）",
		"节点9（内网）",
		"节点10（内网）",
		"节点11（内网）",
		"节点12（内网）",
		"节点13（内网）",
	}

	// ── 课程类型 ──────────────────────────────────────────────────────────
	a.ui.CourseTypeRadio.Options = []string{"普通网课", "体育课", "普通课"}

	// ── 分类复选框 ────────────────────────────────────────────────────────
	labels := []string{
		"科技类", "人文类", "经营类",
		"体育类", "创新创业类", "艺术类",
		"自然科学类", "思政类", "其他类",
	}
	for i, label := range labels {
		a.ui.CategoryChecks[i] = widget.NewCheck(label, nil)
	}

	a.initButtons()
	setDefaults(a.ui)
}

func (a *App) initButtons() {
	// ── 启动按钮：绿色 ────────────────────────────────────────────────────
	a.startLiquid = NewLiquidButtonWithAccent(
		"启动",
		theme.MediaPlayIcon(),
		color.NRGBA{R: 0x43, G: 0xA0, B: 0x47, A: 0xFF},
		func() { a.onStartClicked() },
	)

	// ── 停止按钮：红色，初始禁用 ──────────────────────────────────────────
	a.stopLiquid = NewLiquidButtonWithAccent(
		"停止",
		theme.MediaStopIcon(),
		color.NRGBA{R: 0xE5, G: 0x39, B: 0x35, A: 0xFF},
		func() { a.onStopClicked() },
	)
	a.stopLiquid.Disable()

	// ── 复制日志按钮：蓝色 ────────────────────────────────────────────────
	a.copyLiquid = NewLiquidButtonWithAccent(
		"复制日志",
		theme.ContentCopyIcon(),
		color.NRGBA{R: 0x19, G: 0x76, B: 0xD2, A: 0xFF},
		func() {
			// 日志含学号/行为信息，复制前提示
			dialog.ShowConfirm(
				"安全提示",
				"日志中包含您的学号及选课行为信息。\n\n"+
					"系统剪贴板可被其他程序读取，若开启了云剪贴板同步，\n"+
					"内容还会上传至云端服务器。\n\n"+
					"确认复制？复制后请及时清空剪贴板。",
				func(ok bool) {
					if !ok {
						return
					}
					if a.logger.Copy() {
						dialog.ShowInformation("已复制", "日志已复制到剪贴板，请用完后及时清空。", a.window)
					} else {
						dialog.ShowInformation("复制失败", "日志为空或复制失败，请重试。", a.window)
					}
				},
				a.window,
			)
		},
	)

	// widget.Button 占位（保持 UIComponents 字段兼容，不参与实际渲染）
	a.ui.StartBtn = widget.NewButton("", nil)
	a.ui.StopBtn = widget.NewButton("", nil)
	a.ui.CopyLogBtn = widget.NewButton("", nil)
}

func (a *App) initLogger() {
	a.logger = logger.NewLogger(a.ui)
}

func (a *App) initClient() {
	a.client = client.NewClientWithProxy(a.ui.NodeSelect.Selected, a.ui.AgentEntry.Text)
}

func (a *App) initRobber() {
	a.robber = robber.NewRobber(a.client, a.logger)
}

// ── UI 构建 ───────────────────────────────────────────────────────────────────

func (a *App) buildUI() {
	a.window.SetContent(a.buildMainLayout())
}

func (a *App) buildMainLayout() *fyne.Container {
	titleBanner := a.buildTitleCard()

	tabs := container.NewAppTabs(
		container.NewTabItem("基础配置", a.buildConfigTab()),
		container.NewTabItem("高级设置", a.buildAdvancedTab()),
		container.NewTabItem("课程列表", a.buildCourseListTab()),
		container.NewTabItem("运行日志", a.buildLogTab()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	buttonBar := a.buildButtonBar()

	return container.NewBorder(titleBanner, buttonBar, nil, nil, tabs)
}

func (a *App) buildTitleCard() fyne.CanvasObject {
	return buildTitleBanner()
}

// buildConfigTab 基础配置 Tab（可滚动）
func (a *App) buildConfigTab() *fyne.Container {
	content := container.NewVBox(
		a.buildAuthCard(),
		a.buildNodeCard(),
		a.buildTimeCard(),
	)
	return container.NewPadded(container.NewVScroll(content))
}

// buildAdvancedTab 高级设置 Tab（可滚动）
func (a *App) buildAdvancedTab() *fyne.Container {
	content := container.NewVBox(
		a.buildCourseTypeCard(),
		a.buildCategoryCard(),
		a.buildFilterCard(),
	)
	return container.NewPadded(container.NewVScroll(content))
}

// buildLogTab 日志 Tab
func (a *App) buildLogTab() *fyne.Container {
	return container.NewPadded(buildLogPanel("运行日志", 6, a.ui.LogScroll))
}

// buildCourseListTab 课程列表 Tab（用于显示获取到的课程，支持应用内直接选课）
func (a *App) buildCourseListTab() *fyne.Container {
	// 课程列表标题
	title := widget.NewLabel("已获取的课程列表（点击课程可直接选课）")
	title.TextStyle = fyne.TextStyle{Bold: true}

	// 刷新按钮
	refreshBtn := widget.NewButton("刷新课程列表", func() {
		a.refreshCourseList()
	})

	// 导出按钮
	exportBtn := widget.NewButton("导出课程信息", func() {
		a.exportCourseInfo()
	})

	// 按钮行
	buttonRow := container.NewHBox(refreshBtn, exportBtn)

	// 选中的课程索引
	var selectedIndex int = -1

	// 选课按钮（初始禁用）
	selectBtn := widget.NewButton("选 择 此 课 程", func() {
		if selectedIndex >= 0 && selectedIndex < len(a.ui.CourseData) {
			a.manualSelectCourse(a.ui.CourseData[selectedIndex])
		}
	})
	selectBtn.Importance = widget.HighImportance
	selectBtn.Disable()

	// 课程列表（使用 List 组件）
	a.ui.CourseList = widget.NewList(
		func() int {
			return len(a.ui.CourseData)
		},
		func() fyne.CanvasObject {
			// 列表项模板
			return container.NewVBox(
				widget.NewLabel("课程名称"),
				container.NewHBox(
					widget.NewLabel("教师: "),
					widget.NewLabel("时间: "),
					widget.NewLabel("容量: "),
				),
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i >= len(a.ui.CourseData) {
				return
			}
			course := a.ui.CourseData[i]
			container := o.(*fyne.Container)

			// 更新课程名称
			titleLabel := container.Objects[0].(*widget.Label)
			titleLabel.SetText(fmt.Sprintf("%s (%s学分)", course.Name, formatCredit(course.Credit)))

			// 更新详细信息
			detailBox := container.Objects[1].(*fyne.Container)
			teacherLabel := detailBox.Objects[0].(*widget.Label)
			timeLabel := detailBox.Objects[1].(*widget.Label)
			capacityLabel := detailBox.Objects[2].(*widget.Label)

			teacherLabel.SetText(fmt.Sprintf("教师: %s", course.Teacher))
			timeLabel.SetText(fmt.Sprintf("时间: %s", course.WeekTime))
			capacityLabel.SetText(fmt.Sprintf("容量: %d/%d", course.Selected, course.Capacity))
		},
	)

	// 列表选择事件
	a.ui.CourseList.OnSelected = func(id widget.ListItemID) {
		selectedIndex = id
		if id >= 0 && id < len(a.ui.CourseData) {
			course := a.ui.CourseData[id]
			if course.IsFull() {
				selectBtn.SetText("课程已满")
				selectBtn.Disable()
			} else {
				selectBtn.SetText(fmt.Sprintf("选择: %s", course.Name))
				selectBtn.Enable()
			}
		}
	}

	a.ui.CourseList.OnUnselected = func(id widget.ListItemID) {
		selectedIndex = -1
		selectBtn.SetText("选 择 此 课 程")
		selectBtn.Disable()
	}

	// 说明文字
	hint := canvas.NewText("提示：点击列表中的课程，然后点击"选择此课程"按钮即可在应用内直接选课", mdForegroundSub)
	hint.TextSize = 11

	// 布局
	content := container.NewBorder(
		container.NewVBox(title, buttonRow, hint),
		container.NewPadded(selectBtn),
		nil, nil,
		a.ui.CourseList,
	)

	return container.NewPadded(content)
}

// ── 卡片构建 ──────────────────────────────────────────────────────────────────

func (a *App) buildAuthCard() fyne.CanvasObject {
	content := container.NewVBox(
		mdFieldRow("账号", a.ui.UsernameEntry),
		mdSectionDivider(),
		mdFieldRow("密码", a.ui.PasswordEntry),
	)
	return materialCard("账号信息", 0, content)
}

func (a *App) buildNodeCard() fyne.CanvasObject {
	// 节点选择 + 节点说明提示
	nodeHint := canvas.NewText("节点1-5 HTTPS 校外可用；节点6-13 HTTP 仅校内可用", mdForegroundSub)
	nodeHint.TextSize = 11

	content := container.NewVBox(
		mdFieldRow("服务节点", a.ui.NodeSelect),
		container.NewPadded(nodeHint),
		mdSectionDivider(),
		mdFieldRow("HTTP 代理", a.ui.AgentEntry),
	)
	return materialCard("网络配置", 1, content)
}

func (a *App) buildTimeCard() fyne.CanvasObject {
	timeRow := container.NewHBox(
		a.ui.HourEntry, widget.NewLabel("时"),
		a.ui.MinuteEntry, widget.NewLabel("分"),
	)
	advRow := container.NewHBox(a.ui.AdvanceEntry, widget.NewLabel("分钟"))
	threadRow := container.NewHBox(a.ui.ThreadEntry, widget.NewLabel("个"))

	// 线程数提示
	threadHint := canvas.NewText("建议 5~15，过高会触发服务端限流", mdForegroundSub)
	threadHint.TextSize = 11

	content := container.NewVBox(
		mdFieldRow("选课时间", timeRow),
		mdSectionDivider(),
		mdFieldRow("提前开抢", advRow),
		mdSectionDivider(),
		mdFieldRow("并发线程", threadRow),
		container.NewPadded(threadHint),
	)
	return materialCard("时间与线程", 2, content)
}

func (a *App) buildCourseTypeCard() fyne.CanvasObject {
	return materialCard("课程类型", 3, a.ui.CourseTypeRadio)
}

func (a *App) buildCategoryCard() fyne.CanvasObject {
	checks := make([]fyne.CanvasObject, 9)
	for i, check := range a.ui.CategoryChecks {
		checks[i] = check
	}
	return materialCard("课程分类（多选）", 4, container.NewGridWithColumns(3, checks...))
}

func (a *App) buildFilterCard() fyne.CanvasObject {
	creditRow := container.NewHBox(a.ui.MinCreditEntry, widget.NewLabel("分"))

	content := container.NewVBox(
		mdFieldRow("最低学分", creditRow),
		mdSectionDivider(),
		mdFieldRow("课程名称", a.ui.CourseNameEntry),
		mdSectionDivider(),
		mdFieldRow("教师姓名", a.ui.TeacherEntry),
		mdSectionDivider(),
		mdFieldRow("课程编号", a.ui.CourseNumEntry),
	)
	return materialCard("筛选条件", 5, content)
}

// buildButtonBar 底部液态玻璃操作栏
// statusLabel 由 a.statusLabel 持有，可在运行时动态更新
func (a *App) buildButtonBar() fyne.CanvasObject {
	// 就绪状态：浅蓝白色（柔和、清晰）
	a.statusLabel = canvas.NewText("● 就绪", color.NRGBA{R: 0xC8, G: 0xD8, B: 0xFF, A: 0xFF})
	a.statusLabel.TextSize = 13

	return buildDynamicButtonBar(a.startLiquid, a.stopLiquid, a.copyLiquid, a.statusLabel)
}

// ── 事件处理 ──────────────────────────────────────────────────────────────────

func (a *App) onStartClicked() {
	cfg := a.ui.GetConfig()

	if cfg.Username == "" || cfg.Password == "" {
		dialog.ShowError(fmt.Errorf("请输入账号和密码"), a.window)
		return
	}

	// HTTP 内网节点安全警告
	// ⚠️ 注意：cfg.NodeURL 存的是节点显示名（如"节点6（内网）"），不是真实 URL，
	// 必须先通过 client.NodeURLFromName 翻译成真实 Base URL 再判断 http:// 前缀，
	// 否则判断永远不成立（节点名不以 "http://" 开头）。
	realURL := client.NodeURLFromName(cfg.NodeURL)
	if len(realURL) >= 7 && realURL[:7] == "http://" {
		dialog.ShowConfirm(
			"⚠️ 不安全的网络连接",
			"当前选择的节点使用 HTTP（明文）传输。\n\n"+
				"在校园网/内网环境下，同一网段的设备可以通过\n"+
				"ARP 欺骗截获您的 Session Cookie，\n"+
				"等同于账号被盗。\n\n"+
				"建议切换到 HTTPS 节点（节点1-5）。\n"+
				"内网节点（节点6-13）仅在校园网内可用。\n"+
				"确定仍要使用当前内网节点继续？",
			func(ok bool) {
				if !ok {
					return
				}
				a.doStartRobbery(cfg)
			},
			a.window,
		)
		return
	}

	a.doStartRobbery(cfg)
}

func (a *App) doStartRobbery(cfg *model.Config) {
	// 自动保存配置（启动时保存，程序崩溃也不会丢失配置）
	if err := model.SaveConfig(cfg); err != nil {
		a.logger.Warn(fmt.Sprintf("配置保存失败（不影响运行）: %v", err))
	} else {
		a.logger.Info("配置已自动保存")
	}

	// 重置遥测计数器
	stealth.Global.Reset()

	// 禁用所有输入，切换按钮状态
	disableInputs(a.ui, true)
	a.stopLiquid.Enable()
	a.startLiquid.Disable()

	// 更新状态芯片：登录中 → 温暖 Amber 闪烁感
	a.setStatus("● 登录中...", color.NRGBA{R: 0xFF, G: 0xD0, B: 0x40, A: 0xFF})

	a.logger.Clear()
	a.logger.Info("开始抢课任务...")

	go func() {
		defer func() {
			// 密码原地清零（防止堆栈/内存残留）
			if len(cfg.Password) > 0 {
				b := []byte(cfg.Password)
				for i := range b {
					b[i] = 0
				}
				cfg.Password = ""
			}

			if r := recover(); r != nil {
				a.logger.Error(fmt.Sprintf("抢课任务异常: %v", r))
				a.resetUIAfterStop()
			}
		}()

		// 重建客户端（使用当前节点 + 代理）
		a.client = client.NewClientWithProxy(cfg.NodeURL, cfg.Agent)
		a.robber = robber.NewRobber(a.client, a.logger)

		if err := a.robber.Start(cfg); err != nil {
			a.logger.Error(fmt.Sprintf("启动失败: %v", err))
			a.resetUIAfterStop()
			return
		}

		// Start() 登录成功后立即返回，更新状态为"抢课中"（亮绿）
		a.setStatus("● 抢课中", color.NRGBA{R: 0x4A, G: 0xDE, B: 0x80, A: 0xFF})
	}()
}

func (a *App) onStopClicked() {
	a.logger.Info("正在停止抢课...")
	// Anti-Fix-Bug: 添加 nil 检查，防止崩溃
	if a.robber != nil {
		a.robber.Stop()
	}
	a.resetUIAfterStop()
}

// resetUIAfterStop 停止后恢复 UI 状态
func (a *App) resetUIAfterStop() {
	// Anti-Fix-Bug: 添加 nil 检查
	if a.ui != nil {
		disableInputs(a.ui, false)
	}
	if a.stopLiquid != nil {
		a.stopLiquid.Disable()
	}
	if a.startLiquid != nil {
		a.startLiquid.Enable()
	}
	// 已停止：亮红色，清晰辨识
	a.setStatus("● 已停止", color.NRGBA{R: 0xFF, G: 0x6B, B: 0x6B, A: 0xFF})
}

// setStatus 更新底部状态芯片文字和颜色
func (a *App) setStatus(text string, col color.NRGBA) {
	if a.statusLabel == nil {
		return
	}
	a.statusLabel.Text = text
	a.statusLabel.Color = col
	a.statusLabel.Refresh()
}

// refreshCourseList 刷新课程列表显示
func (a *App) refreshCourseList() {
	if a.robber == nil {
		dialog.ShowInformation("提示", "请先启动抢课任务以获取课程列表", a.window)
		return
	}

	courses := a.robber.GetLastMatchedCourses()
	if courses == nil || len(courses) == 0 {
		// 尝试获取全部课程列表
		list := a.robber.GetLastCourseList()
		if list != nil && len(list.Items) > 0 {
			a.ui.CourseData = list.Items
			a.ui.CourseList.Refresh()
			dialog.ShowInformation("提示", fmt.Sprintf("已加载 %d 门课程", len(list.Items)), a.window)
		} else {
			dialog.ShowInformation("提示", "暂无课程数据，请等待抢课任务获取课程列表", a.window)
		}
		return
	}

	a.ui.CourseData = courses
	a.ui.CourseList.Refresh()
	dialog.ShowInformation("提示", fmt.Sprintf("已加载 %d 门匹配的课程", len(courses)), a.window)
}

// exportCourseInfo 导出课程信息到剪贴板
func (a *App) exportCourseInfo() {
	if len(a.ui.CourseData) == 0 {
		dialog.ShowInformation("提示", "暂无课程数据可导出", a.window)
		return
	}

	var sb strings.Builder
	sb.WriteString("=== GCC课程选课助手 - 课程信息导出 ===\n")
	sb.WriteString(fmt.Sprintf("导出时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("课程数量: %d\n\n", len(a.ui.CourseData)))

	for i, course := range a.ui.CourseData {
		sb.WriteString(fmt.Sprintf("【%d】%s\n", i+1, course.Name))
		sb.WriteString(fmt.Sprintf("  课程编号: %s\n", course.Number))
		sb.WriteString(fmt.Sprintf("  教师: %s\n", course.Teacher))
		sb.WriteString(fmt.Sprintf("  学分: %d\n", course.Credit))
		sb.WriteString(fmt.Sprintf("  上课时间: %s\n", course.WeekTime))
		sb.WriteString(fmt.Sprintf("  上课地点: %s\n", course.Room))
		sb.WriteString(fmt.Sprintf("  容量: %d/%d\n", course.Selected, course.Capacity))
		if course.Extra != nil && course.Extra.DoJxbID != "" {
			sb.WriteString(fmt.Sprintf("  教学班ID: %s\n", course.Extra.DoJxbID))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("=== 手动选课指引 ===\n")
	sb.WriteString("1. 访问: https://jwxt.gcc.edu.cn/xsxk/zzxkyzb_cxZzxkYzbIndex.html?gnmkdm=N253512&layout=default\n")
	sb.WriteString("2. 登录教务系统\n")
	sb.WriteString("3. 根据上述课程信息搜索并选择课程\n")
	sb.WriteString("4. 点击选课按钮完成选课\n")

	// 复制到剪贴板
	a.window.Clipboard().SetContent(sb.String())
	dialog.ShowInformation("导出成功", "课程信息已复制到剪贴板，可直接粘贴到记事本保存", a.window)
}

// formatCredit 格式化学分显示
func formatCredit(credit int) string {
	if credit == 0 {
		return "未知"
	}
	return fmt.Sprintf("%d", credit)
}

// manualSelectCourse 在应用内手动选择课程
func (a *App) manualSelectCourse(course *model.Course) {
	if a.robber == nil {
		dialog.ShowError(fmt.Errorf("抢课器未初始化，请先启动任务"), a.window)
		return
	}

	// 检查课程是否已满
	if course.IsFull() {
		dialog.ShowInformation("提示", "该课程已满，请选择其他课程", a.window)
		return
	}

	// 显示确认对话框
	confirmMsg := fmt.Sprintf(
		"确定要选择以下课程吗？\n\n"+
			"课程: %s\n"+
			"教师: %s\n"+
			"学分: %d\n"+
			"时间: %s\n"+
			"容量: %d/%d",
		course.Name, course.Teacher, course.Credit,
		course.WeekTime, course.Selected, course.Capacity,
	)

	dialog.ShowConfirm("确认选课", confirmMsg, func(ok bool) {
		if !ok {
			return
		}

		// 在后台执行选课
		go func() {
			a.logger.Info(fmt.Sprintf("正在手动选课: %s (%s)...", course.Name, course.Teacher))

			err := a.robber.ManualSelectCourse(course)
			if err != nil {
				a.logger.Error(fmt.Sprintf("手动选课失败: %v", err))
				dialog.ShowError(fmt.Errorf("选课失败: %v", err), a.window)
				return
			}

			a.logger.Success(fmt.Sprintf("✅ 手动选课成功: %s (%s)", course.Name, course.Teacher))
			dialog.ShowInformation("成功", fmt.Sprintf("选课成功！\n\n课程: %s\n教师: %s", course.Name, course.Teacher), a.window)
		}()
	}, a.window)
}

// Run 启动应用主循环
func (a *App) Run() {
	// 应用关闭时取消所有 goroutine
	defer a.cancel()
	a.window.ShowAndRun()
}

// loadSavedConfig 从本地文件加载上次保存的配置并填充 UI
func (a *App) loadSavedConfig() {
	if !model.ConfigExists() {
		return
	}
	cfg, err := model.LoadConfig()
	if err != nil {
		a.logger.Warn(fmt.Sprintf("加载历史配置失败: %v", err))
		return
	}
	// 填充 UI 字段
	if cfg.Username != "" {
		a.ui.UsernameEntry.SetText(cfg.Username)
	}
	if cfg.Password != "" {
		a.ui.PasswordEntry.SetText(cfg.Password)
	}
	if cfg.NodeURL != "" {
		a.ui.NodeSelect.SetSelected(cfg.NodeURL)
	}
	if cfg.Agent != "" {
		a.ui.AgentEntry.SetText(cfg.Agent)
	}
	a.ui.HourEntry.SetText(fmt.Sprintf("%d", cfg.Hour))
	a.ui.MinuteEntry.SetText(fmt.Sprintf("%d", cfg.Minute))
	a.ui.AdvanceEntry.SetText(fmt.Sprintf("%d", cfg.Advance))
	a.ui.ThreadEntry.SetText(fmt.Sprintf("%d", cfg.Threads))
	a.ui.MinCreditEntry.SetText(fmt.Sprintf("%d", cfg.MinCredit))
	if cfg.CourseName != "" {
		a.ui.CourseNameEntry.SetText(cfg.CourseName)
	}
	if cfg.TeacherName != "" {
		a.ui.TeacherEntry.SetText(cfg.TeacherName)
	}
	if cfg.CourseNumber != "" {
		a.ui.CourseNumEntry.SetText(cfg.CourseNumber)
	}
	a.logger.Info("✓ 已加载上次保存的配置")
}

// startTelemetryLoop 定期将遥测摘要和策略建议输出到日志（每 30s 一次）
//
// 仅在抢课运行期间输出（检测 robber.IsRunning()），避免空闲时刷屏。
func (a *App) startTelemetryLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if a.robber == nil || !a.robber.IsRunning() {
				continue
			}
			// 输出遥测摘要
			a.logger.Info(stealth.Global.Summary())
			// 输出策略建议
			advices := stealth.Global.Analyze()
			if len(advices) > 0 {
				a.logger.Warn(stealth.FormatAdvices(advices))
			}
		case <-a.ctx.Done():
			// 应用关闭，退出循环
			return
		}
	}
}

// ── 辅助函数 ──────────────────────────────────────────────────────────────────

func setDefaults(ui *model.UIComponents) {
	ui.HourEntry.SetText("12")
	ui.MinuteEntry.SetText("30")
	ui.AdvanceEntry.SetText("1")
	ui.ThreadEntry.SetText("10")
	ui.CourseTypeRadio.SetSelected("普通网课")
	ui.NodeSelect.SetSelectedIndex(0)
	ui.MinCreditEntry.SetText("2")
}

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

// buildDynamicButtonBar 动态底部操作栏（statusLabel 外部持有，可运行时更新）
//
// 视觉层次（底→顶）：
//  1. barBg       — 深色半透明背景（Ink 深蓝）
//  2. topGlow     — 顶部 Amber 高光线（2px，液态玻璃上边缘反光）
//  3. topGlow2    — 顶部白色次级高光（1px，更柔和）
//  4. foreground  — 按钮行 + 状态芯片
func buildDynamicButtonBar(startBtn, stopBtn, copyBtn fyne.CanvasObject, statusLbl *canvas.Text) fyne.CanvasObject {
	// 背景：深色半透明，与暖黄主界面形成强烈对比
	barBg := canvas.NewRectangle(color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0xEE})

	// Amber 顶部高光线（主色调呼应）
	topGlow := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x99})
	topGlow.SetMinSize(fyne.NewSize(0, 2))

	// 白色次级高光线（更柔和的玻璃质感）
	topGlow2 := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x1A})
	topGlow2.SetMinSize(fyne.NewSize(0, 1))

	// 状态芯片：精致玻璃胶囊设计
	// 外层：圆角矩形 + 低透明度填充 + 半透明描边
	chipBg := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x16})
	chipBg.CornerRadius = 18
	chipBg.StrokeColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x50}
	chipBg.StrokeWidth = 1

	// 内部高光层（模拟玻璃折射）
	chipShim := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x1A})
	chipShim.CornerRadius = 18

	// 文字字号稍大，更易读
	statusLbl.TextSize = 13

	statusChip := container.NewPadded(
		container.NewStack(chipBg, chipShim, container.NewPadded(statusLbl)),
	)

	// 按钮间分隔线
	sep := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x28})
	sep.SetMinSize(fyne.NewSize(1, 28))

	buttons := container.NewHBox(startBtn, stopBtn, sep, copyBtn)
	row := container.NewBorder(nil, nil, nil, statusChip, buttons)

	foreground := container.NewVBox(
		topGlow,
		topGlow2,
		container.NewPadded(row),
	)

	return container.NewStack(barBg, foreground)
}
