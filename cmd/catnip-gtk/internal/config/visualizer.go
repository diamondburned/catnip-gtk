package config

import (
	"math"

	"github.com/diamondburned/catnip-gtk"
	"github.com/diamondburned/handy"
	"github.com/gotk3/gotk3/gtk"
	"github.com/noriah/catnip/dsp"
	"gonum.org/v1/gonum/dsp/window"

	catnipwindow "github.com/noriah/catnip/dsp/window"
)

type Visualizer struct {
	SampleRate float64
	FrameRate  float64

	WindowFn     WindowFn
	SmoothFactor float64
	Distribution Distribution

	ScaleSlowWindow     float64
	ScaleFastWindow     float64
	ScaleDumpPercent    float64
	ScaleResetDeviation float64
}

func NewVisualizer() Visualizer {
	return Visualizer{
		SampleRate: 48000,
		FrameRate:  60,

		SmoothFactor: 65.69,
		WindowFn:     BlackmanHarris,
		Distribution: DistributeLog,

		ScaleSlowWindow:     5,
		ScaleFastWindow:     4,
		ScaleDumpPercent:    0.75,
		ScaleResetDeviation: 1.0,
	}
}

func (v Visualizer) SampleSize() int {
	return int(math.Round(v.SampleRate / v.FrameRate))
}

func (v *Visualizer) Page(apply func()) *handy.PreferencesPage {
	sampleRateSpin, _ := gtk.SpinButtonNewWithRange(4000, 192000, 4000)
	sampleRateSpin.SetVAlign(gtk.ALIGN_CENTER)
	sampleRateSpin.SetProperty("digits", 0)
	sampleRateSpin.SetValue(v.SampleRate)
	sampleRateSpin.Show()
	sampleRateSpin.Connect("value-changed", func(sampleRateSpin *gtk.SpinButton) {
		v.SampleRate = sampleRateSpin.GetValue()
		apply()
	})

	sampleRateRow := handy.ActionRowNew()
	sampleRateRow.Add(sampleRateSpin)
	sampleRateRow.SetActivatableWidget(sampleRateSpin)
	sampleRateRow.SetTitle("Sample Rate (Hz)")
	sampleRateRow.SetSubtitle("The sample rate to record; higher is more accurate.")
	sampleRateRow.Show()

	frameRateSpin, _ := gtk.SpinButtonNewWithRange(5, 240, 5)
	frameRateSpin.SetVAlign(gtk.ALIGN_CENTER)
	frameRateSpin.SetProperty("digits", 0)
	frameRateSpin.SetValue(v.FrameRate)
	frameRateSpin.Show()
	frameRateSpin.Connect("value-changed", func(frameRateSpin *gtk.SpinButton) {
		v.FrameRate = frameRateSpin.GetValue()
		apply()
	})

	frameRateRow := handy.ActionRowNew()
	frameRateRow.Add(frameRateSpin)
	frameRateRow.SetActivatableWidget(frameRateSpin)
	frameRateRow.SetTitle("Frame Rate (fps)")
	frameRateRow.SetSubtitle("The frame rate to sample; lower is more accurate.")
	frameRateRow.Show()

	samplingGroup := handy.PreferencesGroupNew()
	samplingGroup.SetTitle("Sampling")
	samplingGroup.Add(sampleRateRow)
	samplingGroup.Add(frameRateRow)
	samplingGroup.Show()

	windowCombo, _ := gtk.ComboBoxTextNew()
	windowCombo.SetVAlign(gtk.ALIGN_CENTER)
	windowCombo.Show()
	for _, windowFn := range windowFns {
		windowCombo.Append(string(windowFn), string(windowFn))
	}
	windowCombo.SetActiveID(string(v.WindowFn))
	windowCombo.Connect("changed", func(windowCombo *gtk.ComboBoxText) {
		v.WindowFn = WindowFn(windowCombo.GetActiveID())
		apply()
	})

	windowRow := handy.ActionRowNew()
	windowRow.Add(windowCombo)
	windowRow.SetActivatableWidget(windowCombo)
	windowRow.SetTitle("Window Function")
	windowRow.SetSubtitle("The window function to use for signal processing.")
	windowRow.Show()

	distributionCombo, _ := gtk.ComboBoxTextNew()
	distributionCombo.SetVAlign(gtk.ALIGN_CENTER)
	distributionCombo.Show()
	distributionCombo.Append(string(DistributeLog), string(DistributeLog))
	distributionCombo.Append(string(DistributeEqual), string(DistributeEqual))
	distributionCombo.SetActiveID(string(v.Distribution))
	distributionCombo.Connect("changed", func(distributionCombo *gtk.ComboBoxText) {
		v.Distribution = Distribution(distributionCombo.GetActiveID())
		apply()
	})

	distributionRow := handy.ActionRowNew()
	distributionRow.Add(distributionCombo)
	distributionRow.SetActivatableWidget(distributionRow)
	distributionRow.SetTitle("Distribution")
	distributionRow.SetSubtitle("The frequency distribution algorithm to use.")
	distributionRow.Show()

	smoothFactorSpin, _ := gtk.SpinButtonNewWithRange(0, 100, 2)
	smoothFactorSpin.SetVAlign(gtk.ALIGN_CENTER)
	smoothFactorSpin.SetProperty("digits", 2)
	smoothFactorSpin.SetValue(v.SmoothFactor)
	smoothFactorSpin.Show()
	smoothFactorSpin.Connect("value-changed", func(smoothFactorSpin *gtk.SpinButton) {
		v.SmoothFactor = smoothFactorSpin.GetValue()
		apply()
	})

	smoothFactorRow := handy.ActionRowNew()
	smoothFactorRow.Add(smoothFactorSpin)
	smoothFactorRow.SetActivatableWidget(smoothFactorSpin)
	smoothFactorRow.SetTitle("Smooth Factor")
	smoothFactorRow.SetSubtitle("The variable for smoothing; higher means smoother.")
	smoothFactorRow.Show()

	signalProcGroup := handy.PreferencesGroupNew()
	signalProcGroup.SetTitle("Signal Processing")
	signalProcGroup.Add(windowRow)
	signalProcGroup.Add(distributionRow)
	signalProcGroup.Add(smoothFactorRow)
	signalProcGroup.Show()

	page := handy.PreferencesPageNew()
	page.SetTitle("Visualizer")
	page.SetIconName("preferences-desktop-display-symbolic")
	page.Add(samplingGroup)
	page.Add(signalProcGroup)

	return page
}

