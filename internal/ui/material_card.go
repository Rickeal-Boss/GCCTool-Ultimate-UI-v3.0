package ui

// material_card.go — Material Design 3 风格卡片与辅助控件
//
// 原则：
//   - 所有布局用 Fyne 内置 container（Border / Stack / HBox / VBox / Grid），
//     不再自定义 fyne.Layout，避免 resize 时递归重布局
//   - 卡片白色背景 + 左侧彩色强调条 + 圆角 + 投影，形成明确的视觉层次
//   - 标题行带语义小色块，内容区用 Padded 包裹留出呼吸感
//   - 所有废弃的自定义 layout（bannerLayout / barLayout / chipLayout）已删除

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ── 卡片强调色（每种卡片独立，形成视觉区分）────────────────────────────────
var mdAccentColors = []color.NRGBA{
	{R: 0xFF, G: 0xB3, B: 0x00, A: 0xFF}, // Amber  — 账号信息
	{R: 0x19, G: 0x76, B: 0xD2, A: 0xFF}, // Blue   — 网络配置
	{R: 0x43, G: 0xA0, B: 0x47, A: 0xFF}, // Green  — 时间设置
	{R: 0xE5, G: 0x39, B: 0x35, A: 0xFF}, // Red    — 课程类型
	{R: 0x7B, G: 0x1F, B: 0xA2, A: 0xFF}, // Purple — 课程分类
	{R: 0x00, G: 0x89, B: 0x7B, A: 0xFF}, // Teal   — 筛选条件
	{R: 0xF5, G: 0x7C, B: 0x00, A: 0xFF}, // Orange — 运行日志
}

// ── materialCard 标准内容卡片 ────────────────────────────────────────────────
//
//	title     — 卡片标题（空字符串则不显示标题行）
//	accentIdx — 左侧强调条颜色索引
//	content   — 卡片内容
//
// 布局层次（底→顶）：
//
//	shadowRect  — 投影矩形（偏移 2px，深色半透明）
//	bg          — 白色圆角背景
//	accentBar   — 左侧 4px 彩色条
//	header+content — 标题行 + 内容
func materialCard(title string, accentIdx int, content fyne.CanvasObject) fyne.CanvasObject {
	accent := mdAccentColors[accentIdx%len(mdAccentColors)]

	// ── 投影层（比卡片大 2px，右下偏移，模拟 dp2 阴影）────────────────────
	shadowRect := canvas.NewRectangle(color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x14})
	shadowRect.CornerRadius = 10

	// ── 白色卡片背景 ──────────────────────────────────────────────────────────
	bg := canvas.NewRectangle(mdSurface)
	bg.CornerRadius = 8
	bg.StrokeColor = color.NRGBA{R: 0xE8, G: 0xE8, B: 0xE8, A: 0xFF}
	bg.StrokeWidth = 1

	// ── 左侧彩色强调条（4px 宽，顶部圆角，底部平）─────────────────────────
	accentBar := canvas.NewRectangle(accent)
	accentBar.CornerRadius = 4
	accentBar.SetMinSize(fyne.NewSize(4, 0))

	// ── 标题行 ────────────────────────────────────────────────────────────────
	var header fyne.CanvasObject
	if title != "" {
		// 语义色块：12×12 圆角正方形，与卡片强调色一致
		dot := canvas.NewRectangle(accent)
		dot.CornerRadius = 3
		dot.SetMinSize(fyne.NewSize(12, 12))

		titleLabel := widget.NewRichTextFromMarkdown("**" + title + "**")

		// 标题下方轻量分割线
		divider := canvas.NewRectangle(color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF})
		divider.SetMinSize(fyne.NewSize(0, 1))

		titleRow := container.NewBorder(nil, nil, container.NewPadded(dot), nil,
			container.NewPadded(titleLabel))
		header = container.NewVBox(titleRow, divider)
	}

	// ── 内容区 ────────────────────────────────────────────────────────────────
	var innerContent fyne.CanvasObject
	if header != nil {
		innerContent = container.NewVBox(header, container.NewPadded(content))
	} else {
		innerContent = container.NewPadded(content)
	}

	// ── 强调条 + 内容区横向排列 ───────────────────────────────────────────────
	cardBody := container.NewBorder(nil, nil, accentBar, nil, innerContent)

	// ── Stack：shadowRect → bg → cardBody ────────────────────────────────────
	// 用 CustomPaddedLayout 把 shadowRect 向右下偏移 2px 实现阴影感
	card := container.NewStack(shadowRect, bg, cardBody)

	// 外层 Padded：给卡片四周留 6px 呼吸空间（投影不被裁剪）
	return container.NewPadded(card)
}

