package ui

// liquid_button.go — iOS 26 液态玻璃风格按钮
//
// 视觉层次（从底到顶）：
//   1. glowRect   — 外发光光晕（比按钮大 4px，极低透明度）
//   2. baseRect   — 玻璃主体（半透明白色 + 大圆角）
//   3. borderRect — 玻璃边框（半透明白色描边，模拟边缘反光）
//   4. shimRect   — 顶部高光条（白色渐变，模拟玻璃折射，高度占 40%）
//   5. label      — 图标 + 文字
//
// 由于 Fyne 不支持真实背景模糊，通过多层半透明矩形 + 色彩叠加模拟毛玻璃质感。
// 按下时 baseRect 透明度加深，松开恢复，提供视觉反馈。

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// ── 液态玻璃按钮颜色常量 ─────────────────────────────────────────────────────

// 玻璃主体：白色半透明，模拟磨砂玻璃
var lgBase = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x55}

// 悬停时主体：稍微更亮
var lgBaseHover = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x72}

// 按下时主体：加深，给压感反馈
var lgBasePressed = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x38}

// 禁用时主体：更低透明度
var lgBaseDisabled = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x28}

// 玻璃边框：半透明白色，模拟玻璃边缘折射
var lgBorder = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xAA}

// 顶部高光：白色，不透明度 35%，模拟玻璃顶部折射光
var lgShimTop = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x59}

// 高光底部（渐变消失方向，用极低透明度）
var lgShimBottom = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x0A}

// 外发光：按钮颜色的半透明扩散
var lgGlow = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x18}

// 文字色：深色，保证可读性
var lgText = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0xFF}

// 禁用文字色
var lgTextDisabled = color.NRGBA{R: 0x9E, G: 0x9E, B: 0x9E, A: 0xFF}

// ── LiquidButton 液态玻璃按钮 ────────────────────────────────────────────────

// LiquidButton 是一个自定义 Widget，实现 iOS 26 液态玻璃风格按钮。
// 支持图标、文字、悬停/按下/禁用状态动画。
type LiquidButton struct {
	widget.BaseWidget

	// 配置
	Label    string
	Icon     fyne.Resource
	OnTapped func()

	// 颜色强调（影响外发光和边框色调）
	// 默认 nil 时使用白色系；传入颜色后混入对应色调
	AccentColor *color.NRGBA

	// 内部状态
	hovered  bool
	pressed  bool
	disabled bool

	// 渲染对象（由 CreateRenderer 创建并持有）
	renderer *liquidButtonRenderer
}

// NewLiquidButton 创建液态玻璃按钮
func NewLiquidButton(label string, icon fyne.Resource, tapped func()) *LiquidButton {
	b := &LiquidButton{
		Label:    label,
		Icon:     icon,
		OnTapped: tapped,
	}
	b.ExtendBaseWidget(b)
	return b
}

// NewLiquidButtonWithAccent 创建带色调的液态玻璃按钮（用于启动/停止等有语义颜色的按钮）
func NewLiquidButtonWithAccent(label string, icon fyne.Resource, accent color.NRGBA, tapped func()) *LiquidButton {
	b := NewLiquidButton(label, icon, tapped)
	b.AccentColor = &accent
	return b
}

// Enable 启用按钮
func (b *LiquidButton) Enable() {
	b.disabled = false
	b.Refresh()
}

// Disable 禁用按钮
func (b *LiquidButton) Disable() {
	b.disabled = true
	b.Refresh()
}

// Disabled 返回是否禁用
func (b *LiquidButton) Disabled() bool {
	return b.disabled
}

// Tapped 响应点击
func (b *LiquidButton) Tapped(_ *fyne.PointEvent) {
	if b.disabled {
		return
	}
	b.pressed = false
	b.Refresh()
	if b.OnTapped != nil {
		b.OnTapped()
	}
}

// TappedSecondary 右键不做任何处理
func (b *LiquidButton) TappedSecondary(_ *fyne.PointEvent) {}

// MouseIn 鼠标进入
func (b *LiquidButton) MouseIn(_ *desktop.MouseEvent) {
	if b.disabled {
		return
	}
	b.hovered = true
	b.Refresh()
}

// MouseOut 鼠标离开
func (b *LiquidButton) MouseOut() {
	b.hovered = false
	b.pressed = false
	b.Refresh()
}

// MouseMoved 鼠标移动（不处理）
func (b *LiquidButton) MouseMoved(_ *desktop.MouseEvent) {}

// FocusGained 获得焦点
func (b *LiquidButton) FocusGained() {}

// FocusLost 失去焦点
func (b *LiquidButton) FocusLost() {}

// TypedRune 键盘输入（空格/回车触发）
func (b *LiquidButton) TypedRune(r rune) {
	if r == ' ' {
		b.Tapped(nil)
	}
}