type Distribution string

const (
	DistributeLog   Distribution = "Logarithmic"
	DistributeEqual Distribution = "Equal"
)

func (dt Distribution) AsSpectrumType() dsp.SpectrumType {
	switch dt {
	case DistributeLog:
		return dsp.TypeLog
	case DistributeEqual:
		return dsp.TypeEqual
	default:
		return dsp.TypeDefault
	}
}

type WindowFn string

const (
	BartlettHann    WindowFn = "Bartlett–Hann"
	Blackman        WindowFn = "Blackman"
	BlackmanHarris  WindowFn = "Blackman–Harris"
	BlackmanNuttall WindowFn = "Blackman–Nuttall"
	FlatTop         WindowFn = "Flat Top"
	Hamming         WindowFn = "Hamming"
	Hann            WindowFn = "Hann"
	Lanczos         WindowFn = "Lanczos"
	Nuttall         WindowFn = "Nuttall"
	Rectangular     WindowFn = "Rectangular"
	Sine            WindowFn = "Sine"
	Triangular      WindowFn = "Triangular"
	CosineSum       WindowFn = "Cosine-Sum"
	PlanckTaper     WindowFn = "Planck–Taper"
)

var windowFns = []WindowFn{
	BartlettHann,
	Blackman,
	BlackmanHarris,
	BlackmanNuttall,
	FlatTop,
	Hamming,
	Hann,
	Lanczos,
	Nuttall,
	Rectangular,
	Sine,
	Triangular,
	CosineSum,
	PlanckTaper,
}

func (wfn WindowFn) AsFunction() catnipwindow.Function {
	switch wfn {
	case BartlettHann:
		return catnip.WrapExternalWindowFn(window.BartlettHann)
	case Blackman:
		return catnip.WrapExternalWindowFn(window.Blackman)
	case BlackmanHarris:
		return catnip.WrapExternalWindowFn(window.BlackmanHarris)
	case BlackmanNuttall:
		return catnip.WrapExternalWindowFn(window.BlackmanNuttall)
	case FlatTop:
		return catnip.WrapExternalWindowFn(window.FlatTop)
	case Hamming:
		return catnip.WrapExternalWindowFn(window.Hamming)
	case Hann:
		return catnip.WrapExternalWindowFn(window.Hann)
	case Lanczos:
		return catnip.WrapExternalWindowFn(window.Lanczos)
	case Nuttall:
		return catnip.WrapExternalWindowFn(window.Nuttall)
	case Rectangular:
		return catnip.WrapExternalWindowFn(window.Rectangular)
	case Sine:
		return catnip.WrapExternalWindowFn(window.Sine)
	case Triangular:
		return catnip.WrapExternalWindowFn(window.Triangular)
	case CosineSum:
		return func(buf []float64) { catnipwindow.CosSum(buf, 0.5) }
	case PlanckTaper:
		return func(buf []float64) { catnipwindow.PlanckTaper(buf, 0.5) }
	default:
		return Blackman.AsFunction()
	}
}
