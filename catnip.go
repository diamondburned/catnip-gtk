package catnip

import (
	"image/color"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gtk"
	"github.com/noriah/catnip/dsp"
	"github.com/noriah/catnip/dsp/window"
)

// Config is the catnip config.
type Config struct {
	// Backend is the backend name from list-backends
	Backend string
	// Device is the device name from list-devices
	Device string

	DrawOptions

	WindowFn     window.Function // default CosSum, a0 = 0.50
	Scaling      ScalingConfig
	SampleRate   float64
	SmoothFactor float64
	WinVar       float64
	SampleSize   int
	Monophonic   bool
	MinimumClamp float64 // height before visible
	SpectrumType dsp.SpectrumType
}

// WrapExternalWindowFn wraps external (mostly gonum/dsp/window) functions to be
// compatible with catnip's usage. The implementation will assume that the given
// function modifies the given slice in its place, which is the case for most
// gonum functions, but it might not always be the case. If the implementation
// does not, then the caller should write their own function to copy.
func WrapExternalWindowFn(fn func([]float64) []float64) window.Function {
	return func(buf []float64) { fn(buf) }
}

// DrawOptions is the option for Cairo draws.
type DrawOptions struct {
	LineCap  cairo.LineCap  // default BUTT
	LineJoin cairo.LineJoin // default MITER

	Colors     Colors
	Offsets    DrawOffsets
	BarWidth   float64 // not really pixels
	SpaceWidth float64 // not really pixels

	// ForceEven will round the width and height to be even. This will force
	// Cairo to always draw the bars sharply.
	ForceEven bool
}

func (opts DrawOptions) even(n int) int {
	if !opts.ForceEven {
		return n
	}
	return n - (n % 2)
}

// DrawOffsets controls the offset for the Drawer.
type DrawOffsets struct {
	X, Y float64
}

// apply applies the draw offset.
func (offset DrawOffsets) apply(x, y float64) (float64, float64) {
	return x + offset.X, y + offset.Y
}

// Colors is the color settings for the Drawer.
type Colors struct {
	Foreground color.Color // use Gtk if nil
	Background color.Color // transparent if nil
}

// ScalingConfig is the scaling settings for the visualizer.
type ScalingConfig struct {
	StaticScale    float64 // 0 for dynamic scale
	SlowWindow     float64
	FastWindow     float64
	DumpPercent    float64
	ResetDeviation float64
}

func NewConfig() Config {
	return Config{
		Backend: "portaudio",
		Device:  "",

		// Default to CosSum with WinVar = 0.50.
		WindowFn: func(buf []float64) { window.CosSum(buf, 0.50) },

		SampleRate:   48000,
		SmoothFactor: 65.69,
		SampleSize:   48000 / 30, // 30fps
		Monophonic:   false,
		MinimumClamp: 1,
		SpectrumType: dsp.TypeDefault,

		DrawOptions: DrawOptions{
			LineCap:    cairo.LINE_CAP_BUTT,
			LineJoin:   cairo.LINE_JOIN_MITER,
			BarWidth:   10,
			SpaceWidth: 5,
		},

		Scaling: ScalingConfig{
			SlowWindow:     5,
			FastWindow:     5 * 0.2,
			DumpPercent:    0.75,
			ResetDeviation: 1.0,
		},
	}
}

// Area is the area that Catnip draws onto.
type Area struct {
	*Drawer
	gtk.DrawingArea
}

// New creates a new Catnip DrawingArea from the given config.
func New(cfg Config) *Area {
	draw, _ := gtk.DrawingAreaNew()

	drawer := NewDrawer(draw, cfg)
	drawer.SetWidgetStyle(draw)
	drawer.ConnectDraw(draw)
	drawer.ConnectDestroy(draw)
	drawer.ConnectSizeAllocate(draw)

	return &Area{
		Drawer:      drawer,
		DrawingArea: *draw,
	}
}
