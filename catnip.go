package catnip

import (
	"fmt"
	"image/color"
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
	"github.com/noriah/catnip/dsp/window"
	"github.com/noriah/catnip/input"
	"github.com/pkg/errors"
)

// Config is the catnip config.
type Config struct {
	// Backend is the backend name from list-backends
	Backend string
	// Device is the device name from list-devices
	Device string

	WindowFn     window.Function // default CosSum, a0 = 0.50
	Scaling      ScalingConfig
	SampleRate   float64
	SampleSize   int
	SmoothFactor float64
	MinimumClamp float64 // height before visible

	DrawOptions

	DrawStyle  DrawStyle
	Monophonic bool
}

// DrawStyle is the style to draw the bars symmetrically.
type DrawStyle uint8

const (
	// VerticalBars is drawing the bars vertically mirrored if stereo.
	DrawVerticalBars DrawStyle = iota
	// HorizontalBars is drawing the bars horizontally mirrored if stereo.
	DrawHorizontalBars
	// DrawLines draws the spectrum as lines.
	DrawLines
)

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
	LineCap   cairo.LineCap  // default BUTT
	LineJoin  cairo.LineJoin // default MITER
	AntiAlias cairo.Antialias

	FrameRate int

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

func (opts DrawOptions) round(f float64) float64 {
	if !opts.ForceEven {
		return f
	}
	return math.Round(f)
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
		SampleSize:   48000 / 30, // 30 resample/s
		SmoothFactor: 65.69,
		Monophonic:   false,
		MinimumClamp: 1,

		DrawOptions: DrawOptions{
			LineCap:    cairo.LINE_CAP_BUTT,
			LineJoin:   cairo.LINE_JOIN_MITER,
			AntiAlias:  cairo.ANTIALIAS_DEFAULT,
			FrameRate:  60, // 60fps
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

// InitBackend initializes a new input backend.
func (c *Config) InitBackend() (input.Backend, error) {
	backend := input.FindBackend(c.Backend)
	if backend == nil {
		return nil, fmt.Errorf("backend not found: %q", c.Backend)
	}

	if err := backend.Init(); err != nil {
		return nil, errors.Wrap(err, "failed to initialize input backend")
	}

	return backend, nil
}

// InitDevice initializes an input device with the given initalized backend.
func (c *Config) InitDevice(b input.Backend) (input.Device, error) {
	if c.Device == "" {
		def, err := b.DefaultDevice()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get default device")
		}

		return def, nil
	}

	devices, err := b.Devices()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get devices")
	}

	for idx := range devices {
		if devices[idx].String() == c.Device {
			return devices[idx], nil
		}
	}

	return nil, errors.Errorf("device %q not found; check list-devices", c.Device)
}

// Area is the area that Catnip draws onto.
type Area struct {
	*gtk.DrawingArea
	*Drawer
}

// New creates a new Catnip DrawingArea from the given config.
func New(cfg Config) *Area {
	draw := gtk.NewDrawingArea()
	return &Area{
		DrawingArea: draw,
		Drawer:      NewDrawer(draw, cfg),
	}
}