// TypedKey 键盘按键
func (b *LiquidButton) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter {
		b.Tapped(nil)
	}
}

// MinSize 最小尺寸
func (b *LiquidButton) MinSize() fyne.Size {
	b.ExtendBaseWidget(b)
	return b.BaseWidget.MinSize()
}

// CreateRenderer 创建渲染器
func (b *LiquidButton) CreateRenderer() fyne.WidgetRenderer {
	b.ExtendBaseWidget(b)

	// ── 外发光层 ────────────────────────────────────────────────
	glow := canvas.NewRectangle(lgGlow)
	glow.CornerRadius = 18

	// ── 玻璃主体 ────────────────────────────────────────────────
	base := canvas.NewRectangle(lgBase)
	base.CornerRadius = 14
	base.StrokeColor = lgBorder
	base.StrokeWidth = 1.2

	// ── 顶部高光条（占按钮高度上半部分）────────────────────────
	// 用两个矩形模拟渐变：顶部不透明 → 底部透明
	shimTop := canvas.NewRectangle(lgShimTop)
	shimTop.CornerRadius = 14

	shimBottom := canvas.NewRectangle(lgShimBottom)
	shimBottom.CornerRadius = 0 // 底部平切，让两段无缝拼接

	// ── 图标 ────────────────────────────────────────────────────
	var iconObj *canvas.Image
	if b.Icon != nil {
		iconObj = canvas.NewImageFromResource(b.Icon)
		iconObj.FillMode = canvas.ImageFillContain
		iconObj.SetMinSize(fyne.NewSize(16, 16))
	}

	// ── 文字标签 ─────────────────────────────────────────────────
	lbl := canvas.NewText(b.Label, lgText)
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	lbl.TextSize = 14
	lbl.Alignment = fyne.TextAlignCenter

	r := &liquidButtonRenderer{
		btn:        b,
		glow:       glow,
		base:       base,
		shimTop:    shimTop,
		shimBottom: shimBottom,
		iconObj:    iconObj,
		lbl:        lbl,
	}
	b.renderer = r
	return r
}

// ── liquidButtonRenderer ─────────────────────────────────────────────────────

type liquidButtonRenderer struct {
	btn        *LiquidButton
	glow       *canvas.Rectangle
	base       *canvas.Rectangle
	shimTop    *canvas.Rectangle
	shimBottom *canvas.Rectangle
	iconObj    *canvas.Image
	lbl        *canvas.Text
}

func (r *liquidButtonRenderer) Layout(size fyne.Size) {
	const glowPad float32 = 4
	const cornerRadius float32 = 14

	// 外发光比按钮大 glowPad*2，居中
	r.glow.Move(fyne.NewPos(-glowPad, -glowPad))
	r.glow.Resize(fyne.NewSize(size.Width+glowPad*2, size.Height+glowPad*2))

	// 玻璃主体铺满
	r.base.Move(fyne.NewPos(0, 0))
	r.base.Resize(size)

	// 顶部高光：上半部分（约40%高度），顶角圆角和主体一致，底部平
	shimH := size.Height * 0.42
	r.shimTop.Move(fyne.NewPos(1, 1))
	r.shimTop.Resize(fyne.NewSize(size.Width-2, shimH*0.5))
	r.shimTop.CornerRadius = cornerRadius

	r.shimBottom.Move(fyne.NewPos(1, 1+shimH*0.5))
	r.shimBottom.Resize(fyne.NewSize(size.Width-2, shimH*0.5))

	// 图标 + 文字水平居中排列
	const iconSize float32 = 16
	const gap float32 = 5

	var contentW float32
	hasIcon := r.iconObj != nil && r.btn.Icon != nil
	if hasIcon {
		// 粗略估算文字宽度（无法精确，用字符数 * fontSize * 0.6 近似）
		textW := float32(len([]rune(r.btn.Label))) * 14 * 0.62
		contentW = iconSize + gap + textW
	} else {
		contentW = float32(len([]rune(r.btn.Label))) * 14 * 0.62
	}

	startX := (size.Width - contentW) / 2
	centerY := (size.Height - iconSize) / 2

	if hasIcon {
		r.iconObj.Move(fyne.NewPos(startX, centerY))
		r.iconObj.Resize(fyne.NewSize(iconSize, iconSize))
		r.lbl.Move(fyne.NewPos(startX+iconSize+gap, (size.Height-14)/2))
	} else {
		r.lbl.Move(fyne.NewPos(startX, (size.Height-14)/2))
	}
	r.lbl.Resize(fyne.NewSize(contentW, 20))
}

