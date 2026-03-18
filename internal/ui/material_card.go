package ui

// material_card.go — Material Design 3 风格卡片与辅助控件
//
// 设计原则：
//   - 卡片：白色背景 + 精致多层阴影 + 左侧彩色强调条（6px）+ 圆角 10px
//   - 标题行：语义色块（圆角正方形）+ 半透明背景浮层 + 粗体标题
//   - 字段行：彩色小圆点 + 固定标签宽度，排版整齐
//   - 标题横幅：Amber 渐变底色 + 几何波纹装饰 + 顶部高光线 + 底部强调线
//   - 日志面板：Border 布局确保内容填满剩余高度

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ── 卡片强调色系统（Material You 色彩角色）─────────────────────────────────
//
// 每张卡片使用独立颜色，形成清晰的视觉区分和语义识别
var mdAccentColors = []color.NRGBA{
	{R: 0xFF, G: 0xB3, B: 0x00, A: 0xFF}, // [0] Amber 700  — 账号信息
	{R: 0x19, G: 0x76, B: 0xD2, A: 0xFF}, // [1] Blue 700   — 网络配置
	{R: 0x2E, G: 0x7D, B: 0x32, A: 0xFF}, // [2] Green 800  — 时间设置（加深，更醒目）
	{R: 0xC6, G: 0x28, B: 0x28, A: 0xFF}, // [3] Red 700    — 课程类型（加深）
	{R: 0x6A, G: 0x1B, B: 0x9A, A: 0xFF}, // [4] Purple 800 — 课程分类（加深）
	{R: 0x00, G: 0x6D, B: 0x63, A: 0xFF}, // [5] Teal 700   — 筛选条件（加深）
	{R: 0xE6, G: 0x51, B: 0x00, A: 0xFF}, // [6] Orange 900 — 运行日志
}

// accentBgAlpha 强调条对应的标题行背景（极低透明度，制造呼吸感）
const accentBgAlpha = 0x0C // ~5% opacity

// ── materialCard 标准内容卡片 ────────────────────────────────────────────────
//
//	title     — 卡片标题（空字符串则不显示标题行）
//	accentIdx — 左侧强调条颜色索引（对应 mdAccentColors）
//	content   — 卡片内容区域
//
// 视觉层次（底→顶）：
//
//	outerShadow — 外层柔阴影（偏移 3px，极低透明度）
//	innerShadow — 内层精阴影（偏移 1.5px，低透明度）
//	bg          — 白色圆角背景 + 细描边
//	accentBar   — 左侧 6px 彩色强调条
//	header      — 标题行（色块 + 文字 + 半透明浮层）
//	content     — 内容区
func materialCard(title string, accentIdx int, content fyne.CanvasObject) fyne.CanvasObject {
	accent := mdAccentColors[accentIdx%len(mdAccentColors)]

	// ── 多层阴影（模拟 Material Elevation 2dp）────────────────────────────
	// 外层阴影：更大偏移，极低透明度，营造空间感
	outerShadow := canvas.NewRectangle(color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0x0D})
	outerShadow.CornerRadius = 13
	// 内层阴影：精细阴影，增强立体感
	innerShadow := canvas.NewRectangle(color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0x18})
	innerShadow.CornerRadius = 11

	// ── 卡片背景 ──────────────────────────────────────────────────────────
	bg := canvas.NewRectangle(mdSurface)
	bg.CornerRadius = 10
	// 边框：冷灰 + 微量强调色染色
	bg.StrokeColor = color.NRGBA{
		R: blendU8(0xE8, accent.R, 0.08),
		G: blendU8(0xEC, accent.G, 0.08),
		B: blendU8(0xF0, accent.B, 0.08),
		A: 0xFF,
	}
	bg.StrokeWidth = 1

	// ── 左侧彩色强调条（6px，顶部圆角）─────────────────────────────────
	accentBar := canvas.NewRectangle(accent)
	accentBar.CornerRadius = 5
	accentBar.SetMinSize(fyne.NewSize(6, 0))

	// ── 标题行 ────────────────────────────────────────────────────────────
	var header fyne.CanvasObject
	if title != "" {
		// 标题行背景浮层（极低透明度，与强调色系一致）
		headerBg := canvas.NewRectangle(color.NRGBA{
			R: accent.R, G: accent.G, B: accent.B, A: accentBgAlpha,
		})
		headerBg.CornerRadius = 0

		// 语义色块：14×14 圆角正方形，与强调条同色
		dot := canvas.NewRectangle(accent)
		dot.CornerRadius = 4
		dot.SetMinSize(fyne.NewSize(14, 14))

		// 标题文字：粗体，使用主题前景色
		titleLabel := widget.NewRichTextFromMarkdown("**" + title + "**")

		titleRow := container.NewBorder(nil, nil,
			container.NewPadded(dot), nil,
			container.NewPadded(titleLabel),
		)
		headerContent := container.NewStack(headerBg, titleRow)

		// 标题下方精致分割线（强调色调）
		divider := canvas.NewRectangle(color.NRGBA{
			R: blendU8(0xF0, accent.R, 0.12),
			G: blendU8(0xF0, accent.G, 0.12),
			B: blendU8(0xF0, accent.B, 0.12),
			A: 0xFF,
		})
		divider.SetMinSize(fyne.NewSize(0, 1))
		header = container.NewVBox(headerContent, divider)
	}

	// ── 内容区 ────────────────────────────────────────────────────────────
	var innerContent fyne.CanvasObject
	if header != nil {
		innerContent = container.NewVBox(header, container.NewPadded(content))
	} else {
		innerContent = container.NewPadded(content)
	}

	// ── 强调条 + 内容横向排列 ─────────────────────────────────────────────
	cardBody := container.NewBorder(nil, nil, accentBar, nil, innerContent)

	// ── Stack：多层阴影 → 背景 → 内容 ────────────────────────────────────
	card := container.NewStack(outerShadow, innerShadow, bg, cardBody)

	// 外层 Padded：给卡片四周留 8px 呼吸空间
	return container.NewPadded(card)
}

