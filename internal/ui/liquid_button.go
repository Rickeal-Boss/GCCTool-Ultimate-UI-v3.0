package ui

// liquid_button.go — iOS 26 液态玻璃风格按钮（精致升级版）
//
// 视觉层次（底→顶）：
//  1. glowOuter   — 外层柔和光晕（比按钮大 6px，极低透明度）
//  2. glowInner   — 内层紧贴光晕（比按钮大 2px，低透明度）
//  3. baseRect    — 玻璃主体（半透明白色 + 大圆角 16px + 彩色描边）
//  4. shimTop     — 顶部高光上段（白色半透明，模拟玻璃折射，顶角圆角）
//  5. shimBottom  — 顶部高光下段（渐隐过渡）
//  6. bottomLine  — 底部微高光线（玻璃底部反光边缘感）
//  7. icon + label — 图标 + 文字
//
// 状态机：normal → hovered → pressed → released (tapped) / disabled
//
// 交互优化：
//   - MouseDown 立即给 pressed 视觉反馈（不等 Tapped）
//   - 悬停时光晕颜色加深 + 高光增强
//   - 按下时主体加深 + 高光减弱（压入感）
//   - 强调色系统：每个按钮独立染色，区分语义

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ── 基础颜色常量 ─────────────────────────────────────────────────────────────

var (
	// 玻璃主体
	lgBase         = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x50} // 正常：约 31% 透明白
	lgBaseHover    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x6E} // 悬停：43% 透明白
	lgBasePressed  = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x34} // 按下：20% 透明白（压入感）
	lgBaseDisabled = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x22} // 禁用：13% 透明白

	// 边框
	lgBorder        = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xAA} // 正常边框
	lgBorderHover   = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xCC} // 悬停边框
	lgBorderPressed = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // 按下边框（最亮，凹陷发光感）

	// 顶部高光
	lgShimTop        = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x55} // 正常高光上段
	lgShimTopHover   = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x77} // 悬停高光增强
	lgShimTopPressed = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x22} // 按下高光减弱
	lgShimBottom     = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x0C} // 高光下段（淡出）

	// 底部反光线
	lgBottomLine = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x1A} // 底部微高光

	// 外发光
	lgGlowOuter = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x0E} // 外层光晕
	lgGlowInner = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x18} // 内层光晕

	// 文字
	lgText         = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0xFF} // 正常文字（Ink 近黑）
	lgTextDisabled = color.NRGBA{R: 0x9E, G: 0xA3, B: 0xB8, A: 0xFF} // 禁用文字（Slate 400）
)

// ── LiquidButton ─────────────────────────────────────────────────────────────

// LiquidButton 液态玻璃风格自定义 Widget
// 实现 desktop.Hoverable + desktop.Mouseable
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

// ── 状态控制 ─────────────────────────────────────────────────────────────────

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

// Tapped 鼠标松开后触发回调
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

// MouseDown 立即更新 pressed 状态，给用户即时视觉反馈（比等 Tapped 快一帧）
func (b *LiquidButton) MouseDown(_ *desktop.MouseEvent) {
	if b.disabled {
		return
	}
	b.pressed = true
	b.Refresh()
}

// MouseUp 回到 hovered 状态
func (b *LiquidButton) MouseUp(_ *desktop.MouseEvent) {
	if b.disabled {
		return
	}
	b.pressed = false
	b.Refresh()
}

func (b *LiquidButton) MouseIn(_ *desktop.MouseEvent) {
	if b.disabled {
		return
	}
	b.hovered = true
	b.Refresh()
}

