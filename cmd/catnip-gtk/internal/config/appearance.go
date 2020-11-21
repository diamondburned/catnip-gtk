package config

import (
	"github.com/diamondburned/catnip-gtk"
	"github.com/diamondburned/handy"
	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

type Appearance struct {
	LineCap LineCap

	ForegroundColor OptionalColor
	BackgroundColor OptionalColor

	BarWidth     float64
	SpaceWidth   float64 // gap width
	MinimumClamp float64
	DualChannel  bool // .Monophonic
	Symmetry     catnip.Symmetry

	CustomCSS string
}

func symmetryString(s catnip.Symmetry) string {
	switch s {
	case catnip.Vertical:
		return "Vertical"
	case catnip.Horizontal:
		return "Horizontal"
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
		DualChannel:  true,
	}
}

func (ac *Appearance) Page(apply func()) *handy.PreferencesPage {
	lineCapCombo, _ := gtk.ComboBoxTextNew()
	lineCapCombo.SetVAlign(gtk.ALIGN_CENTER)
	lineCapCombo.Append(string(CapButt), string(CapButt))
	lineCapCombo.Append(string(CapRound), string(CapRound))
	lineCapCombo.SetActiveID(string(ac.LineCap))
	lineCapCombo.Show()
	lineCapCombo.Connect("changed", func() {
		ac.LineCap = LineCap(lineCapCombo.GetActiveID())
		apply()
	})

	lineCapRow := handy.ActionRowNew()
	lineCapRow.Add(lineCapCombo)
	lineCapRow.SetActivatableWidget(lineCapCombo)
	lineCapRow.SetTitle("Line Cap")
	lineCapRow.SetSubtitle("Whether to draw the bars squared or rounded.")
	lineCapRow.Show()

	barSpin, _ := gtk.SpinButtonNewWithRange(1, 100, 1)
	barSpin.SetVAlign(gtk.ALIGN_CENTER)
	barSpin.SetValue(ac.BarWidth)
	barSpin.Show()
	barSpin.Connect("value-changed", func() {
		ac.BarWidth = barSpin.GetValue()
		apply()
	})

	barRow := handy.ActionRowNew()
	barRow.Add(barSpin)
	barRow.SetActivatableWidget(barSpin)
	barRow.SetTitle("Bar Width")
	barRow.SetSubtitle("The thickness of the bar in arbitrary unit.")
	barRow.Show()

	spaceSpin, _ := gtk.SpinButtonNewWithRange(1, 100, 1)
	spaceSpin.SetVAlign(gtk.ALIGN_CENTER)
	spaceSpin.SetValue(ac.SpaceWidth)
	spaceSpin.Show()
	spaceSpin.Connect("value-changed", func() {
		ac.SpaceWidth = spaceSpin.GetValue()
		apply()
	})

	spaceRow := handy.ActionRowNew()
	spaceRow.Add(spaceSpin)
	spaceRow.SetActivatableWidget(spaceSpin)
	spaceRow.SetTitle("Gap Width")
	spaceRow.SetSubtitle("The width of the gaps between bars in arbitrary unit.")
	spaceRow.Show()

	clampSpin, _ := gtk.SpinButtonNewWithRange(0, 25, 1)
	clampSpin.SetVAlign(gtk.ALIGN_CENTER)
	clampSpin.SetValue(ac.MinimumClamp)
	clampSpin.Show()
	clampSpin.Connect("value-changed", func() {
		ac.MinimumClamp = clampSpin.GetValue()
		apply()
	})

	clampRow := handy.ActionRowNew()
	clampRow.Add(clampSpin)
	clampRow.SetActivatableWidget(clampSpin)
	clampRow.SetTitle("Clamp Height")
	clampRow.SetSubtitle("The height at which the bar should be clamped to 0.")
	clampRow.Show()

	dualCh, _ := gtk.SwitchNew()
	dualCh.SetVAlign(gtk.ALIGN_CENTER)
	dualCh.SetActive(ac.DualChannel)
	dualCh.Show()
	dualCh.Connect("state-set", func(_ *gtk.Switch, state bool) {
		ac.DualChannel = state
		apply()
	})

	dualChRow := handy.ActionRowNew()
	dualChRow.Add(dualCh)
	dualChRow.SetActivatableWidget(dualCh)
	dualChRow.SetTitle("Dual Channels")
	dualChRow.SetSubtitle("If enabled, will draw two channels mirrored instead of one.")
	dualChRow.Show()

	symmCombo, _ := gtk.ComboBoxTextNew()
	symmCombo.SetVAlign(gtk.ALIGN_CENTER)
	symmCombo.AppendText(symmetryString(catnip.Vertical))
	symmCombo.AppendText(symmetryString(catnip.Horizontal))
	symmCombo.SetActive(int(ac.Symmetry))
	symmCombo.Show()
	symmCombo.Connect("changed", func() {
		ac.Symmetry = catnip.Symmetry(symmCombo.GetActive())
		apply()
	})

	symmRow := handy.ActionRowNew()
	symmRow.Add(symmCombo)
	symmRow.SetActivatableWidget(symmCombo)
	symmRow.SetTitle("Symmetry")
	symmRow.SetSubtitle("Whether to mirror bars vertically or horizontally.")
	symmRow.Show()

	barGroup := handy.PreferencesGroupNew()
	barGroup.SetTitle("Bars")
	barGroup.Add(lineCapRow)
	barGroup.Add(barRow)
	barGroup.Add(spaceRow)
	barGroup.Add(clampRow)
	barGroup.Add(dualChRow)
	barGroup.Add(symmRow)
	barGroup.Show()

	fgRow := newColorRow(&ac.ForegroundColor, true, apply)
	fgRow.SetTitle("Foreground Color")
	fgRow.SetSubtitle("The color of the visualizer bars.")
	fgRow.Show()

	bgRow := newColorRow(&ac.BackgroundColor, false, apply)
	bgRow.SetTitle("Background Color")
	bgRow.SetSubtitle("The color of the background window.")
	bgRow.Show()

	colorGroup := handy.PreferencesGroupNew()
	colorGroup.SetTitle("Colors")
	colorGroup.Add(fgRow)
	colorGroup.Add(bgRow)
	colorGroup.Show()

	cssText, _ := gtk.TextViewNew()
	cssText.SetBorderWidth(5)
	cssText.SetMonospace(true)
	cssText.SetAcceptsTab(true)
	cssText.Show()

	cssBuf, _ := cssText.GetBuffer()
	cssBuf.SetText(ac.CustomCSS)
	cssBuf.Connect("changed", func() {
		start, end := cssBuf.GetBounds()
		ac.CustomCSS, _ = cssBuf.GetText(start, end, false)
		apply()
	})

	textScroll, _ := gtk.ScrolledWindowNew(nil, nil)
	textScroll.SetPolicy(gtk.POLICY_ALWAYS, gtk.POLICY_NEVER)
	textScroll.SetSizeRequest(-1, 300)
	textScroll.SetVExpand(true)
	textScroll.Add(cssText)
	textScroll.Show()

	cssGroup := handy.PreferencesGroupNew()
	cssGroup.SetTitle("Custom CSS")
	cssGroup.Add(textScroll)
	cssGroup.Show()

	page := handy.PreferencesPageNew()
	page.SetTitle("Appearance")
	page.SetIconName("applications-graphics-symbolic")
	page.Add(barGroup)
	page.Add(colorGroup)
	page.Add(cssGroup)

	return page
}

func newColorRow(optc *OptionalColor, fg bool, apply func()) *handy.ActionRow {
	color, _ := gtk.ColorButtonNew()
	color.SetVAlign(gtk.ALIGN_CENTER)
	color.SetUseAlpha(true)
	color.Show()
	color.Connect("color-set", func() {
		rgba := color.GetRGBA()
		cacc := catnip.ColorFromGDK(*rgba)

		*optc = &cacc
		apply()
	})

	var defaultRGBA = gdk.NewRGBA(0, 0, 0, 0)
	if fg {
		style, _ := color.GetStyleContext()
		defaultRGBA = style.GetColor(gtk.STATE_FLAG_NORMAL)
	}

	var rgba *gdk.RGBA
	if colorValue := *optc; colorValue != nil {
		cacc := *colorValue
		rgba = gdk.NewRGBA(cacc[0], cacc[1], cacc[2], cacc[3])
	}

	if rgba != nil {
		color.SetRGBA(rgba)
	} else {
		color.SetRGBA(defaultRGBA)
	}

	reset, _ := gtk.ButtonNewFromIconName("edit-undo-symbolic", gtk.ICON_SIZE_BUTTON)
	reset.SetRelief(gtk.RELIEF_NONE)
	reset.SetVAlign(gtk.ALIGN_CENTER)
	reset.SetTooltipText("Revert")
	reset.Show()
	reset.Connect("clicked", func() {
		*optc = nil
		color.SetRGBA(defaultRGBA)
		apply()
	})

	row := handy.ActionRowNew()
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
