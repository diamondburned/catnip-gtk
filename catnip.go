package catnip

import (
	"context"
	"image/color"
	"math"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/noriah/catnip/dsp"
	"github.com/noriah/catnip/dsp/window"
	"github.com/noriah/catnip/fft"
	"github.com/noriah/catnip/input"
	"github.com/noriah/catnip/util"
	"github.com/pkg/errors"
)

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
		SpectrumType: dsp.TypeDefault,

		Scaling: ScalingConfig{
			SlowWindow:     5,
			FastWindow:     5 * 0.2,
			DumpPercent:    0.75,
			ResetDeviation: 1.0,
		},
	}
}

type cairoColor [4]float64

// Area is the area that Catnip draws onto.
type Area struct {
	gtk.DrawingArea
	cfg Config

	ctx    context.Context
	cancel context.CancelFunc

	backend input.Backend
	session input.Session

	fg cairoColor
	bg cairoColor

	// total bar + space width
	binWidth float64
	// channels; 1 if monophonic
	channels int

	drawState drawState
}

// New creates a new Catnip DrawingArea from the given config.
func New(cfg Config) *Area {
	ctx, cancel := context.WithCancel(context.Background())

	area := &Area{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,

		channels: 2,
		// Weird Cairo tricks require multiplication and division by 2. Unsure
		// why.
		binWidth: cfg.BarWidth + (cfg.SpaceWidth * 2),
	}

	if cfg.Monophonic {
		area.channels = 1
	}

	// Allocate a second bar buffer for copying.
	area.drawState.barBufs = area.makeBarBuf()

	draw, _ := gtk.DrawingAreaNew()
	draw.Connect("draw", area.draw)
	draw.Connect("size-allocate", func() {
		area.drawState.SetWidth(draw.GetAllocatedWidth())
	})
	draw.Connect("destroy", area.stop)

	area.DrawingArea = *draw

	// Get the colors.
	styleCtx, _ := draw.GetStyleContext()

	area.fg = getColor(cfg.ForegroundColor, styleCtx.GetColor(gtk.STATE_FLAG_NORMAL))
	area.bg = getColor(cfg.BackgroundColor, gdk.NewRGBA(0, 0, 0, 0))

	return area
}

// getColor gets the color from the given c Color interface. If c is nil, then
// the color is taken from the given gdk.RGBA instead.
func getColor(c color.Color, rgba *gdk.RGBA) (color cairoColor) {
	if c == nil {
		copy(color[:], rgba.Floats())
		return
	}

	r, g, b, a := c.RGBA()

	color[0] = float64(r) / 0xFFFF
	color[1] = float64(g) / 0xFFFF
	color[2] = float64(b) / 0xFFFF
	color[3] = float64(a) / 0xFFFF

	return
}

func (a *Area) stop() {
	a.cancel()
	a.session.Stop()
	a.backend.Close()
}

// Start starts the area. This function blocks permanently until the audio loop
// is dead, so it should be called inside a goroutine. This function should not
// be called more than once, else it will panic.
//
// The loop will automatically close when the DrawingArea is destroyed.
func (a *Area) Start() error {
	if a.session != nil {
		// Panic is reasonable, as calling Start() multiple times (in multiple
		// goroutines) may cause undefined behaviors.
		panic("BUG: catnip.Area is already started.")
	}

	backend, err := initBackend(a.cfg)
	if err != nil {
		return errors.Wrap(err, "failed to initialize input backend")
	}
	a.backend = backend

	device, err := getDevice(backend, a.cfg)
	if err != nil {
		return err
	}

	var sessConfig = input.SessionConfig{
		Device:     device,
		FrameSize:  a.channels,
		SampleSize: a.cfg.SampleSize,
		SampleRate: a.cfg.SampleRate,
	}

	session, err := backend.Start(sessConfig)
	if err != nil {
		return errors.Wrap(err, "failed to start the input backend")
	}
	a.session = session

	if err := session.Start(); err != nil {
		return errors.Wrap(err, "failed to start input session")
	}

	return a.start()
}