// ── buildLogPanel 日志面板（填满剩余高度）──────────────────────────────────
//
// 使用 Border 布局确保日志内容区填满整个面板（VBox 不拉伸子元素）
func buildLogPanel(title string, accentIdx int, content fyne.CanvasObject) fyne.CanvasObject {
	accent := mdAccentColors[accentIdx%len(mdAccentColors)]

	// ── 多层阴影 ──────────────────────────────────────────────────────────
	outerShadow := canvas.NewRectangle(color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0x0D})
	outerShadow.CornerRadius = 13
	innerShadow := canvas.NewRectangle(color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0x18})
	innerShadow.CornerRadius = 11

	bg := canvas.NewRectangle(mdSurface)
	bg.CornerRadius = 10
	bg.StrokeColor = color.NRGBA{
		R: blendU8(0xE8, accent.R, 0.08),
		G: blendU8(0xEC, accent.G, 0.08),
		B: blendU8(0xF0, accent.B, 0.08),
		A: 0xFF,
	}
	bg.StrokeWidth = 1

	// ── 左侧强调条 ────────────────────────────────────────────────────────
	accentBar := canvas.NewRectangle(accent)
	accentBar.CornerRadius = 5
	accentBar.SetMinSize(fyne.NewSize(6, 0))

	// ── 标题行 ────────────────────────────────────────────────────────────
	headerBg := canvas.NewRectangle(color.NRGBA{
		R: accent.R, G: accent.G, B: accent.B, A: accentBgAlpha,
	})

	dot := canvas.NewRectangle(accent)
	dot.CornerRadius = 4
	dot.SetMinSize(fyne.NewSize(14, 14))

	titleLabel := widget.NewRichTextFromMarkdown("**" + title + "**")
	titleRow := container.NewBorder(nil, nil,
		container.NewPadded(dot), nil,
		container.NewPadded(titleLabel),
	)
	headerContent := container.NewStack(headerBg, titleRow)

	divider := canvas.NewRectangle(color.NRGBA{
		R: blendU8(0xF0, accent.R, 0.12),
		G: blendU8(0xF0, accent.G, 0.12),
		B: blendU8(0xF0, accent.B, 0.12),
		A: 0xFF,
	})
	divider.SetMinSize(fyne.NewSize(0, 1))
	header := container.NewVBox(headerContent, divider)

	// ── Border：header 固定顶部，content 填满剩余 ─────────────────────────
	inner := container.NewBorder(header, nil, nil, nil, container.NewPadded(content))
	cardBody := container.NewBorder(nil, nil, accentBar, nil, inner)

	return container.NewStack(outerShadow, innerShadow, bg, cardBody)
}

