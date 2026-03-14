package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// materialYellowTheme 黄色背景 Material 风格主题
type materialYellowTheme struct{}

var _ fyne.Theme = (*materialYellowTheme)(nil)

// Material Yellow 调色板
var (
	// 背景色：温暖的 Amber 50
	mdBackground = color.NRGBA{R: 0xFF, G: 0xF8, B: 0xE1, A: 0xFF}
	// 卡片/surface 色：纯白，与背景形成层次
	mdSurface = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	// 主色：Material Amber 700
	mdPrimary = color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0xFF}
	// 前景/文字色：深灰，高对比
	mdForeground = color.NRGBA{R: 0x21, G: 0x21, B: 0x21, A: 0xFF}
	// 次要文字
	mdDisabled = color.NRGBA{R: 0x9E, G: 0x9E, B: 0x9E, A: 0xFF}
	// 危险色：Material Red 600
	mdDanger = color.NRGBA{R: 0xE5, G: 0x39, B: 0x35, A: 0xFF}
	// 成功色：Material Green 600
	mdSuccess = color.NRGBA{R: 0x43, G: 0xA0, B: 0x47, A: 0xFF}
	// 输入框焦点色
	mdFocus = color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x40}
	// 分隔线
	mdSeparator = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	// 阴影色（半透明黑）
	mdShadow = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x1A}
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
		return color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	case theme.ColorNameError:
		return mdDanger
	case theme.ColorNameFocus:
		return mdFocus
	case theme.ColorNameForeground:
		return mdForeground
	case theme.ColorNameHover:
		return color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x1F}
	case theme.ColorNameInputBackground:
		return mdSurface
	case theme.ColorNameInputBorder:
		return color.NRGBA{R: 0xBD, G: 0xBD, B: 0xBD, A: 0xFF}
	case theme.ColorNameMenuBackground:
		return mdSurface
	case theme.ColorNameOverlayBackground:
		return mdSurface
	case theme.ColorNamePlaceHolder:
		return mdDisabled
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x3D}
	case theme.ColorNamePrimary:
		return mdPrimary
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x7F}
	case theme.ColorNameSeparator:
		return mdSeparator
	case theme.ColorNameShadow:
		return mdShadow
	case theme.ColorNameSuccess:
		return mdSuccess
	case theme.ColorNameWarning:
		return color.NRGBA{R: 0xFF, G: 0x6F, B: 0x00, A: 0xFF}
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
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameInputBorder:
		return 2
	case theme.SizeNameScrollBar:
		return 6
	case theme.SizeNameScrollBarSmall:
		return 3
	}
	return theme.DefaultTheme().Size(name)
}
