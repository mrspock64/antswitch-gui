package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Dark palette (default) — modeled after the WavelogGate2/WSPR-Beacon look.
var (
	colorBackground = color.NRGBA{R: 0x0b, G: 0x0d, B: 0x12, A: 0xff}
	colorCard       = color.NRGBA{R: 0x15, G: 0x18, B: 0x21, A: 0xff}
	colorCardHover  = color.NRGBA{R: 0x1b, G: 0x1f, B: 0x2a, A: 0xff}
	colorForeground = color.NRGBA{R: 0xe8, G: 0xea, B: 0xee, A: 0xff}
	colorMuted      = color.NRGBA{R: 0x8a, G: 0x8f, B: 0x9c, A: 0xff}
	colorBorder     = color.NRGBA{R: 0x24, G: 0x28, B: 0x33, A: 0xff}
)

// Light palette.
var (
	colorBackgroundLight = color.NRGBA{R: 0xf5, G: 0xf6, B: 0xf8, A: 0xff}
	colorCardLight       = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorCardHoverLight  = color.NRGBA{R: 0xed, G: 0xef, B: 0xf3, A: 0xff}
	colorForegroundLight = color.NRGBA{R: 0x1a, G: 0x1c, B: 0x22, A: 0xff}
	colorMutedLight      = color.NRGBA{R: 0x6b, G: 0x70, B: 0x7c, A: 0xff}
	colorBorderLight     = color.NRGBA{R: 0xdd, G: 0xe0, B: 0xe6, A: 0xff}
)

// Shared across both variants.
var (
	colorAccent      = color.NRGBA{R: 0xf2, G: 0xa9, B: 0x3b, A: 0xff}
	colorAccentDim   = color.NRGBA{R: 0xf2, G: 0xa9, B: 0x3b, A: 0x33}
	colorOnAccent    = color.NRGBA{R: 0x18, G: 0x12, B: 0x08, A: 0xff}
	colorSuccess     = color.NRGBA{R: 0x2f, G: 0xb8, B: 0x72, A: 0xff}
	colorError       = color.NRGBA{R: 0xe0, G: 0x4a, B: 0x4a, A: 0xff}
)

// antSwitchTheme is a card-styled theme with an orange accent that can be
// switched between a dark and a light variant.
type antSwitchTheme struct {
	light bool
}

func newAntSwitchTheme(light bool) fyne.Theme {
	return antSwitchTheme{light: light}
}

func (t antSwitchTheme) palette() (bg, card, cardHover, fg, muted, border color.Color) {
	if t.light {
		return colorBackgroundLight, colorCardLight, colorCardHoverLight, colorForegroundLight, colorMutedLight, colorBorderLight
	}
	return colorBackground, colorCard, colorCardHover, colorForeground, colorMuted, colorBorder
}

func (t antSwitchTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	bg, card, cardHover, fg, muted, border := t.palette()

	switch name {
	case theme.ColorNameBackground:
		return bg
	case theme.ColorNameForeground:
		return fg
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameHyperlink:
		return colorAccent
	case theme.ColorNameForegroundOnPrimary:
		return colorOnAccent
	case theme.ColorNameButton, theme.ColorNameInputBackground:
		return card
	case theme.ColorNameDisabledButton:
		return card
	case theme.ColorNameDisabled:
		return muted
	case theme.ColorNamePlaceHolder:
		return muted
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return border
	case theme.ColorNameHover:
		return cardHover
	case theme.ColorNamePressed:
		return cardHover
	case theme.ColorNameSelection:
		return colorAccentDim
	case theme.ColorNameScrollBar, theme.ColorNameShadow, theme.ColorNameOverlayBackground:
		return bg
	case theme.ColorNameMenuBackground, theme.ColorNameHeaderBackground:
		return card
	case theme.ColorNameError:
		return colorError
	case theme.ColorNameSuccess:
		return colorSuccess
	}
	variant := theme.VariantDark
	if t.light {
		variant = theme.VariantLight
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (antSwitchTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (antSwitchTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (antSwitchTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameButtonRadius, theme.SizeNameInputRadius:
		return 10
	case theme.SizeNameCardRadius:
		return 12
	case theme.SizeNamePadding:
		return 8
	}
	return theme.DefaultTheme().Size(name)
}

// fg/muted/border/card/bg helpers used by widgets.go — these read the
// *current* app theme so canvas.Text/Rectangle objects (which don't get
// theme refreshes automatically) can be built with the right colors and
// then explicitly refreshed on theme change (see main.go applyTheme).
func currentPalette() (bg, card, cardHover, fg, muted, border color.Color) {
	t, _ := fyne.CurrentApp().Settings().Theme().(antSwitchTheme)
	return t.palette()
}