// MouseOut 清除 hover 和 pressed 状态
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

	// 外层光晕（最底层，比按钮宽 6px）
	glowOuter := canvas.NewRectangle(lgGlowOuter)
	glowOuter.CornerRadius = 22

	// 内层光晕（比按钮宽 2px）
	glowInner := canvas.NewRectangle(lgGlowInner)
	glowInner.CornerRadius = 18

	// 玻璃主体
	base := canvas.NewRectangle(lgBase)
	base.CornerRadius = 16
	base.StrokeColor = lgBorder
	base.StrokeWidth = 1.2

	// 顶部高光上段（圆角）
	shimTop := canvas.NewRectangle(lgShimTop)
	shimTop.CornerRadius = 16

	// 顶部高光下段（无圆角，平滑过渡）
	shimBottom := canvas.NewRectangle(lgShimBottom)
	shimBottom.CornerRadius = 0

	// 底部反光线
	bottomLine := canvas.NewRectangle(lgBottomLine)
	bottomLine.CornerRadius = 16

	// 图标
	var iconObj *canvas.Image
	if b.Icon != nil {
		iconObj = canvas.NewImageFromResource(b.Icon)
		iconObj.FillMode = canvas.ImageFillContain
		iconObj.SetMinSize(fyne.NewSize(18, 18))
	}

	// 文字（加粗，14px）
	lbl := canvas.NewText(b.Label, lgText)
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	lbl.TextSize = 14
	lbl.Alignment = fyne.TextAlignCenter

	r := &liquidButtonRenderer{
		btn:        b,
		glowOuter:  glowOuter,
		glowInner:  glowInner,
		base:       base,
		shimTop:    shimTop,
		shimBottom: shimBottom,
		bottomLine: bottomLine,
		iconObj:    iconObj,
		lbl:        lbl,
	}
	b.renderer = r
	return r
}

// ── liquidButtonRenderer ─────────────────────────────────────────────────────

type liquidButtonRenderer struct {
	btn        *LiquidButton
	glowOuter  *canvas.Rectangle
	glowInner  *canvas.Rectangle
	base       *canvas.Rectangle
	shimTop    *canvas.Rectangle
	shimBottom *canvas.Rectangle
	bottomLine *canvas.Rectangle
	iconObj    *canvas.Image
	lbl        *canvas.Text
}

func (r *liquidButtonRenderer) Layout(size fyne.Size) {
	const outerGlowPad float32 = 6
	const innerGlowPad float32 = 2
	const iconSize float32 = 18
	const gap float32 = 6

	// 外层光晕：超出按钮边界
	r.glowOuter.Move(fyne.NewPos(-outerGlowPad, -outerGlowPad))
	r.glowOuter.Resize(fyne.NewSize(size.Width+outerGlowPad*2, size.Height+outerGlowPad*2))

	// 内层光晕：略超出
	r.glowInner.Move(fyne.NewPos(-innerGlowPad, -innerGlowPad))
	r.glowInner.Resize(fyne.NewSize(size.Width+innerGlowPad*2, size.Height+innerGlowPad*2))

	// 玻璃主体铺满
	r.base.Move(fyne.NewPos(0, 0))
	r.base.Resize(size)

	// 顶部高光：高度约 40%，分两段
	shimH := size.Height * 0.40
	r.shimTop.Move(fyne.NewPos(1, 1))
	r.shimTop.Resize(fyne.NewSize(size.Width-2, shimH*0.52))
	r.shimTop.CornerRadius = 16

	r.shimBottom.Move(fyne.NewPos(1, 1+shimH*0.52))
	r.shimBottom.Resize(fyne.NewSize(size.Width-2, shimH*0.48))

	// 底部反光线（最后 2px）
	r.bottomLine.Move(fyne.NewPos(1, size.Height-3))
	r.bottomLine.Resize(fyne.NewSize(size.Width-2, 2))

	// 图标 + 文字水平居中布局
	textW := r.measureTextWidth()
	hasIcon := r.iconObj != nil && r.btn.Icon != nil

	var contentW float32
	if hasIcon {
		contentW = iconSize + gap + textW
	} else {
		contentW = textW
	}

	startX := (size.Width - contentW) / 2
	if startX < 8 {
		startX = 8
	}
	centerY := (size.Height - iconSize) / 2
	textY := (size.Height - 14) / 2

	if hasIcon {
		r.iconObj.Move(fyne.NewPos(startX, centerY))
		r.iconObj.Resize(fyne.NewSize(iconSize, iconSize))
		r.lbl.Move(fyne.NewPos(startX+iconSize+gap, textY))
	} else {
		r.lbl.Move(fyne.NewPos(startX, textY))
	}
	r.lbl.Resize(fyne.NewSize(contentW, 20))
}