func (a *Area) start() error {
	var (
		fftBuf   = make([]complex128, a.cfg.SampleSize/2+1)
		spBinBuf = make(dsp.BinBuf, a.cfg.SampleSize)

		slowMax    = int(a.cfg.Scaling.SlowWindow*a.cfg.SampleRate) / a.cfg.SampleSize * 2
		fastMax    = int(a.cfg.Scaling.FastWindow*a.cfg.SampleRate) / a.cfg.SampleSize * 2
		windowData = make([]float64, slowMax+fastMax)

		barBufs = a.makeBarBuf()
		plans   = make([]*fft.Plan, a.channels)
	)

	var slowWindow = &util.MovingWindow{
		Data:     windowData[0:slowMax],
		Capacity: slowMax,
	}

	var fastWindow = &util.MovingWindow{
		Data:     windowData[slowMax : slowMax+fastMax],
		Capacity: fastMax,
	}

	// DrawDelay is the time we wait between ticks to draw.
	var drawDelay = time.Second / time.Duration(a.cfg.SampleRate/float64(a.cfg.SampleSize))

	// Make a spectrum
	var spectrum = dsp.Spectrum{
		SampleRate: a.cfg.SampleRate,
		SampleSize: a.cfg.SampleSize,
		Bins:       spBinBuf,
	}

	spectrum.SetSmoothing(a.cfg.SmoothFactor / 100)
	spectrum.SetWinVar(a.cfg.WinVar)
	spectrum.SetType(a.cfg.SpectrumType)

	// Root Context

	var inputBufs = a.session.SampleBuffers()

	for idx, buf := range inputBufs {
		plans[idx] = &fft.Plan{
			Input:  buf,
			Output: fftBuf,
		}
	}

	var barCount int
	var peak, scale float64

	var timer = time.NewTimer(drawDelay)
	defer timer.Stop()

	defer a.drawState.Invalidate()

	// Periodically queue redraw.
	glib.TimeoutAdd(uint(drawDelay/time.Millisecond), func() bool {
		a.QueueDraw()

		select {
		case <-a.ctx.Done():
			return false
		default:
			return true
		}
	})

	for {
		if a.session.ReadyRead() < a.cfg.SampleSize {
			continue
		}

		if err := a.session.Read(a.ctx); err != nil {
			return errors.Wrap(err, "failed to read audio input")
		}

		if barVar := a.bars(); barVar != barCount {
			barCount = spectrum.Recalculate(barVar)
		}

		peak = 0
		scale = 0

		for idx, buf := range barBufs {
			window.CosSum(inputBufs[idx], a.cfg.WinVar)
			plans[idx].Execute()
			spectrum.Process(buf, fftBuf)

			for _, v := range buf[:barCount] {
				if peak < v {
					peak = v
				}
			}
		}

		// do some scaling if we are above 0
		if peak > 0 {
			fastWindow.Update(peak)
			var vMean, vSD = slowWindow.Update(peak)

			if length := slowWindow.Len(); length >= fastWindow.Cap() {
				if math.Abs(fastWindow.Mean()-vMean) > (a.cfg.Scaling.ResetDeviation * vSD) {
					vMean, vSD = slowWindow.Drop(int(float64(length) * a.cfg.Scaling.DumpPercent))
				}
			}

			if t := vMean + (1.5 * vSD); t > 1.0 {
				scale = t
			}
		}

		a.drawState.Set(barBufs, barCount, scale)

		select {
		case <-a.ctx.Done():
			return nil
		default:
			// micro-optimization
		}

		<-timer.C
		timer.Reset(drawDelay)
	}
}

func (a *Area) makeBarBuf() [][]float64 {
	// Allocate a large slice with one large backing array.
	var fullBuf = make([]float64, a.channels*a.cfg.SampleSize)

	// Allocate smaller slice views.
	var barBufs = make([][]float64, a.channels)

	for idx := range barBufs {

		start := idx * a.cfg.SampleSize
		end := (idx + 1) * a.cfg.SampleSize

		barBufs[idx] = fullBuf[start:end]
	}

	return barBufs
}

// bars calculates the number of bars. It is thread-safe.
func (a *Area) bars() int {
	return int(a.drawState.Width() / a.binWidth / float64(a.channels))
}
