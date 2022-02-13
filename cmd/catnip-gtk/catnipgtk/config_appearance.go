package catnipgtk

import (
	"fmt"

	"github.com/diamondburned/catnip-gtk"
	"github.com/diamondburned/gotk4-handy/pkg/handy"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v3"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
)

type Appearance struct {
	LineCap LineCap

	ForegroundColor OptionalColor
	BackgroundColor OptionalColor

	BarWidth     float64
	SpaceWidth   float64 // gap width
	MinimumClamp float64
	AntiAlias    AntiAlias

	DrawStyle catnip.DrawStyle

	CustomCSS string
}

func symmetryString(s catnip.DrawStyle) string {
	switch s {
	case catnip.DrawVerticalBars:
		return "Vertical Bars"
	case catnip.DrawHorizontalBars:
		return "Horizontal Bars"
	case catnip.DrawLines:
		return "Lines"
	default:
		return ""
	}
}

func NewAppearance() Appearance {
	return Appearance{
		LineCap:      CapButt,
		BarWidth:     4,
		SpaceWidth:   1,
		MinimumClamp: 1,
		AntiAlias:    AntiAliasGood,
	}
}

func (ac *Appearance) Page(apply func()) *handy.PreferencesPage {
	lineCapCombo := gtk.NewComboBoxText()
	lineCapCombo.SetVAlign(gtk.AlignCenter)
	addCombo(lineCapCombo, CapButt, CapRound)
	lineCapCombo.SetActiveID(string(ac.LineCap))
	lineCapCombo.Show()
	lineCapCombo.Connect("changed", func(lineCapCombo *gtk.ComboBoxText) {
		ac.LineCap = LineCap(lineCapCombo.ActiveID())
		apply()
	})

	lineCapRow := handy.NewActionRow()
	lineCapRow.Add(lineCapCombo)
	lineCapRow.SetActivatableWidget(lineCapCombo)
	lineCapRow.SetTitle("Bar Cap")
	lineCapRow.SetSubtitle("Whether to draw the bars squared or rounded.")
	lineCapRow.Show()

	barSpin := gtk.NewSpinButtonWithRange(1, 100, 1)
	barSpin.SetVAlign(gtk.AlignCenter)
	barSpin.SetDigits(1)
	barSpin.SetValue(ac.BarWidth)
	barSpin.Show()
	barSpin.Connect("value-changed", func(barSpin *gtk.SpinButton) {
		ac.BarWidth = barSpin.Value()
		apply()
	})

	barRow := handy.NewActionRow()
	barRow.Add(barSpin)
	barRow.SetActivatableWidget(barSpin)
	barRow.SetTitle("Bar/Line Width")
	barRow.SetSubtitle("The thickness of the bar or line in arbitrary unit.")
	barRow.Show()

	spaceSpin := gtk.NewSpinButtonWithRange(0, 100, 1)
	spaceSpin.SetVAlign(gtk.AlignCenter)
	spaceSpin.SetDigits(3)
	spaceSpin.SetValue(ac.SpaceWidth)
	spaceSpin.Show()
	spaceSpin.Connect("value-changed", func(spaceSpin *gtk.SpinButton) {
		ac.SpaceWidth = spaceSpin.Value()
		apply()
	})

	spaceRow := handy.NewActionRow()
	spaceRow.Add(spaceSpin)
	spaceRow.SetActivatableWidget(spaceSpin)
	spaceRow.SetTitle("Gap Width")
	spaceRow.SetSubtitle("The width of the gaps between bars in arbitrary unit.")
	spaceRow.Show()

	clampSpin := gtk.NewSpinButtonWithRange(0, 25, 1)
	clampSpin.SetVAlign(gtk.AlignCenter)
	clampSpin.SetValue(ac.MinimumClamp)
	clampSpin.Show()
	clampSpin.Connect("value-changed", func(clampSpin *gtk.SpinButton) {
		ac.MinimumClamp = clampSpin.Value()
		apply()
	})

	clampRow := handy.NewActionRow()
	clampRow.Add(clampSpin)
	clampRow.SetActivatableWidget(clampSpin)
	clampRow.SetTitle("Clamp Height")
	clampRow.SetSubtitle("The value at which the bar or line should be clamped to 0.")
	clampRow.Show()

	aaCombo := gtk.NewComboBoxText()
	aaCombo.SetVAlign(gtk.AlignCenter)
	addCombo(
		aaCombo,
		AntiAliasNone,
		AntiAliasGrey,
		AntiAliasSubpixel,
		AntiAliasFast,
		AntiAliasGood,
		AntiAliasBest,
	)
	aaCombo.SetActiveID(string(ac.AntiAlias))
	aaCombo.Show()
	aaCombo.Connect("changed", func(aaCombo *gtk.ComboBoxText) {
		ac.AntiAlias = AntiAlias(aaCombo.ActiveID())
		apply()
	})

	aaRow := handy.NewActionRow()
	aaRow.Add(aaCombo)
	aaRow.SetActivatableWidget(aaCombo)
	aaRow.SetTitle("Anti-Aliasing")
	aaRow.SetSubtitle("The anti-alias mode to draw with.")
	aaRow.Show()

	styleCombo := gtk.NewComboBoxText()
	styleCombo.SetVAlign(gtk.AlignCenter)
	styleCombo.AppendText(symmetryString(catnip.DrawVerticalBars))
	styleCombo.AppendText(symmetryString(catnip.DrawHorizontalBars))
	styleCombo.AppendText(symmetryString(catnip.DrawLines))
	styleCombo.SetActive(int(ac.DrawStyle))
	styleCombo.Show()
	styleCombo.Connect("changed", func(symmCombo *gtk.ComboBoxText) {
		ac.DrawStyle = catnip.DrawStyle(symmCombo.Active())
		apply()
	})

	styleRow := handy.NewActionRow()
	styleRow.Add(styleCombo)
	styleRow.SetActivatableWidget(styleCombo)
	styleRow.SetTitle("DrawStyle")
	styleRow.SetSubtitle("Whether to mirror bars vertically or horizontally.")
	styleRow.Show()

	barGroup := handy.NewPreferencesGroup()
	barGroup.SetTitle("Bars")
	barGroup.Add(lineCapRow)
	barGroup.Add(barRow)
	barGroup.Add(spaceRow)
	barGroup.Add(clampRow)
	barGroup.Add(aaRow)
	barGroup.Add(styleRow)
	barGroup.Show()

	fgRow := newColorRow(&ac.ForegroundColor, true, apply)
	fgRow.SetTitle("Foreground Color")
	fgRow.SetSubtitle("The color of the visualizer bars.")
	fgRow.Show()

	bgRow := newColorRow(&ac.BackgroundColor, false, apply)
	bgRow.SetTitle("Background Color")
	bgRow.SetSubtitle("The color of the background window.")
	bgRow.Show()

	colorGroup := handy.NewPreferencesGroup()
	colorGroup.SetTitle("Colors")
	colorGroup.Add(fgRow)
	colorGroup.Add(bgRow)
	colorGroup.Show()

	cssText := gtk.NewTextView()
	cssText.SetBorderWidth(5)
	cssText.SetMonospace(true)
	cssText.SetAcceptsTab(true)
	cssText.Show()

	cssBuf := cssText.Buffer()
	cssBuf.SetText(ac.CustomCSS)
	cssBuf.Connect("changed", func(cssBuf *gtk.TextBuffer) {
		start, end := cssBuf.Bounds()
		ac.CustomCSS = cssBuf.Text(start, end, false)
		apply()
	})

	textScroll := gtk.NewScrolledWindow(nil, nil)
	textScroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyNever)
	textScroll.SetSizeRequest(-1, 300)
	textScroll.SetVExpand(true)
	textScroll.Add(cssText)
	textScroll.Show()

	cssGroup := handy.NewPreferencesGroup()
	cssGroup.SetTitle("Custom CSS")
	cssGroup.Add(textScroll)
	cssGroup.Show()

	page := handy.NewPreferencesPage()
	page.SetTitle("Appearance")
	page.SetIconName("applications-graphics-symbolic")
	page.Add(barGroup)
	page.Add(colorGroup)
	page.Add(cssGroup)

	return page
}