// measureTextWidth 估算文字宽度（中文/全角 ≈ 14px，ASCII ≈ 8.7px）
func (r *liquidButtonRenderer) measureTextWidth() float32 {
	var w float32
	for _, ch := range r.btn.Label {
		if ch > 0x7F {
			w += 14 * 1.0 // 中文/全角
		} else {
			w += 14 * 0.62 // ASCII
		}
	}
	return w
}

func (r *liquidButtonRenderer) MinSize() fyne.Size {
	const hPad float32 = 24
	const iconSize float32 = 18
	const gap float32 = 6

	textW := r.measureTextWidth()
	var w float32
	if r.iconObj != nil && r.btn.Icon != nil {
		w = iconSize + gap + textW + hPad*2
	} else {
		w = textW + hPad*2
	}
	if w < 96 {
		w = 96
	}
	return fyne.NewSize(w, 42)
}

func (r *liquidButtonRenderer) Refresh() {
	b := r.btn

	// ── 主体颜色（按状态）────────────────────────────────────────────────
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

	// ── 强调色混入 ────────────────────────────────────────────────────────
	if b.AccentColor != nil && !b.disabled {
		ac := *b.AccentColor
		tint := float32(0.20)
		if b.pressed {
			tint = 0.32 // 按下时色调更浓
		} else if b.hovered {
			tint = 0.25 // 悬停时稍深
		}

		mixF32 := func(a, c uint8, t float32) uint8 {
			return uint8(float32(a)*(1-t) + float32(c)*t)
		}

		baseCol.R = mixF32(baseCol.R, ac.R, tint)
		baseCol.G = mixF32(baseCol.G, ac.G, tint)
		baseCol.B = mixF32(baseCol.B, ac.B, tint)

		// 光晕颜色随强调色染色
		glowAlpha := uint8(0x14)
		if b.hovered {
			glowAlpha = 0x22
		}
		if b.pressed {
			glowAlpha = 0x2A
		}
		r.glowOuter.FillColor = color.NRGBA{R: ac.R, G: ac.G, B: ac.B, A: glowAlpha / 2}
		r.glowInner.FillColor = color.NRGBA{R: ac.R, G: ac.G, B: ac.B, A: glowAlpha}

		// 边框颜色随状态变化
		borderAlpha := uint8(0xBB)
		borderTint := float32(0.28)
		borderWidth := float32(1.2)
		if b.pressed {
			borderAlpha = 0xFF
			borderTint = 0.55
			borderWidth = 1.8
		} else if b.hovered {
			borderAlpha = 0xDD
			borderTint = 0.40
			borderWidth = 1.5
		}
		r.base.StrokeColor = color.NRGBA{
			R: mixF32(0xFF, ac.R, borderTint),
			G: mixF32(0xFF, ac.G, borderTint),
			B: mixF32(0xFF, ac.B, borderTint),
			A: borderAlpha,
		}
		r.base.StrokeWidth = borderWidth
	} else {
		// 无强调色：使用默认光晕和边框
		if b.hovered && !b.disabled {
			r.glowOuter.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x18}
			r.glowInner.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x28}
			r.base.StrokeColor = lgBorderHover
			r.base.StrokeWidth = 1.5
		} else {
			r.glowOuter.FillColor = lgGlowOuter
			r.glowInner.FillColor = lgGlowInner
			r.base.StrokeColor = lgBorder
			r.base.StrokeWidth = 1.2
		}
	}

	r.base.FillColor = baseCol

	// ── 顶部高光（按状态）────────────────────────────────────────────────
	switch {
	case b.disabled:
		r.shimTop.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x18}
	case b.pressed:
		r.shimTop.FillColor = lgShimTopPressed
	case b.hovered:
		r.shimTop.FillColor = lgShimTopHover
	default:
		r.shimTop.FillColor = lgShimTop
	}
	r.shimBottom.FillColor = lgShimBottom

	// ── 底部反光线 ────────────────────────────────────────────────────────
	if b.disabled {
		r.bottomLine.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x08}
	} else {
		r.bottomLine.FillColor = lgBottomLine
	}

	// ── 文字颜色 ──────────────────────────────────────────────────────────
	if b.disabled {
		r.lbl.Color = lgTextDisabled
	} else {
		r.lbl.Color = lgText
	}

	// ── 图标透明度 ────────────────────────────────────────────────────────
	if r.iconObj != nil {
		switch {
		case b.disabled:
			r.iconObj.Translucency = 0.55
		case b.pressed:
			r.iconObj.Translucency = 0.05
		default:
			r.iconObj.Translucency = 0
		}
		r.iconObj.Refresh()
	}

	r.lbl.Refresh()
	r.base.Refresh()
	r.shimTop.Refresh()
	r.shimBottom.Refresh()
	r.bottomLine.Refresh()
	r.glowInner.Refresh()
	r.glowOuter.Refresh()
}

