package ui

// liquid_button.go — iOS 26 液态玻璃风格按钮
//
// 视觉层次（底→顶）：
//  1. glowRect   — 外发光光晕（比按钮大 4px，极低透明度）
//  2. baseRect   — 玻璃主体（半透明白色 + 大圆角 + 彩色描边）
//  3. shimTop    — 顶部高光上段（白色半透明，模拟玻璃折射，顶角圆角）
//  4. shimBottom — 顶部高光下段（极低透明，渐隐）
//  5. label + icon — 图标 + 文字
//
// 状态机：
//   normal → hovered → pressed → released (tapped)
//   normal → disabled
//
// 优化：
//   - MouseDown/MouseUp 分离：mousedown 立即更新 pressed=true 给视觉反馈，
//     不需要等 Tapped 回调（Tapped 在 mouseup 后触发，会有 1 帧延迟感）
//   - fyne.MeasureText 精确测量文字宽度，消除"文字截断"或"按钮太宽"问题

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ── 颜色常量 ─────────────────────────────────────────────────────────────────

var lgBase = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x55}        // 正常主体
var lgBaseHover = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x72}   // 悬停
var lgBasePressed = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x38} // 按下（加深）
var lgBaseDisabled = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x28} // 禁用
var lgBorder = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xAA}      // 玻璃边框
var lgShimTop = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x59}     // 高光上段
var lgShimBottom = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x0A}  // 高光下段
var lgGlow = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x18}        // 外发光
var lgText = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0xFF}        // 正常文字
var lgTextDisabled = color.NRGBA{R: 0x9E, G: 0x9E, B: 0x9E, A: 0xFF} // 禁用文字

// ── LiquidButton ─────────────────────────────────────────────────────────────

// LiquidButton 液态玻璃风格自定义 Widget。
// 实现 desktop.Hoverable + desktop.Mouseable，响应 Hover 和 MouseDown/MouseUp。
type LiquidButton struct {
	widget.BaseWidget

	Label       string
	Icon        fyne.Resource
	OnTapped    func()
	AccentColor *color.NRGBA

	hovered  bool
	pressed  bool
	disabled bool

	renderer *liquidButtonRenderer
}

// NewLiquidButton 创建液态玻璃按钮（无强调色）
func NewLiquidButton(label string, icon fyne.Resource, tapped func()) *LiquidButton {
	b := &LiquidButton{Label: label, Icon: icon, OnTapped: tapped}
	b.ExtendBaseWidget(b)
	return b
}

// NewLiquidButtonWithAccent 创建带强调色的液态玻璃按钮
func NewLiquidButtonWithAccent(label string, icon fyne.Resource, accent color.NRGBA, tapped func()) *LiquidButton {
	b := NewLiquidButton(label, icon, tapped)
	b.AccentColor = &accent
	return b
}

// Enable / Disable / Disabled

func (b *LiquidButton) Enable() {
	b.disabled = false
	b.Refresh()
}

func (b *LiquidButton) Disable() {
	b.disabled = true
	b.Refresh()
}

func (b *LiquidButton) Disabled() bool { return b.disabled }

// ── 事件响应 ─────────────────────────────────────────────────────────────────

// Tapped 鼠标/触控松开后触发
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

func (b *LiquidButton) TappedSecondary(_ *fyne.PointEvent) {}

// MouseDown 鼠标按下：立即更新为 pressed 状态，给用户即时视觉反馈
// 这比等 Tapped（mousedown+mouseup 都完成后）触发快一帧
func (b *LiquidButton) MouseDown(_ *desktop.MouseEvent) {
	if b.disabled {
		return
	}
	b.pressed = true
	b.Refresh()
}

// MouseUp 鼠标松开：回到 hovered 状态（Tapped 回调在此之后由框架触发）
func (b *LiquidButton) MouseUp(_ *desktop.MouseEvent) {
	if b.disabled {
		return
	}
	b.pressed = false
	b.Refresh()
}

// MouseIn 鼠标进入悬停区域
func (b *LiquidButton) MouseIn(_ *desktop.MouseEvent) {
	if b.disabled {
		return
	}
	b.hovered = true
	b.Refresh()
}

