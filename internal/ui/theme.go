package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// materialYellowTheme 黄色背景 Material 风格主题
//
// 调色板来自 Material Design 3 "Amber" 配色方案：
//   - 背景    Amber 50    #FFF8E1
//   - 主色    Amber 700   #FFB300
//   - 卡片    白色         #FFFFFF
//   - 前景    深灰         #212121
//   - 危险    Red 600      #E53935
//   - 成功    Green 600    #43A047
type materialYellowTheme struct{}

var _ fyne.Theme = (*materialYellowTheme)(nil)

// ── Material Yellow 调色板 ───────────────────────────────────────────────────
var (
	// 背景色：温暖的 Amber 50，不过曝
	mdBackground = color.NRGBA{R: 0xFF, G: 0xF8, B: 0xE1, A: 0xFF}
	// 卡片/surface 色：纯白，与背景形成层次
	mdSurface = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	// 主色：Material Amber 700
	mdPrimary = color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0xFF}
	// 前景/文字色：深灰，高对比
	mdForeground = color.NRGBA{R: 0x21, G: 0x21, B: 0x21, A: 0xFF}
	// 次要文字 / 禁用色
	mdDisabled = color.NRGBA{R: 0x9E, G: 0x9E, B: 0x9E, A: 0xFF}
	// 危险色：Material Red 600
	mdDanger = color.NRGBA{R: 0xE5, G: 0x39, B: 0x35, A: 0xFF}
	// 成功色：Material Green 600
	mdSuccess = color.NRGBA{R: 0x43, G: 0xA0, B: 0x47, A: 0xFF}
	// 输入框焦点色（Amber 半透明，柔和不刺眼）
	mdFocus = color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x50}
	// 分隔线
	mdSeparator = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	// 阴影色（半透明黑，用于卡片投影）
	mdShadow = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x20}
	// 选中高亮（Amber 15% 透明）
	mdSelection = color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x3D}
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
		// Amber 12% 透明，悬停时轻微高亮
		return color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x1F}
	case theme.ColorNameInputBackground:
		return mdSurface
	case theme.ColorNameInputBorder:
		// 输入框边框略深，比默认更清晰
		return color.NRGBA{R: 0xBD, G: 0xBD, B: 0xBD, A: 0xFF}
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
		// 滚动条：Amber 50% 透明，可见但不突兀
		return color.NRGBA{R: 0xFF, G: 0xB3, B: 0x00, A: 0x7F}
	case theme.ColorNameSeparator:
		return mdSeparator
	case theme.ColorNameShadow:
		return mdShadow
	case theme.ColorNameSuccess:
		return mdSuccess
	case theme.ColorNameWarning:
		return color.NRGBA{R: 0xFF, G: 0x6F, B: 0x00, A: 0xFF}
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
	// 外边距：卡片之间留 10px，比默认 4px 宽松
	case theme.SizeNamePadding:
		return 10
	// 内边距：卡片内容与边框 10px
	case theme.SizeNameInnerPadding:
		return 10
	// 正文字号 14px（Windows 默认 13px 略小）
	case theme.SizeNameText:
		return 14
	// 标题字号 20px
	case theme.SizeNameHeadingText:
		return 20
	// 副标题 16px
	case theme.SizeNameSubHeadingText:
		return 16
	// 小文字（hint、badge）12px
	case theme.SizeNameCaptionText:
		return 12
	// 输入框边框宽度 1.5px（稍粗，更清晰）
	case theme.SizeNameInputBorder:
		return 1.5
	// 输入框最小高度 38px（和 LiquidButton 对齐）
	case theme.SizeNameInputRadius:
		return 6
	// 滚动条宽度 7px（比默认 4px 更容易抓）
	case theme.SizeNameScrollBar:
		return 7
	// 悬停时细滚动条 3px
	case theme.SizeNameScrollBarSmall:
		return 3
	// 图标尺寸 20px（比默认 18px 略大，更易识别）
	case theme.SizeNameInlineIcon:
		return 20
	// 分割线 1px
	case theme.SizeNameSeparatorThickness:
		return 1
	}
	return theme.DefaultTheme().Size(name)
}
