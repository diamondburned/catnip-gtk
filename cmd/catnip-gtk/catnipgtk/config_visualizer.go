package catnipgtk

import (
	"fmt"

	"github.com/diamondburned/catnip-gtk"
	"github.com/diamondburned/handy"
	"github.com/gotk3/gotk3/gtk"
	"gonum.org/v1/gonum/dsp/window"

	catnipwindow "github.com/noriah/catnip/dsp/window"
)

type Visualizer struct {
	SampleRate float64
	SampleSize int
	FrameRate  int

	WindowFn     WindowFn
	SmoothFactor float64

	ScaleSlowWindow     float64
	ScaleFastWindow     float64
	ScaleDumpPercent    float64
	ScaleResetDeviation float64
}

func NewVisualizer() Visualizer {
	return Visualizer{
		SampleRate: 44100,
		SampleSize: 1024,
		FrameRate:  60,

		SmoothFactor: 65.69,
		WindowFn:     BlackmanHarris,

		ScaleSlowWindow:     5,
		ScaleFastWindow:     4,
		ScaleDumpPercent:    0.75,
		ScaleResetDeviation: 1.0,
	}
}

func (v *Visualizer) Page(apply func()) *handy.PreferencesPage {
	samplingGroup := handy.PreferencesGroupNew()

	updateSamplingLabel := func() {
		fₛ := v.SampleRate / float64(v.SampleSize)
		samplingGroup.SetTitle(fmt.Sprintf(
			"Sampling and Drawing (fₛ ≈ %.1f samples/s, latency ≈ %.1fms)",
			fₛ, 1000/fₛ,
		))
	}
	updateSamplingLabel()

	sampleRateSpin, _ := gtk.SpinButtonNewWithRange(4000, 192000, 4000)
	sampleRateSpin.SetVAlign(gtk.ALIGN_CENTER)
	sampleRateSpin.SetProperty("digits", 0)
	sampleRateSpin.SetValue(v.SampleRate)
	sampleRateSpin.Show()
	sampleRateSpin.Connect("value-changed", func(sampleRateSpin *gtk.SpinButton) {
		v.SampleRate = sampleRateSpin.GetValue()
		updateSamplingLabel()
		apply()
	})

	sampleRateRow := handy.ActionRowNew()
	sampleRateRow.Add(sampleRateSpin)
	sampleRateRow.SetActivatableWidget(sampleRateSpin)
	sampleRateRow.SetTitle("Sample Rate (Hz)")
	sampleRateRow.SetSubtitle("The sample rate to record; higher is more accurate.")
	sampleRateRow.Show()

	sampleSizeSpin, _ := gtk.SpinButtonNewWithRange(1, 102400, 128)
	sampleSizeSpin.SetVAlign(gtk.ALIGN_CENTER)
	sampleSizeSpin.SetProperty("digits", 0)
	sampleSizeSpin.SetValue(float64(v.SampleSize))
	sampleSizeSpin.Show()
	sampleSizeSpin.Connect("value-changed", func(sampleSizeSpin *gtk.SpinButton) {
		v.SampleSize = sampleSizeSpin.GetValueAsInt()
		updateSamplingLabel()
		apply()
	})

	sampleSizeRow := handy.ActionRowNew()
	sampleSizeRow.Add(sampleSizeSpin)
	sampleSizeRow.SetActivatableWidget(sampleSizeSpin)
	sampleSizeRow.SetTitle("Sample Size")
	sampleSizeRow.SetSubtitle("The sample size to record; higher is more accurate but slower.")
	sampleSizeRow.Show()

	frameRateSpin, _ := gtk.SpinButtonNewWithRange(5, 240, 5)
	frameRateSpin.SetVAlign(gtk.ALIGN_CENTER)
	frameRateSpin.SetProperty("digits", 0)
	frameRateSpin.SetValue(float64(v.FrameRate))
	frameRateSpin.Show()
	frameRateSpin.Connect("value-changed", func(frameRateSpin *gtk.SpinButton) {
		v.FrameRate = frameRateSpin.GetValueAsInt()
		apply()
	})

	frameRateRow := handy.ActionRowNew()
	frameRateRow.Add(frameRateSpin)
	frameRateRow.SetActivatableWidget(frameRateSpin)
	frameRateRow.SetTitle("Frame Rate (fps)")
	frameRateRow.SetSubtitle("The frame rate to draw in parallel with the sampling; " +
		"this affects the smoothing.")
	frameRateRow.Show()

	samplingGroup.Add(sampleRateRow)
	samplingGroup.Add(sampleSizeRow)
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
	signalProcGroup.Add(smoothFactorRow)
	signalProcGroup.Show()

	page := handy.PreferencesPageNew()
	page.SetTitle("Visualizer")
	page.SetIconName("preferences-desktop-display-symbolic")
	page.Add(samplingGroup)
	page.Add(signalProcGroup)

	return page
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