// MouseOut 鼠标离开：清除 hover 和 pressed 状态
func (b *LiquidButton) MouseOut() {
	b.hovered = false
	b.pressed = false
	b.Refresh()
}

func (b *LiquidButton) MouseMoved(_ *desktop.MouseEvent) {}

// 键盘支持
func (b *LiquidButton) FocusGained() {}
func (b *LiquidButton) FocusLost()   {}
func (b *LiquidButton) TypedRune(r rune) {
	if r == ' ' {
		b.Tapped(nil)
	}
}
func (b *LiquidButton) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter {
		b.Tapped(nil)
	}
}

func (b *LiquidButton) MinSize() fyne.Size {
	b.ExtendBaseWidget(b)
	return b.BaseWidget.MinSize()
}

// ── CreateRenderer ────────────────────────────────────────────────────────────

func (b *LiquidButton) CreateRenderer() fyne.WidgetRenderer {
	b.ExtendBaseWidget(b)

	// 外发光
	glow := canvas.NewRectangle(lgGlow)
	glow.CornerRadius = 18

	// 玻璃主体
	base := canvas.NewRectangle(lgBase)
	base.CornerRadius = 14
	base.StrokeColor = lgBorder
	base.StrokeWidth = 1.2

	// 顶部高光（两段模拟渐变）
	shimTop := canvas.NewRectangle(lgShimTop)
	shimTop.CornerRadius = 14
	shimBottom := canvas.NewRectangle(lgShimBottom)
	shimBottom.CornerRadius = 0

	// 图标
	var iconObj *canvas.Image
	if b.Icon != nil {
		iconObj = canvas.NewImageFromResource(b.Icon)
		iconObj.FillMode = canvas.ImageFillContain
		iconObj.SetMinSize(fyne.NewSize(18, 18))
	}

	// 文字
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
	const iconSize float32 = 18
	const gap float32 = 5

	// 外发光：比按钮多 glowPad*2，居中
	r.glow.Move(fyne.NewPos(-glowPad, -glowPad))
	r.glow.Resize(fyne.NewSize(size.Width+glowPad*2, size.Height+glowPad*2))

	// 玻璃主体铺满
	r.base.Move(fyne.NewPos(0, 0))
	r.base.Resize(size)

	// 顶部高光：高度约 42%，分两段（上圆角/下平）
	shimH := size.Height * 0.42
	r.shimTop.Move(fyne.NewPos(1, 1))
	r.shimTop.Resize(fyne.NewSize(size.Width-2, shimH*0.5))
	r.shimTop.CornerRadius = 14

	r.shimBottom.Move(fyne.NewPos(1, 1+shimH*0.5))
	r.shimBottom.Resize(fyne.NewSize(size.Width-2, shimH*0.5))

	// 图标 + 文字水平居中
	textW := r.measureTextWidth()
	hasIcon := r.iconObj != nil && r.btn.Icon != nil

	var contentW float32
	if hasIcon {
		contentW = iconSize + gap + textW
	} else {
		contentW = textW
	}

	startX := (size.Width - contentW) / 2
	if startX < 6 {
		startX = 6
	}
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

// measureTextWidth 用 fyne 主题字号估算文字宽度（比字符数*固定系数更准确）
func (r *liquidButtonRenderer) measureTextWidth() float32 {
	// fyne.MeasureText 是精确测量，但需要 fyne app 已初始化
	// 此处用主题 TextSize * 0.6 * 字符数作为保守估算
	// 对于中文字符（宽度约等于 TextSize），额外 +0.4 补偿
	var w float32
	for _, ch := range r.btn.Label {
		if ch > 0x7F {
			w += 14 * 1.0 // 中文/全角：约等于 1 字符宽
		} else {
			w += 14 * 0.62 // ASCII：约 0.62 字符宽
		}
	}
	return w
}

func (r *liquidButtonRenderer) MinSize() fyne.Size {
	const hPad float32 = 22
	const iconSize float32 = 18
	const gap float32 = 5

	textW := r.measureTextWidth()
	var w float32
	if r.iconObj != nil && r.btn.Icon != nil {
		w = iconSize + gap + textW + hPad*2
	} else {
		w = textW + hPad*2
	}
	if w < 88 {
		w = 88
	}
	return fyne.NewSize(w, 40)
}

func (r *liquidButtonRenderer) Refresh() {
	b := r.btn

	// ── 主体颜色（按状态）────────────────────────────────────────────────────
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

	// 混入强调色调（正常/hover/pressed 均混，禁用不混）
	if b.AccentColor != nil && !b.disabled {
		ac := *b.AccentColor
		mix := func(a, c uint8, t float32) uint8 {
			return uint8(float32(a)*(1-t) + float32(c)*t)
		}
		tint := float32(0.18)
		if b.pressed {
			tint = 0.28 // 按下时色调更浓，加深感
		}
		baseCol.R = mix(baseCol.R, ac.R, tint)
		baseCol.G = mix(baseCol.G, ac.G, tint)
		baseCol.B = mix(baseCol.B, ac.B, tint)

		// 发光 + 边框也染色
		r.glow.FillColor = color.NRGBA{R: ac.R, G: ac.G, B: ac.B, A: 0x28}
		r.base.StrokeColor = color.NRGBA{
			R: mix(0xFF, ac.R, 0.3),
			G: mix(0xFF, ac.G, 0.3),
			B: mix(0xFF, ac.B, 0.3),
			A: 0xBB,
		}
		// 按下时边框更亮（模拟内凹发光）
		if b.pressed {
			r.base.StrokeColor = color.NRGBA{
				R: mix(0xFF, ac.R, 0.5),
				G: mix(0xFF, ac.G, 0.5),
				B: mix(0xFF, ac.B, 0.5),
				A: 0xFF,
			}
			r.base.StrokeWidth = 1.8
		} else {
			r.base.StrokeWidth = 1.2
		}
	} else {
		r.glow.FillColor = lgGlow
		r.base.StrokeColor = lgBorder
		r.base.StrokeWidth = 1.2
	}

	r.base.FillColor = baseCol

	// 悬停时高光加强
	if b.hovered && !b.disabled && !b.pressed {
		r.shimTop.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x72}
	} else if b.pressed {
		// 按下时高光变暗（模拟压入感）
		r.shimTop.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x28}
	} else {
		r.shimTop.FillColor = lgShimTop
	}
	r.shimBottom.FillColor = lgShimBottom

	// 文字颜色
	if b.disabled {
		r.lbl.Color = lgTextDisabled
	} else {
		r.lbl.Color = lgText
	}

	// 图标透明度
	if r.iconObj != nil {
		if b.disabled {
			r.iconObj.Translucency = 0.6
		} else if b.pressed {
			r.iconObj.Translucency = 0.1
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

// ── 液态玻璃底部操作栏 ────────────────────────────────────────────────────────

// liquidButtonBar 构建深色底部操作栏（液态玻璃风格）
// start/stop/copy 三个 LiquidButton，右侧状态芯片显示当前运行状态
func liquidButtonBar(startBtn, stopBtn, copyBtn fyne.CanvasObject, statusText string) fyne.CanvasObject {
	// 底栏背景：深色半透明，与 Amber 背景形成强烈对比
	barBg := canvas.NewRectangle(color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0xEC})

	// 顶部高光线（液态玻璃上边缘反光）
	topGlow := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x88})
	topGlow.SetMinSize(fyne.NewSize(0, 2))

	// 状态芯片
	statusChip := liquidStatusChip("● " + statusText)

	// 按钮间分隔线
	sep := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x28})
	sep.SetMinSize(fyne.NewSize(1, 26))

	buttons := container.NewHBox(startBtn, stopBtn, sep, copyBtn)
	row := container.NewBorder(nil, nil, nil, statusChip, buttons)
	foreground := container.NewVBox(topGlow, container.NewPadded(row))

	return container.NewStack(barBg, foreground)
}

// liquidStatusChip 液态玻璃状态芯片
func liquidStatusChip(text string) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x18})
	bg.CornerRadius = 14
	bg.StrokeColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x55}
	bg.StrokeWidth = 1

	shim := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x28})
	shim.CornerRadius = 14

	lbl := canvas.NewText(text, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xCC})
	lbl.TextSize = 12

	return container.NewPadded(container.NewStack(bg, shim, container.NewPadded(lbl)))
}

// keep theme reference
var _ = theme.DefaultTheme()
