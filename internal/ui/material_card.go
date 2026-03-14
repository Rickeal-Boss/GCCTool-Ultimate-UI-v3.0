package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// mdAccentColors Material 卡片左侧强调色（每种卡片独立颜色）
var mdAccentColors = []color.NRGBA{
	{R: 0xFF, G: 0xB3, B: 0x00, A: 0xFF}, // Amber  — 账号信息
	{R: 0x19, G: 0x76, B: 0xD2, A: 0xFF}, // Blue   — 网络配置
	{R: 0x43, G: 0xA0, B: 0x47, A: 0xFF}, // Green  — 时间设置
	{R: 0xE5, G: 0x39, B: 0x35, A: 0xFF}, // Red    — 课程类型
	{R: 0x7B, G: 0x1F, B: 0xA2, A: 0xFF}, // Purple — 课程分类
	{R: 0x00, G: 0x89, B: 0x7B, A: 0xFF}, // Teal   — 筛选条件
	{R: 0xF5, G: 0x7C, B: 0x00, A: 0xFF}, // Orange — 运行日志
}

// materialCard 构建 Material Design 风格卡片
//
//	title     — 卡片标题
//	accentIdx — 左侧强调色索引（使用 mdAccentColors）
//	content   — 卡片内容
//
// 性能优化：使用 container.NewStack（Fyne 内置，无自定义 layout 递归）
// 替代原先的 shadowLayout + cardStackLayout 双层自定义布局，
// 大幅降低窗口 resize 时的重布局开销，消除卡顿。
func materialCard(title string, accentIdx int, content fyne.CanvasObject) fyne.CanvasObject {
	accent := mdAccentColors[accentIdx%len(mdAccentColors)]

	// ── 卡片白色背景（带圆角）────────────────────────────────────
	bg := canvas.NewRectangle(color.White)
	bg.CornerRadius = 8
	bg.StrokeColor = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	bg.StrokeWidth = 1

	// ── 左侧彩色强调条 ────────────────────────────────────────────
	accentBar := canvas.NewRectangle(accent)
	accentBar.CornerRadius = 4
	accentBar.SetMinSize(fyne.NewSize(4, 0))

	// ── 标题行 ────────────────────────────────────────────────────
	var header fyne.CanvasObject
	if title != "" {
		titleLabel := widget.NewLabelWithStyle(
			title,
			fyne.TextAlignLeading,
			fyne.TextStyle{Bold: true},
		)
		// 标题下方分割线
		divider := canvas.NewRectangle(color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF})
		divider.SetMinSize(fyne.NewSize(0, 1))

		// 标题左侧小色块装饰
		dot := canvas.NewRectangle(accent)
		dot.CornerRadius = 2
		dot.SetMinSize(fyne.NewSize(12, 12))

		titleRow := container.NewHBox(dot, titleLabel)
		header = container.NewVBox(titleRow, divider)
	}

	// ── 内容区（带内边距）────────────────────────────────────────
	var innerContent fyne.CanvasObject
	if header != nil {
		innerContent = container.NewVBox(header, container.NewPadded(content))
	} else {
		innerContent = container.NewPadded(content)
	}

	// ── 强调条 + 内容横向排列 ─────────────────────────────────────
	cardBody := container.NewBorder(nil, nil, accentBar, nil, innerContent)

	// ── 用 NewStack 叠放：bg 在底，cardBody 在上 ──────────────────
	// NewStack 是 Fyne 内置布局，比自定义 layout 性能更好，resize 无卡顿
	return container.NewPadded(
		container.NewStack(bg, cardBody),
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// buildLogPanel 日志面板：与 materialCard 同风格，但内容区填满所有剩余空间
//
// materialCard 内部用 VBox 包内容，VBox 不会拉伸子元素到剩余高度，
// 导致 LogScroll 只显示最小高度。这里改用 container.NewBorder：
//   - 顶部：标题行（固定高度）
//   - 中间：content（填充剩余全部高度）
// ─────────────────────────────────────────────────────────────────────────────
func buildLogPanel(title string, accentIdx int, content fyne.CanvasObject) fyne.CanvasObject {
	accent := mdAccentColors[accentIdx%len(mdAccentColors)]

	// ── 卡片背景 ──────────────────────────────────────────────────
	bg := canvas.NewRectangle(color.White)
	bg.CornerRadius = 8
	bg.StrokeColor = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	bg.StrokeWidth = 1

	// ── 左侧彩色强调条 ────────────────────────────────────────────
	accentBar := canvas.NewRectangle(accent)
	accentBar.CornerRadius = 4
	accentBar.SetMinSize(fyne.NewSize(4, 0))

	// ── 标题行 ────────────────────────────────────────────────────
	dot := canvas.NewRectangle(accent)
	dot.CornerRadius = 2
	dot.SetMinSize(fyne.NewSize(12, 12))

	titleLabel := widget.NewLabelWithStyle(
		title,
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	titleRow := container.NewHBox(dot, titleLabel)

	divider := canvas.NewRectangle(color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF})
	divider.SetMinSize(fyne.NewSize(0, 1))

	header := container.NewVBox(titleRow, divider)

	// ── 内容区：用 Border 让 content 填满剩余高度 ─────────────────
	// top=header 固定，center=content 自动拉伸填充
	inner := container.NewBorder(header, nil, nil, nil, container.NewPadded(content))

	// ── 强调条 + 内容区横向排列 ───────────────────────────────────
	cardBody := container.NewBorder(nil, nil, accentBar, nil, inner)

	// ── NewStack：bg 铺满底层，cardBody 叠上去 ────────────────────
	return container.NewStack(bg, cardBody)
}

// ─────────────────────────────────────────────────────────────────────────────
// buildTitleBanner 构建顶部 Material 标题横幅（Amber 底色）
// ─────────────────────────────────────────────────────────────────────────────
func buildTitleBanner() fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0xFF})
	bg.CornerRadius = 0

	// 半透明圆形装饰
	deco := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x33})
	deco.CornerRadius = 50
	deco.SetMinSize(fyne.NewSize(48, 48))

	title := widget.NewLabelWithStyle(
		"GCC 课程选课助手  V3.0",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	title.Importance = widget.HighImportance

	subtitle := widget.NewLabelWithStyle(
		"自动化选课工具  ·  仅供学习研究使用",
		fyne.TextAlignCenter,
		fyne.TextStyle{Italic: true},
	)

	divider := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x4D})
	divider.SetMinSize(fyne.NewSize(0, 1))

	textCol := container.NewVBox(
		widget.NewLabel(""), // 上边距
		title,
		subtitle,
		divider,
		widget.NewLabel(""), // 下边距
	)

	foreground := container.NewBorder(nil, nil,
		container.NewPadded(deco), nil,
		container.NewPadded(textCol),
	)

	// NewStack：bg 铺满底层，foreground 叠加，无需自定义 layout
	return container.NewStack(bg, foreground)
}

