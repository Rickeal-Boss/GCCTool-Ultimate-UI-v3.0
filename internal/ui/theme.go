package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// materialYellowTheme — Material Design 3 "Amber Gold" 桌面主题
//
// 设计语言：
//   - 背景      Amber 50 暖白  #FFF8E1  — 温暖不刺眼，呼吸感强
//   - 主色      Amber 700      #FFB300  — 高饱和强调色，用于交互焦点
//   - 深主色    Amber 800      #FF8F00  — 渐变底色、边框强调
//   - 卡片      纯白 + 微投影   #FFFFFF  — 与背景形成清晰层次
//   - 文字      近黑 Ink       #1A1A2E  — 高对比，阅读舒适
//   - 次要文字  Slate 400      #94A3B8  — 提示/占位/禁用
//   - 成功      Emerald 600    #059669
//   - 危险      Rose 600       #E53935
//   - 信息      Indigo 500     #6366F1
type materialYellowTheme struct{}

var _ fyne.Theme = (*materialYellowTheme)(nil)

// ── 设计令牌（Design Tokens）────────────────────────────────────────────────
var (
	// ── Surface & Background
	mdBackground = color.NRGBA{R: 0xFF, G: 0xF8, B: 0xE1, A: 0xFF} // Amber 50
	mdSurface    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // 纯白卡片

	// ── Brand / Amber
	mdPrimary     = color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0xFF} // Amber 700
	mdPrimaryDark = color.NRGBA{R: 0xFF, G: 0x8F, B: 0x00, A: 0xFF} // Amber 800（深色变体）
	mdPrimaryDim  = color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x28} // 15% 透明，用于 hover/selection

	// ── Foreground / Typography
	mdForeground    = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0xFF} // Ink 近黑
	mdForegroundSub = color.NRGBA{R: 0x4A, G: 0x5A, B: 0x7A, A: 0xFF} // Slate 600，次要正文
	mdDisabled      = color.NRGBA{R: 0x9E, G: 0xA3, B: 0xB8, A: 0xFF} // Slate 400

	// ── Semantic Colors
	mdSuccess = color.NRGBA{R: 0x05, G: 0x96, B: 0x69, A: 0xFF} // Emerald 600
	mdDanger  = color.NRGBA{R: 0xE5, G: 0x39, B: 0x35, A: 0xFF} // Rose 600
	mdWarning = color.NRGBA{R: 0xFF, G: 0x6F, B: 0x00, A: 0xFF} // Deep Orange 800
	mdInfo    = color.NRGBA{R: 0x63, G: 0x66, B: 0xF1, A: 0xFF} // Indigo 500

	// ── State Colors
	mdFocus     = color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x55} // Amber focus ring
	mdSelection = color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x3A} // 23% 选中高亮

	// ── Structural
	mdSeparator = color.NRGBA{R: 0xE8, G: 0xEC, B: 0xF0, A: 0xFF} // 轻量分割线
	mdShadow    = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x2E, A: 0x1A} // 10% ink 阴影

	// ── Input
	mdInputBg     = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	mdInputBorder = color.NRGBA{R: 0xC8, G: 0xCC, B: 0xD8, A: 0xFF} // 冷灰边框
)

func (m *materialYellowTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return mdBackground
	case theme.ColorNameButton:
		return mdPrimary
	case theme.ColorNameDisabled:
		return mdDisabled
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 0xDC, G: 0xE0, B: 0xE8, A: 0xFF}
	case theme.ColorNameError:
		return mdDanger
	case theme.ColorNameFocus:
		return mdFocus
	case theme.ColorNameForeground:
		return mdForeground
	case theme.ColorNameHover:
		// Amber 12%：悬停轻微高亮，不过于突出
		return color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x1F}
	case theme.ColorNameInputBackground:
		return mdInputBg
	case theme.ColorNameInputBorder:
		return mdInputBorder
	case theme.ColorNameMenuBackground:
		return mdSurface
	case theme.ColorNameOverlayBackground:
		return mdSurface
	case theme.ColorNamePlaceHolder:
		return mdDisabled
	case theme.ColorNamePressed:
		return mdSelection
	case theme.ColorNamePrimary:
		return mdPrimary
	case theme.ColorNameScrollBar:
		// Amber 40%：可见但柔和
		return color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x66}
	case theme.ColorNameSeparator:
		return mdSeparator
	case theme.ColorNameShadow:
		return mdShadow
	case theme.ColorNameSuccess:
		return mdSuccess
	case theme.ColorNameWarning:
		return mdWarning
	case theme.ColorNameSelection:
		return mdSelection
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (m *materialYellowTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m *materialYellowTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *materialYellowTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	// 外边距：卡片间距 12px（比默认 4px 宽松，信息密度更佳）
	case theme.SizeNamePadding:
		return 12
	// 内边距：卡片内容留白 10px
	case theme.SizeNameInnerPadding:
		return 10
	// 正文字号：15px（Windows 默认 13px 略小，视觉更舒适）
	case theme.SizeNameText:
		return 15
	// 标题字号：22px
	case theme.SizeNameHeadingText:
		return 22
	// 副标题：17px
	case theme.SizeNameSubHeadingText:
		return 17
	// 小文字（hint、badge）：12px
	case theme.SizeNameCaptionText:
		return 12
	// 输入框边框：1.5px（稍粗，更清晰）
	case theme.SizeNameInputBorder:
		return 1.5
	// 输入框圆角：7px（比卡片略小，层次感）
	case theme.SizeNameInputRadius:
		return 7
	// 滚动条宽度：8px（更易抓握）
	case theme.SizeNameScrollBar:
		return 8
	// 悬停细滚动条：4px
	case theme.SizeNameScrollBarSmall:
		return 4
	// 图标尺寸：22px（比默认 18px 大，易识别）
	case theme.SizeNameInlineIcon:
		return 22
	// 分割线：1px（纤细，不喧宾夺主）
	case theme.SizeNameSeparatorThickness:
		return 1
	}
	return theme.DefaultTheme().Size(name)
}