// ── buildTitleBanner 顶部标题横幅 ────────────────────────────────────────────
//
// 设计语言：
//   - Amber 700 深色底（与之前统一，但增加层次）
//   - 左侧渐变光晕（圆形几何装饰）
//   - 顶部单像素高光线（玻璃折射感）
//   - 底部 3px Amber 800 强调线（增强与内容的分界感）
//   - 右侧装饰圆（半透明波纹背景）
//   - 左侧图标盒子（磨砂玻璃质感，圆角 14px）
func buildTitleBanner() fyne.CanvasObject {
	// ── 主背景：Amber 700 ─────────────────────────────────────────────────
	bg := canvas.NewRectangle(mdPrimary)
	bg.CornerRadius = 0

	// ── 背景暗部叠加（增加深度，防止过于扁平）─────────────────────────────
	darkOverlay := canvas.NewRectangle(color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x14})

	// ── 装饰性几何圆（右侧，半透明白色，营造波纹感）──────────────────────
	circle1 := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x16})
	circle1.CornerRadius = 70
	circle1.SetMinSize(fyne.NewSize(130, 130))

	circle2 := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x0C})
	circle2.CornerRadius = 50
	circle2.SetMinSize(fyne.NewSize(80, 80))

	circle3 := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x08})
	circle3.CornerRadius = 30
	circle3.SetMinSize(fyne.NewSize(40, 40))

	// ── 顶部高光线（模拟玻璃上边缘折射）────────────────────────────────────
	topShim := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x55})
	topShim.SetMinSize(fyne.NewSize(0, 2))

	// ── 底部深色强调线（Amber 800，与主背景形成细微层次）────────────────────
	bottomLine := canvas.NewRectangle(mdPrimaryDark)
	bottomLine.SetMinSize(fyne.NewSize(0, 3))

	// ── 主标题文字 ────────────────────────────────────────────────────────
	title := canvas.NewText("GCC 课程选课助手  V3.1", color.White)
	title.TextSize = 18
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	// ── 副标题（微信蓝模糊感，高对比白 + 80% 透明）───────────────────────
	subtitle := canvas.NewText("自动化选课工具  ·  仅供学习研究使用", color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xCC})
	subtitle.TextSize = 12
	subtitle.Alignment = fyne.TextAlignCenter

	// ── 文字列（顶部高光 + 标题 + 副标题 + 底部强调线）─────────────────────
	textCol := container.NewVBox(
		topShim,
		container.NewPadded(container.NewVBox(
			container.NewPadded(title),
			subtitle,
		)),
		bottomLine,
	)

	// ── 左侧图标装饰盒（磨砂玻璃感：白色半透明 + 圆角）────────────────────
	iconBox := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x28})
	iconBox.CornerRadius = 14
	iconBox.StrokeColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x44}
	iconBox.StrokeWidth = 1
	iconBox.SetMinSize(fyne.NewSize(48, 48))

	// 图标盒内的高光上段（模拟玻璃折射）
	iconBoxShim := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x20})
	iconBoxShim.CornerRadius = 14
	iconBoxShim.SetMinSize(fyne.NewSize(48, 22))

	iconDeco := container.NewPadded(container.NewStack(iconBox, iconBoxShim))

	// ── 右侧装饰组合（三个圆叠加）────────────────────────────────────────
	rightDeco := container.NewPadded(container.NewStack(circle3, circle2))

	// ── 前景布局：图标装饰 | 文字列 | 右侧装饰 ──────────────────────────
	foreground := container.NewBorder(nil, nil, iconDeco, rightDeco, textCol)

	// ── Stack 最终合成 ────────────────────────────────────────────────────
	return container.NewStack(bg, darkOverlay, circle1, foreground)
}

// ── 字段行辅助函数 ────────────────────────────────────────────────────────────

// mdFieldRow 标签 + 输入控件一行排列
//
// 布局：[彩色圆点] [标签文字] ... [输入控件（填充剩余宽度）]
// 圆点与标题卡片的强调色一致，形成视觉关联
func mdFieldRow(label string, input fyne.CanvasObject) fyne.CanvasObject {
	// 彩色小圆点：使用主题主色（Amber），直径 6px
	dot := canvas.NewRectangle(mdPrimary)
	dot.CornerRadius = 3
	dot.SetMinSize(fyne.NewSize(6, 6))

	lbl := container.NewHBox(dot, widget.NewLabel(label))
	return container.NewBorder(nil, nil, container.NewPadded(lbl), nil, input)
}

// mdSectionDivider 字段间精致分割线（1px，带 4px 上下间距）
func mdSectionDivider() fyne.CanvasObject {
	line := canvas.NewRectangle(mdSeparator)
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

// mdChip 彩色标签芯片（语义化小标签，用于强调重要信息）
func mdChip(text string, accent color.NRGBA) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: accent.R, G: accent.G, B: accent.B, A: 0x22})
	bg.CornerRadius = 12
	bg.StrokeColor = color.NRGBA{R: accent.R, G: accent.G, B: accent.B, A: 0x55}
	bg.StrokeWidth = 1
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

// ── mdButtonBar Material FAB 风格操作栏（备用）────────────────────────────────
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

// ── blendU8 颜色混合辅助（线性插值）─────────────────────────────────────────
// a: 基础色分量, c: 目标色分量, t: 混合权重 [0.0, 1.0]
func blendU8(a, c uint8, t float64) uint8 {
	result := float64(a)*(1-t) + float64(c)*t
	if result > 255 {
		return 255
	}
	if result < 0 {
		return 0
	}
	return uint8(result)
}

// keep theme reference to avoid import cycle warnings
var _ = theme.DefaultTheme()
