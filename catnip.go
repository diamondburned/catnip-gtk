package catnip

import (
	"image/color"

	"github.com/gotk3/gotk3/gtk"
	"github.com/noriah/catnip/dsp"
)

// Config is the catnip config.
type Config struct {
	// Backend is the backend name from list-backends
	Backend string
	// Device is the device name from list-devices
	Device string

	Scaling ScalingConfig

	ForegroundColor color.Color // use Gtk if nil
	BackgroundColor color.Color // transparent if nil

	SampleRate   float64
	SmoothFactor float64
	WinVar       float64
	BarWidth     float64 // not really pixels
	SpaceWidth   float64 // not really pixels
	SampleSize   int
	Monophonic   bool
	MinimumClamp float64 // height before visible
	SpectrumType dsp.SpectrumType
}

type ScalingConfig struct {
	SlowWindow     float64
	FastWindow     float64
	DumpPercent    float64
	ResetDeviation float64
}

func NewConfig() Config {
	return Config{
		Backend: "portaudio",
		Device:  "",

		SampleRate:   48000,
		SmoothFactor: 65.69,
		WinVar:       0.50,
		BarWidth:     10,
		SpaceWidth:   5,
		SampleSize:   48000 / 30, // 30fps
		Monophonic:   false,
		MinimumClamp: 1,
		SpectrumType: dsp.TypeDefault,

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
	gtk.DrawingArea
	drawer *Drawer
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
		DrawingArea: *draw,
		drawer:      drawer,
	}
}

// Start starts drawing the visualizer onto the area.
func (a *Area) Start() error {
	return a.drawer.Start()
}