// ── buildLogPanel 日志面板（内容区填满剩余高度）──────────────────────────────
//
// 不用 materialCard 的 VBox 包装（VBox 不拉伸子元素），
// 用 Border：标题行固定，content 填充剩余全部高度。
func buildLogPanel(title string, accentIdx int, content fyne.CanvasObject) fyne.CanvasObject {
	accent := mdAccentColors[accentIdx%len(mdAccentColors)]

	// ── 投影 + 背景 ───────────────────────────────────────────────────────────
	shadowRect := canvas.NewRectangle(color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x14})
	shadowRect.CornerRadius = 10
	bg := canvas.NewRectangle(mdSurface)
	bg.CornerRadius = 8
	bg.StrokeColor = color.NRGBA{R: 0xE8, G: 0xE8, B: 0xE8, A: 0xFF}
	bg.StrokeWidth = 1

	// ── 左侧强调条 ────────────────────────────────────────────────────────────
	accentBar := canvas.NewRectangle(accent)
	accentBar.CornerRadius = 4
	accentBar.SetMinSize(fyne.NewSize(4, 0))

	// ── 标题行 ────────────────────────────────────────────────────────────────
	dot := canvas.NewRectangle(accent)
	dot.CornerRadius = 3
	dot.SetMinSize(fyne.NewSize(12, 12))

	titleLabel := widget.NewRichTextFromMarkdown("**" + title + "**")
	titleRow := container.NewBorder(nil, nil, container.NewPadded(dot), nil,
		container.NewPadded(titleLabel))

	divider := canvas.NewRectangle(color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF})
	divider.SetMinSize(fyne.NewSize(0, 1))

	header := container.NewVBox(titleRow, divider)

	// ── Border：header 固定顶部，content 填满剩余 ─────────────────────────────
	inner := container.NewBorder(header, nil, nil, nil, container.NewPadded(content))

	// ── 强调条 + 内容横向 ─────────────────────────────────────────────────────
	cardBody := container.NewBorder(nil, nil, accentBar, nil, inner)

	return container.NewStack(shadowRect, bg, cardBody)
}

// ── buildTitleBanner 顶部 Material 标题横幅 ──────────────────────────────────
//
// Amber 渐变底色 + 图标装饰 + 双行文字
func buildTitleBanner() fyne.CanvasObject {
	// 主背景：Amber 700
	bg := canvas.NewRectangle(mdPrimary)
	bg.CornerRadius = 0

	// 右下装饰圆（半透明，模拟波纹背景）
	circle1 := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x1A})
	circle1.CornerRadius = 60
	circle1.SetMinSize(fyne.NewSize(100, 100))

	circle2 := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x0D})
	circle2.CornerRadius = 40
	circle2.SetMinSize(fyne.NewSize(60, 60))

	// 顶部高光线（模拟玻璃材质上边缘）
	topShim := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x66})
	topShim.SetMinSize(fyne.NewSize(0, 2))

	// 主标题
	title := widget.NewRichTextFromMarkdown("**GCC 课程选课助手  V3.0**")
	title.Alignment = fyne.TextAlignCenter

	// 副标题
	subtitle := canvas.NewText("自动化选课工具  ·  仅供学习研究使用", color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xCC})
	subtitle.TextSize = 12
	subtitle.Alignment = fyne.TextAlignCenter

	// 底部分割线
	bottomLine := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0x8F, B: 0x00, A: 0xFF})
	bottomLine.SetMinSize(fyne.NewSize(0, 3))

	textCol := container.NewVBox(
		topShim,
		container.NewPadded(container.NewVBox(
			container.NewPadded(title),
			subtitle,
		)),
		bottomLine,
	)

	// 左侧图标装饰
	iconDeco := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x33})
	iconDeco.CornerRadius = 12
	iconDeco.SetMinSize(fyne.NewSize(44, 44))

	foreground := container.NewBorder(nil, nil,
		container.NewPadded(iconDeco),
		container.NewPadded(circle2),
		textCol,
	)

	return container.NewStack(bg, circle1, foreground)
}

