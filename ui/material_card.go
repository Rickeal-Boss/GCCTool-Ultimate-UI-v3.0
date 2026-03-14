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
func materialCard(title string, accentIdx int, content fyne.CanvasObject) fyne.CanvasObject {
	accent := mdAccentColors[accentIdx%len(mdAccentColors)]

	// ── 阴影层（偏移 2px，半透明深灰）──────────────────────────────
	shadow := canvas.NewRectangle(color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x18})
	shadow.CornerRadius = 10
	shadow.SetMinSize(fyne.NewSize(0, 0))

	// ── 卡片白色背景 ──────────────────────────────────────────────
	bg := canvas.NewRectangle(color.White)
	bg.CornerRadius = 8

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
		divider := canvas.NewRectangle(color.NRGBA{R: 0xF5, G: 0xF5, B: 0xF5, A: 0xFF})
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

	// ── 叠层：阴影在最底，白底在中，cardBody 在最上 ───────────────
	shadowOffset := container.New(&shadowLayout{}, shadow, cardBody)

	// 外层加 padding，让阴影有空间显示
	return container.NewPadded(
		container.New(&cardStackLayout{bg: bg}, shadowOffset),
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// shadowLayout 让阴影矩形比内容大 2px 并向右下偏移，模拟 Material elevation
// ─────────────────────────────────────────────────────────────────────────────
type shadowLayout struct{}

func (s *shadowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	// shadow 偏移 (2,3)，稍大
	objects[0].Resize(fyne.NewSize(size.Width+2, size.Height+2))
	objects[0].Move(fyne.NewPos(2, 3))
	// 实际内容填满
	objects[1].Resize(size)
	objects[1].Move(fyne.NewPos(0, 0))
}

func (s *shadowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, 0)
	}
	return objects[1].MinSize()
}

// ─────────────────────────────────────────────────────────────────────────────
// cardStackLayout 将白色背景铺满，内容叠在上面
// ─────────────────────────────────────────────────────────────────────────────
type cardStackLayout struct {
	bg *canvas.Rectangle
}

func (c *cardStackLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	c.bg.Resize(size)
	c.bg.Move(fyne.NewPos(0, 0))
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

func (c *cardStackLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
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

// ─────────────────────────────────────────────────────────────────────────────
// buildTitleBanner 构建顶部 Material 标题横幅（Amber 渐变底色）
// ─────────────────────────────────────────────────────────────────────────────
func buildTitleBanner() fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0xFF})
	bg.CornerRadius = 0

	icon := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x33})
	icon.CornerRadius = 50
	icon.SetMinSize(fyne.NewSize(48, 48))

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

	content := container.NewBorder(nil, nil,
		container.NewPadded(icon), nil,
		container.NewPadded(textCol),
	)

	return container.New(&bannerLayout{bg: bg}, content)
}

// bannerLayout 横幅布局：bg 铺满，content 叠加
type bannerLayout struct {
	bg *canvas.Rectangle
}

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
func mdChip(text string, accent color.NRGBA) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: accent.R, G: accent.G, B: accent.B, A: 0x1A})
	bg.CornerRadius = 12

	lbl := widget.NewLabel(text)

	return container.New(&chipLayout{bg: bg}, lbl)
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
func mdInputWithIcon(icon fyne.Resource, entry fyne.CanvasObject) fyne.CanvasObject {
	if icon == nil {
		return entry
	}
	ic := widget.NewIcon(icon)
	ic.SetMinSize(fyne.NewSize(20, 20))
	return container.NewBorder(nil, nil, ic, nil, entry)
}

// ─── 底部按钮栏 Material FAB 风格 ────────────────────────────────────────────

// mdButtonBar 构建带 Material 样式的底部操作栏
func mdButtonBar(startBtn, stopBtn, copyBtn fyne.CanvasObject, statusText string) fyne.CanvasObject {
	barBg := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})
	barBg.CornerRadius = 0

	topLine := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0xFF})
	topLine.SetMinSize(fyne.NewSize(0, 3))

	statusLabel := widget.NewLabelWithStyle(
		statusText,
		fyne.TextAlignTrailing,
		fyne.TextStyle{Italic: true},
	)

	// 芯片状态指示
	statusChip := mdChip("● "+statusText, color.NRGBA{R: 0x43, G: 0xA0, B: 0x47, A: 0xFF})
	_ = statusLabel

	buttons := container.NewHBox(startBtn, stopBtn,
		canvas.NewRectangle(color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}),
		copyBtn,
	)

	row := container.NewBorder(nil, nil, nil, statusChip, buttons)
	inner := container.NewVBox(topLine, container.NewPadded(row))

	return container.New(&barLayout{bg: barBg}, inner)
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

// keep theme reference used in other files
var _ = theme.DefaultTheme()