func (r *liquidButtonRenderer) MinSize() fyne.Size {
	const hPad float32 = 20
	const vPad float32 = 10
	const iconSize float32 = 16
	const gap float32 = 5

	textW := float32(len([]rune(r.btn.Label))) * 14 * 0.62
	var w float32
	if r.iconObj != nil && r.btn.Icon != nil {
		w = iconSize + gap + textW + hPad*2
	} else {
		w = textW + hPad*2
	}
	if w < 80 {
		w = 80
	}
	return fyne.NewSize(w, 38)
}

func (r *liquidButtonRenderer) Refresh() {
	b := r.btn

	// ── 根据状态更新颜色 ─────────────────────────────────────────

	// 计算主体色（混入 AccentColor 色调）
	var baseCol color.NRGBA
	switch {
	case b.disabled:
		baseCol = lgBaseDisabled
	case b.pressed:
		baseCol = lgBasePressed
	case b.hovered:
		baseCol = lgBaseHover
	default:
		baseCol = lgBase
	}

	// 如果有强调色，混入 20% 的色调到主体背景
	if b.AccentColor != nil && !b.disabled {
		ac := *b.AccentColor
		mix := func(a, c uint8, t float32) uint8 {
			return uint8(float32(a)*(1-t) + float32(c)*t)
		}
		baseCol.R = mix(baseCol.R, ac.R, 0.18)
		baseCol.G = mix(baseCol.G, ac.G, 0.18)
		baseCol.B = mix(baseCol.B, ac.B, 0.18)

		// 外发光也染上强调色
		r.glow.FillColor = color.NRGBA{R: ac.R, G: ac.G, B: ac.B, A: 0x22}

		// 边框也偏向强调色
		r.base.StrokeColor = color.NRGBA{
			R: mix(0xFF, ac.R, 0.3),
			G: mix(0xFF, ac.G, 0.3),
			B: mix(0xFF, ac.B, 0.3),
			A: 0xBB,
		}
	} else {
		r.glow.FillColor = lgGlow
		r.base.StrokeColor = lgBorder
	}

	r.base.FillColor = baseCol

	// 悬停时高光加强
	if b.hovered && !b.disabled {
		r.shimTop.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x72}
	} else {
		r.shimTop.FillColor = lgShimTop
	}
	r.shimBottom.FillColor = lgShimBottom

	// 禁用时文字变灰
	if b.disabled {
		r.lbl.Color = lgTextDisabled
	} else {
		r.lbl.Color = lgText
	}

	// 更新图标透明度（禁用时变灰，通过 opacity 模拟）
	if r.iconObj != nil {
		if b.disabled {
			r.iconObj.Translucency = 0.6
		} else {
			r.iconObj.Translucency = 0
		}
		r.iconObj.Refresh()
	}

	r.lbl.Refresh()
	r.base.Refresh()
	r.shimTop.Refresh()
	r.shimBottom.Refresh()
	r.glow.Refresh()
}

func (r *liquidButtonRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.glow, r.base, r.shimTop, r.shimBottom}
	if r.iconObj != nil && r.btn.Icon != nil {
		objs = append(objs, r.iconObj)
	}
	objs = append(objs, r.lbl)
	return objs
}

func (r *liquidButtonRenderer) Destroy() {}

// ── 液态玻璃底部操作栏 ───────────────────────────────────────────────────────

// liquidButtonBar 构建使用液态玻璃按钮的底部操作栏
// 替换原来的普通 widget.Button，整体底栏背景也使用液态玻璃风格
func liquidButtonBar(startBtn, stopBtn, copyBtn fyne.CanvasObject, statusText string) fyne.CanvasObject {
	// 底栏背景：深色半透明，强化玻璃效果
	barBg := canvas.NewRectangle(color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0xE8})

	// 顶部高光线（液态玻璃特有的上边缘反光）
	topGlow := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x55})
	topGlow.SetMinSize(fyne.NewSize(0, 1))

	// 状态芯片（液态玻璃风格）
	statusChip := liquidStatusChip("● "+statusText)

	// 按钮间用半透明竖线分隔
	sep := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x28})
	sep.SetMinSize(fyne.NewSize(1, 24))

	buttons := container.NewHBox(startBtn, stopBtn, sep, copyBtn)

	row := container.NewBorder(nil, nil, nil, statusChip, buttons)
	foreground := container.NewVBox(topGlow, container.NewPadded(row))

	return container.NewStack(barBg, foreground)
}

// liquidStatusChip 液态玻璃风格状态芯片
func liquidStatusChip(text string) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x18})
	bg.CornerRadius = 12
	bg.StrokeColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x55}
	bg.StrokeWidth = 1

	shim := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x28})
	shim.CornerRadius = 12

	lbl := widget.NewLabel(text)

	return container.NewPadded(container.NewStack(bg, shim, container.NewPadded(lbl)))
}
