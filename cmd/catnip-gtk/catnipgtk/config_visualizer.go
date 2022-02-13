package catnipgtk

import (
	"fmt"

	"github.com/diamondburned/catnip-gtk"
	"github.com/diamondburned/gotk4-handy/pkg/handy"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
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
	samplingGroup := handy.NewPreferencesGroup()

	updateSamplingLabel := func() {
		fₛ := v.SampleRate / float64(v.SampleSize)
		samplingGroup.SetTitle(fmt.Sprintf(
			"Sampling and Drawing (fₛ ≈ %.1f samples/s, latency ≈ %.1fms)",
			fₛ, 1000/fₛ,
		))
	}
	updateSamplingLabel()

	sampleRateSpin := gtk.NewSpinButtonWithRange(4000, 192000, 4000)
	sampleRateSpin.SetVAlign(gtk.AlignCenter)
	sampleRateSpin.SetDigits(0)
	sampleRateSpin.SetValue(v.SampleRate)
	sampleRateSpin.Show()
	sampleRateSpin.Connect("value-changed", func(sampleRateSpin *gtk.SpinButton) {
		v.SampleRate = sampleRateSpin.Value()
		updateSamplingLabel()
		apply()
	})

	sampleRateRow := handy.NewActionRow()
	sampleRateRow.Add(sampleRateSpin)
	sampleRateRow.SetActivatableWidget(sampleRateSpin)
	sampleRateRow.SetTitle("Sample Rate (Hz)")
	sampleRateRow.SetSubtitle("The sample rate to record; higher is more accurate.")
	sampleRateRow.Show()

	sampleSizeSpin := gtk.NewSpinButtonWithRange(1, 102400, 128)
	sampleSizeSpin.SetVAlign(gtk.AlignCenter)
	sampleSizeSpin.SetDigits(0)
	sampleSizeSpin.SetValue(float64(v.SampleSize))
	sampleSizeSpin.Show()
	sampleSizeSpin.Connect("value-changed", func(sampleSizeSpin *gtk.SpinButton) {
		v.SampleSize = sampleSizeSpin.ValueAsInt()
		updateSamplingLabel()
		apply()
	})

	sampleSizeRow := handy.NewActionRow()
	sampleSizeRow.Add(sampleSizeSpin)
	sampleSizeRow.SetActivatableWidget(sampleSizeSpin)
	sampleSizeRow.SetTitle("Sample Size")
	sampleSizeRow.SetSubtitle("The sample size to record; higher is more accurate but slower.")
	sampleSizeRow.Show()

	frameRateSpin := gtk.NewSpinButtonWithRange(5, 240, 5)
	frameRateSpin.SetVAlign(gtk.AlignCenter)
	frameRateSpin.SetDigits(0)
	frameRateSpin.SetValue(float64(v.FrameRate))
	frameRateSpin.Show()
	frameRateSpin.Connect("value-changed", func(frameRateSpin *gtk.SpinButton) {
		v.FrameRate = frameRateSpin.ValueAsInt()
		apply()
	})

	frameRateRow := handy.NewActionRow()
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

	windowCombo := gtk.NewComboBoxText()
	windowCombo.SetVAlign(gtk.AlignCenter)
	windowCombo.Show()
	for _, windowFn := range windowFns {
		windowCombo.Append(string(windowFn), string(windowFn))
	}
	windowCombo.SetActiveID(string(v.WindowFn))
	windowCombo.Connect("changed", func(windowCombo *gtk.ComboBoxText) {
		v.WindowFn = WindowFn(windowCombo.ActiveID())
		apply()
	})

	windowRow := handy.NewActionRow()
	windowRow.Add(windowCombo)
	windowRow.SetActivatableWidget(windowCombo)
	windowRow.SetTitle("Window Function")
	windowRow.SetSubtitle("The window function to use for signal processing.")
	windowRow.Show()

	smoothFactorSpin := gtk.NewSpinButtonWithRange(0, 100, 2)
	smoothFactorSpin.SetVAlign(gtk.AlignCenter)
	smoothFactorSpin.SetDigits(2)
	smoothFactorSpin.SetValue(v.SmoothFactor)
	smoothFactorSpin.Show()
	smoothFactorSpin.Connect("value-changed", func(smoothFactorSpin *gtk.SpinButton) {
		v.SmoothFactor = smoothFactorSpin.Value()
		apply()
	})

	smoothFactorRow := handy.NewActionRow()
	smoothFactorRow.Add(smoothFactorSpin)
	smoothFactorRow.SetActivatableWidget(smoothFactorSpin)
	smoothFactorRow.SetTitle("Smooth Factor")
	smoothFactorRow.SetSubtitle("The variable for smoothing; higher means smoother.")
	smoothFactorRow.Show()

	signalProcGroup := handy.NewPreferencesGroup()
	signalProcGroup.SetTitle("Signal Processing")
	signalProcGroup.Add(windowRow)
	signalProcGroup.Add(smoothFactorRow)
	signalProcGroup.Show()

	page := handy.NewPreferencesPage()
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