func addCombo(c *gtk.ComboBoxText, vs ...interface{}) {
	for _, v := range vs {
		s := fmt.Sprint(v)
		c.Append(s, s)
	}
}

func newColorRow(optc *OptionalColor, fg bool, apply func()) *handy.ActionRow {
	color := gtk.NewColorButton()
	color.SetVAlign(gtk.AlignCenter)
	color.SetUseAlpha(true)
	color.Show()
	color.Connect("color-set", func(interface{}) { // hack around lack of marshaler
		cacc := catnip.ColorFromGDK(color.RGBA())
		*optc = &cacc
		apply()
	})

	var defaultRGBA *gdk.RGBA
	if fg {
		style := color.StyleContext()
		defaultRGBA = style.Color(gtk.StateFlagNormal)
	} else {
		rgba := gdk.NewRGBA(0, 0, 0, 0)
		defaultRGBA = &rgba
	}

	var rgba *gdk.RGBA
	if colorValue := *optc; colorValue != nil {
		cacc := *colorValue
		value := gdk.NewRGBA(cacc[0], cacc[1], cacc[2], cacc[3])
		rgba = &value
	}

	if rgba != nil {
		color.SetRGBA(rgba)
	} else {
		color.SetRGBA(defaultRGBA)
	}

	reset := gtk.NewButtonFromIconName("edit-undo-symbolic", int(gtk.IconSizeButton))
	reset.SetRelief(gtk.ReliefNone)
	reset.SetVAlign(gtk.AlignCenter)
	reset.SetTooltipText("Revert")
	reset.Show()
	reset.Connect("destroy", func(reset *gtk.Button) { color.Destroy() }) // prevent leak
	reset.Connect("clicked", func(reset *gtk.Button) {
		*optc = nil
		color.SetRGBA(defaultRGBA)
		apply()
	})

	row := handy.NewActionRow()
	row.AddPrefix(reset)
	row.Add(color)
	row.SetActivatableWidget(color)

	return row
}