func (r *liquidButtonRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.glowOuter, r.glowInner, r.base, r.shimTop, r.shimBottom, r.bottomLine}
	if r.iconObj != nil && r.btn.Icon != nil {
		objs = append(objs, r.iconObj)
	}
	objs = append(objs, r.lbl)
	return objs
}

func (r *liquidButtonRenderer) Destroy() {}

// ── 液态玻璃底部操作栏（buildDynamicButtonBar 使用的静态版本）──────────────

// liquidButtonBar 构建深色底部操作栏（液态玻璃风格）
// start/stop/copy 三个 LiquidButton + 右侧动态状态芯片
func liquidButtonBar(startBtn, stopBtn, copyBtn fyne.CanvasObject, statusText string) fyne.CanvasObject {
	// 底栏背景：深色半透明，与暖黄背景形成强烈对比
	barBg := canvas.NewRectangle(color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0xEE})

	// 顶部高光线（Amber 色，液态玻璃上边缘反光）
	topGlow := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x99})
	topGlow.SetMinSize(fyne.NewSize(0, 2))

	// 顶部次级高光（白色，更柔和）
	topGlow2 := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x22})
	topGlow2.SetMinSize(fyne.NewSize(0, 1))

	// 状态芯片
	statusChip := liquidStatusChip("● " + statusText)

	// 按钮间分隔线
	sep := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x28})
	sep.SetMinSize(fyne.NewSize(1, 28))

	buttons := container.NewHBox(startBtn, stopBtn, sep, copyBtn)
	row := container.NewBorder(nil, nil, nil, statusChip, buttons)
	foreground := container.NewVBox(
		container.NewStack(
			canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x00}),
			topGlow,
		),
		topGlow2,
		container.NewPadded(row),
	)

	return container.NewStack(barBg, foreground)
}

// liquidStatusChip 液态玻璃状态芯片
func liquidStatusChip(text string) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x16})
	bg.CornerRadius = 16
	bg.StrokeColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x50}
	bg.StrokeWidth = 1

	// 内部高光（顶部半圆，模拟玻璃上边缘）
	shim := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x22})
	shim.CornerRadius = 16

	lbl := canvas.NewText(text, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xCC})
	lbl.TextSize = 12

	return container.NewPadded(container.NewStack(bg, shim, container.NewPadded(lbl)))
}

// keep theme reference
var _ = theme.DefaultTheme()