// ── 字段行辅助函数 ────────────────────────────────────────────────────────────

// mdFieldRow 标签 + 输入控件一行排列
// 标签区宽度固定 80px，输入控件填充剩余宽度
func mdFieldRow(label string, input fyne.CanvasObject) fyne.CanvasObject {
	// 小圆点装饰
	dot := canvas.NewRectangle(mdPrimary)
	dot.CornerRadius = 3
	dot.SetMinSize(fyne.NewSize(5, 5))

	lbl := container.NewHBox(dot, widget.NewLabel(label))
	return container.NewBorder(nil, nil, container.NewPadded(lbl), nil, input)
}

// mdSectionDivider 字段间轻量分隔线（带上下 4px 间距）
func mdSectionDivider() fyne.CanvasObject {
	line := canvas.NewRectangle(color.NRGBA{R: 0xEE, G: 0xEE, B: 0xEE, A: 0xFF})
	line.SetMinSize(fyne.NewSize(0, 1))
	return container.NewPadded(line)
}

// mdIconLabel 带图标或圆点装饰的字段标签
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

// mdChip 液态玻璃风格标签芯片（用于小标题强调）
func mdChip(text string, accent color.NRGBA) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: accent.R, G: accent.G, B: accent.B, A: 0x20})
	bg.CornerRadius = 12
	lbl := widget.NewLabel(text)
	return container.NewPadded(container.NewStack(bg, lbl))
}

// mdInputWithIcon 带前缀图标的输入框包装
func mdInputWithIcon(icon fyne.Resource, entry fyne.CanvasObject) fyne.CanvasObject {
	if icon == nil {
		return entry
	}
	ic := widget.NewIcon(icon)
	iconBox := container.New(&fixedSizeLayout{w: 22, h: 22}, ic)
	return container.NewBorder(nil, nil, iconBox, nil, entry)
}

// ── mdButtonBar Material FAB 风格操作栏（备用，主按钮栏使用 liquidButtonBar）──
func mdButtonBar(startBtn, stopBtn, copyBtn fyne.CanvasObject, statusText string) fyne.CanvasObject {
	barBg := canvas.NewRectangle(mdSurface)
	topLine := canvas.NewRectangle(mdPrimary)
	topLine.SetMinSize(fyne.NewSize(0, 3))
	statusChip := mdChip("● "+statusText, color.NRGBA{R: 0x43, G: 0xA0, B: 0x47, A: 0xFF})
	sep := canvas.NewRectangle(color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF})
	sep.SetMinSize(fyne.NewSize(1, 24))
	buttons := container.NewHBox(startBtn, stopBtn, sep, copyBtn)
	row := container.NewBorder(nil, nil, nil, statusChip, buttons)
	foreground := container.NewVBox(topLine, container.NewPadded(row))
	return container.NewStack(barBg, foreground)
}

// ── fixedSizeLayout 固定最小尺寸布局 ─────────────────────────────────────────
// 用于给不支持 SetMinSize 的 widget 设定固定尺寸（如 widget.Icon）
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

// keep theme reference to avoid import cycle warnings
var _ = theme.DefaultTheme()