type OptionalColor = *catnip.CairoColor

type LineCap string

const (
	CapButt  LineCap = "Butt"
	CapRound LineCap = "Round"
)

func (lc LineCap) AsLineCap() cairo.LineCap {
	switch lc {
	case CapButt:
		return cairo.LINE_CAP_BUTT
	case CapRound:
		return cairo.LINE_CAP_ROUND
	default:
		return CapButt.AsLineCap()
	}
}

type AntiAlias string

const (
	AntiAliasNone     AntiAlias = "None"
	AntiAliasGrey     AntiAlias = "Grey"
	AntiAliasSubpixel AntiAlias = "Subpixel"
	AntiAliasFast     AntiAlias = "Fast"
	AntiAliasGood     AntiAlias = "Good"
	AntiAliasBest     AntiAlias = "Best"
)

func (aa AntiAlias) AsAntialias() cairo.Antialias {
	switch aa {
	case AntiAliasNone:
		return cairo.ANTIALIAS_NONE
	case AntiAliasGrey:
		return cairo.ANTIALIAS_GRAY
	case AntiAliasSubpixel:
		return cairo.ANTIALIAS_SUBPIXEL
	case AntiAliasFast:
		return cairo.ANTIALIAS_FAST
	case AntiAliasGood:
		return cairo.ANTIALIAS_GOOD
	case AntiAliasBest:
		return cairo.ANTIALIAS_BEST
	default:
		return cairo.ANTIALIAS_GOOD
	}
}