// bannerLayout 已废弃，由 container.NewStack 替代，保留空结构防止旧引用报错
type bannerLayout struct{ bg *canvas.Rectangle }

func (b *bannerLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	b.bg.Resize(size)
	b.bg.Move(fyne.NewPos(0, 0))
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

func (b *bannerLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	min := fyne.NewSize(0, 80)
	for _, o := range objects {
		s := o.MinSize()
		if s.Width > min.Width {
			min.Width = s.Width
		}
		if s.Height > min.Height {
			min.Height = s.Height
		}
	}
	return min
}

// mdIconLabel 带图标的字段标签（Material style）
func mdIconLabel(icon fyne.Resource, text string) fyne.CanvasObject {
	dot := canvas.NewRectangle(mdPrimary)
	dot.CornerRadius = 3
	dot.SetMinSize(fyne.NewSize(6, 6))

	var lbl fyne.CanvasObject
	if icon != nil {
		lbl = container.NewHBox(widget.NewIcon(icon), widget.NewLabel(text))
	} else {
		lbl = container.NewHBox(dot, widget.NewLabel(text))
	}
	return lbl
}

// mdFieldRow 标签 + 输入控件一行排列
func mdFieldRow(label string, input fyne.CanvasObject) fyne.CanvasObject {
	lbl := mdIconLabel(nil, label)
	return container.NewBorder(nil, nil, lbl, nil, input)
}

// mdSectionDivider 轻量分隔线
func mdSectionDivider() fyne.CanvasObject {
	line := canvas.NewRectangle(color.NRGBA{R: 0xEE, G: 0xEE, B: 0xEE, A: 0xFF})
	line.SetMinSize(fyne.NewSize(0, 1))
	return container.NewPadded(line)
}

// mdChip 标签芯片（用于强调小标题）
// 使用 NewStack 替代 chipLayout，padding 由 container.NewPadded 处理
func mdChip(text string, accent color.NRGBA) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: accent.R, G: accent.G, B: accent.B, A: 0x1A})
	bg.CornerRadius = 12
	lbl := widget.NewLabel(text)
	// NewStack 内容即为 lbl 大小，bg 铺满；外层 padding 留出芯片间距
	return container.NewPadded(container.NewStack(bg, lbl))
}

type chipLayout struct{ bg *canvas.Rectangle }

func (c *chipLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	c.bg.Resize(size)
	c.bg.Move(fyne.NewPos(0, 0))
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

func (c *chipLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	min := fyne.NewSize(0, 0)
	for _, o := range objects {
		s := o.MinSize()
		if s.Width+16 > min.Width {
			min.Width = s.Width + 16
		}
		if s.Height+8 > min.Height {
			min.Height = s.Height + 8
		}
	}
	return min
}

// mdInputWithIcon 带前缀图标的输入框包装
// widget.Icon 不支持 SetMinSize，用固定尺寸容器包裹
func mdInputWithIcon(icon fyne.Resource, entry fyne.CanvasObject) fyne.CanvasObject {
	if icon == nil {
		return entry
	}
	ic := widget.NewIcon(icon)
	// 用固定最小尺寸的矩形作为占位，让图标容器有固定宽度
	iconBox := container.New(&fixedSizeLayout{w: 20, h: 20}, ic)
	return container.NewBorder(nil, nil, iconBox, nil, entry)
}

// ─── 底部按钮栏 Material FAB 风格 ────────────────────────────────────────────

// mdButtonBar 构建带 Material 样式的底部操作栏
// 使用 NewStack 替代 barLayout，减少 resize 计算压力
func mdButtonBar(startBtn, stopBtn, copyBtn fyne.CanvasObject, statusText string) fyne.CanvasObject {
	barBg := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})

	topLine := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0xFF})
	topLine.SetMinSize(fyne.NewSize(0, 3))

	// 芯片状态指示
	statusChip := mdChip("● "+statusText, color.NRGBA{R: 0x43, G: 0xA0, B: 0x47, A: 0xFF})

	buttons := container.NewHBox(startBtn, stopBtn,
		canvas.NewRectangle(color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}),
		copyBtn,
	)

	row := container.NewBorder(nil, nil, nil, statusChip, buttons)
	foreground := container.NewVBox(topLine, container.NewPadded(row))

	// NewStack：barBg 铺满底层，foreground 内容叠上去
	return container.NewStack(barBg, foreground)
}

type barLayout struct{ bg *canvas.Rectangle }

func (b *barLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	b.bg.Resize(size)
	b.bg.Move(fyne.NewPos(0, 0))
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

func (b *barLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	min := fyne.NewSize(0, 0)
	for _, o := range objects {
		s := o.MinSize()
		if s.Width > min.Width {
			min.Width = s.Width
		}
		if s.Height > min.Height {
			min.Height = s.Height
		}
	}
	return min
}

// fixedSizeLayout 固定最小尺寸布局，用于给不支持 SetMinSize 的 widget 设定大小
type fixedSizeLayout struct{ w, h float32 }

func (f *fixedSizeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

func (f *fixedSizeLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(f.w, f.h)
}

// keep theme reference used in other files
var _ = theme.DefaultTheme()
