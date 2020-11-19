package catnip

import (
	"context"
	"image/color"
	"math"
	"sync/atomic"
	"time"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/noriah/catnip/dsp"
	"github.com/noriah/catnip/fft"
	"github.com/noriah/catnip/input"
	"github.com/noriah/catnip/util"
	"github.com/pkg/errors"
)

type CairoColor [4]float64

func ColorFromGDK(rgba gdk.RGBA) CairoColor {
	var cairoC CairoColor
	copy(cairoC[:], rgba.Floats())
	return cairoC
}

func (cc CairoColor) RGBA() (r, g, b, a uint32) {
	r = uint32(cc[0] * 0xFFFF)
	g = uint32(cc[1] * 0xFFFF)
	b = uint32(cc[2] * 0xFFFF)
	a = uint32(cc[3] * 0xFFFF)
	return
}

// DrawQueuer is a custom widget interface that allows draw queueing.
type DrawQueuer interface {
	QueueDraw()
}

var _ DrawQueuer = (*gtk.Widget)(nil)

// Drawer is the separated drawer state without any widget.
type Drawer struct {
	drawQ DrawQueuer

	cfg    Config
	ctx    context.Context
	cancel context.CancelFunc

	backend input.Backend
	device  input.Device
	session input.Session

	fg CairoColor
	bg CairoColor

	// total bar + space width
	binWidth float64
	// channels; 1 if monophonic
	channels int

	paused uint32

	drawState drawState
}

// NewDrawer creates a separated drawer state. The given drawQueuer will be
// called every redrawn frame.
func NewDrawer(drawQ DrawQueuer, cfg Config) *Drawer {
	ctx, cancel := context.WithCancel(context.Background())

	drawer := &Drawer{
		drawQ:  drawQ,
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,

		fg: getColor(cfg.Colors.Foreground, nil, CairoColor{0, 0, 0, 1}),
		bg: getColor(cfg.Colors.Background, nil, CairoColor{0, 0, 0, 0}),

		channels: 2,
		// Weird Cairo tricks require multiplication and division by 2. Unsure
		// why.
		binWidth: cfg.BarWidth + (cfg.SpaceWidth * 2),
	}

	if cfg.Monophonic {
		drawer.channels = 1
	}

	// Allocate a second bar buffer for copying.
	drawer.drawState.barBufs = drawer.makeBarBuf()

	return drawer
}

// getColor gets the color from the given c Color interface. If c is nil, then
// the color is taken from the given gdk.RGBA instead.
func getColor(c color.Color, rgba *gdk.RGBA, fallback CairoColor) (cairoC CairoColor) {
	if c != nil {
		switch c := c.(type) {
		case CairoColor:
			return c
		case *CairoColor:
			return *c
		}

		r, g, b, a := c.RGBA()

		cairoC[0] = float64(r) / 0xFFFF
		cairoC[1] = float64(g) / 0xFFFF
		cairoC[2] = float64(b) / 0xFFFF
		cairoC[3] = float64(a) / 0xFFFF

		return
	}

	if rgba != nil {
		copy(cairoC[:], rgba.Floats())
		return
	}

	return fallback
}

// StyleContexter is the interface to get a style context.
type StyleContexter interface {
	GetStyleContext() (*gtk.StyleContext, error)
}

// SetWidgetStyle lets the Drawer take the given widgetStyler's styles. It
// doesn't set colors that weren't nil in the config.
func (d *Drawer) SetWidgetStyle(widgetStyler StyleContexter) {
	styleCtx, _ := widgetStyler.GetStyleContext()

	d.fg = getColor(d.cfg.Colors.Foreground, styleCtx.GetColor(gtk.STATE_FLAG_NORMAL), d.fg)
	d.bg = getColor(d.cfg.Colors.Background, gdk.NewRGBA(0, 0, 0, 0), d.bg)
}

// Connector is the interface to connect any widget.
type Connector interface {
	gtk.IWidget
	AllocatedSizeGetter
	Connect(string, interface{}, ...interface{}) (glib.SignalHandle, error)
}

var _ Connector = (*gtk.Widget)(nil)

// ConnectDraw connects the given connector to the Draw method.
func (d *Drawer) ConnectDraw(c Connector) (glib.SignalHandle, error) {
	return c.Connect("draw", d.Draw)
}

// ConnectSizeAllocate connects the given connector to update the width of the
// drawer, and consequently, the number of bars drawn.
func (d *Drawer) ConnectSizeAllocate(c Connector) (glib.SignalHandle, error) {
	d.drawState.SetWidth(c.GetAllocatedWidth())

	return c.Connect("size-allocate", func() {
		d.drawState.SetWidth(c.GetAllocatedWidth())
	})
}

// ConnectDestroy connects the destroy signal to destroy the drawer as well. If
// this method is used, then the caller does not need to call Stop().
func (d *Drawer) ConnectDestroy(c Connector) (glib.SignalHandle, error) {
	return c.Connect("destroy", d.cancel)
}

// SetPaused will silent all inputs if true.
func (d *Drawer) SetPaused(paused bool) {
	if paused {
		atomic.StoreUint32(&d.paused, 1)
	} else {
		atomic.StoreUint32(&d.paused, 0)
	}
}

// AllocatedSizeGetter is any widget that can be obtained dimensions of. This is
// used for the Draw method.
type AllocatedSizeGetter interface {
	GetAllocatedWidth() int
	GetAllocatedHeight() int
}

// Draw is bound to the draw signal. Although Draw won't crash if Drawer is not
// started yet, the drawn result is undefined.
func (d *Drawer) Draw(w AllocatedSizeGetter, cr *cairo.Context) {
	var (
		width  = float64(d.cfg.even(w.GetAllocatedWidth()))
		height = float64(d.cfg.even(w.GetAllocatedHeight()))
	)

	cr.SetSourceRGBA(d.bg[0], d.bg[1], d.bg[2], d.bg[3])
	cr.SetLineWidth(d.cfg.BarWidth / 2)
	cr.SetLineJoin(d.cfg.LineJoin)
	cr.SetLineCap(d.cfg.LineCap)

	// TODO: maybe reduce lock contention somehow.
	d.drawState.Lock()
	defer d.drawState.Unlock()

	var (
		scale        = height / d.drawState.scale
		spaceWidth   = d.cfg.SpaceWidth * 2
		cPaddedWidth = (d.binWidth * float64(d.drawState.barCount*d.channels)) - spaceWidth
	)

	if cPaddedWidth > width || cPaddedWidth < 0 {
		cPaddedWidth = width
	}

	var (
		xCol  = (width - cPaddedWidth) / 2
		xBin  = 0
		delta = 1
	)

	for _, chBins := range d.drawState.barBufs {
		var (
			stop    = calculateBar(chBins[xBin]*scale, height, d.cfg.MinimumClamp)
			lCol    = xCol + d.cfg.BarWidth
			lColMax = xCol + (d.binWidth * float64(d.drawState.barCount)) - spaceWidth
		)

		for {
			if xCol >= lCol {
				if xCol >= lColMax {
					break
				}

				if xBin += delta; xBin >= d.drawState.barCount || xBin < 0 {
					break
				}

				stop = calculateBar(chBins[xBin]*scale, height, d.cfg.MinimumClamp)

				xCol += spaceWidth
				lCol = xCol + d.cfg.BarWidth
			}

			// Don't draw if stop is NaN for some reason.
			if !math.IsNaN(stop) {
				cr.SetSourceRGBA(d.fg[0], d.fg[1], d.fg[2], d.fg[3])
				cr.MoveTo(d.cfg.Offsets.apply(xCol, d.cfg.round(height-stop)))
				cr.LineTo(d.cfg.Offsets.apply(xCol, d.cfg.round(height)))
				cr.Stroke()
				cr.SetSourceRGBA(d.bg[0], d.bg[1], d.bg[2], d.bg[3])
			}

			xCol++
		}

		xCol += spaceWidth
		delta = -delta
	}
}

func calculateBar(value, height, clamp float64) float64 {
	bar := math.Max(math.Min(value, height), clamp) - clamp
	// Rescale the lost value.
	bar += bar * (clamp / height)

	return bar
}

// SetBackend overrides the given Backend in the config.
func (d *Drawer) SetBackend(backend input.Backend) {
	d.backend = backend
}

// SetDevice overrides the given Device in the config.
func (d *Drawer) SetDevice(device input.Device) {
	d.device = device
}

// Stop signals the event loop to stop. It does not block.
func (d *Drawer) Stop() {
	d.cancel()
}

// Start starts the area. This function blocks permanently until the audio loop
// is dead, so it should be called inside a goroutine. This function should not
// be called more than once, else it will panic.
//
// The loop will automatically close when the DrawingArea is destroyed.
func (d *Drawer) Start() error {
	if d.session != nil {
		// Panic is reasonable, as calling Start() multiple times (in multiple
		// goroutines) may cause undefined behaviors.
		panic("BUG: catnip.Area is already started.")
	}

	if d.backend == nil {
		backend, err := initBackend(d.cfg)
		if err != nil {
			return errors.Wrap(err, "failed to initialize input backend")
		}

		d.backend = backend
	}

	defer d.backend.Close()

	if d.device == nil {
		device, err := getDevice(d.backend, d.cfg)
		if err != nil {
			return err
		}
		d.device = device
	}

	session, err := d.backend.Start(input.SessionConfig{
		Device:     d.device,
		FrameSize:  d.channels,
		SampleSize: d.cfg.SampleSize,
		SampleRate: d.cfg.SampleRate,
	})
	if err != nil {
		return errors.Wrap(err, "failed to start the input backend")
	}

	d.session = session
	defer d.session.Stop()

	if err := session.Start(); err != nil {
		return errors.Wrap(err, "failed to start input session")
	}

	return d.start()
}

func (d *Drawer) start() error {
	var (
		fftBuf   = make([]complex128, d.cfg.SampleSize/2+1)
		spBinBuf = make(dsp.BinBuf, d.cfg.SampleSize)

		barBufs = d.makeBarBuf()
		plans   = make([]*fft.Plan, d.channels)
	)

	// DrawDelay is the time we wait between ticks to draw.
	var drawDelay = time.Second / time.Duration(d.cfg.SampleRate/float64(d.cfg.SampleSize))

	// Make a spectrum
	var spectrum = dsp.Spectrum{
		SampleRate: d.cfg.SampleRate,
		SampleSize: d.cfg.SampleSize,
		Bins:       spBinBuf,
	}

	spectrum.SetSmoothing(d.cfg.SmoothFactor / 100)
	spectrum.SetWinVar(d.cfg.WinVar)
	spectrum.SetType(d.cfg.SpectrumType)

	// Root Context

	var inputBufs = d.session.SampleBuffers()

	for idx, buf := range inputBufs {
		plans[idx] = &fft.Plan{
			Input:  buf,
			Output: fftBuf,
		}
	}

	var barCount int
	var peak float64

	var scale = d.cfg.Scaling.StaticScale
	var slowWindow, fastWindow *util.MovingWindow

	if scale == 0 {
		var (
			slowMax    = int(d.cfg.Scaling.SlowWindow*d.cfg.SampleRate) / d.cfg.SampleSize * 2
			fastMax    = int(d.cfg.Scaling.FastWindow*d.cfg.SampleRate) / d.cfg.SampleSize * 2
			windowData = make([]float64, slowMax+fastMax)
		)

		slowWindow = &util.MovingWindow{
			Data:     windowData[0:slowMax],
			Capacity: slowMax,
		}

		fastWindow = &util.MovingWindow{
			Data:     windowData[slowMax : slowMax+fastMax],
			Capacity: fastMax,
		}
	}

	var timer = time.NewTimer(drawDelay)
	defer timer.Stop()

	defer d.drawState.Invalidate()

	// Periodically queue redraw.
	ms := uint(drawDelay / time.Millisecond)
	timerHandle, _ := glib.TimeoutAdd(ms, func() bool {
		d.drawQ.QueueDraw()
		return true
	})
	defer glib.SourceRemove(timerHandle)

	for {
		if atomic.LoadUint32(&d.paused) == 1 {
			writeZeroBuf(inputBufs)

			// don't reset the numbers.

		} else {
			if d.session.ReadyRead() < d.cfg.SampleSize {
				continue
			}

			if err := d.session.Read(d.ctx); err != nil {
				return errors.Wrap(err, "failed to read audio input")
			}

			peak = 0
		}

		if barVar := d.bars(); barVar != barCount {
			barCount = spectrum.Recalculate(barVar)
		}

		for idx, buf := range barBufs {
			d.cfg.WindowFn(inputBufs[idx])
			plans[idx].Execute()
			spectrum.Process(buf, fftBuf)

			for _, v := range buf[:barCount] {
				if peak < v {
					peak = v
				}
			}
		}

		// We only need to check for one window to know the other is not nil.
		if (slowWindow != nil) && peak > 0 {
			// Set scale to a default 1.
			scale = 1

			fastWindow.Update(peak)
			var vMean, vSD = slowWindow.Update(peak)

			if length := slowWindow.Len(); length >= fastWindow.Cap() {
				if math.Abs(fastWindow.Mean()-vMean) > (d.cfg.Scaling.ResetDeviation * vSD) {
					vMean, vSD = slowWindow.Drop(int(float64(length) * d.cfg.Scaling.DumpPercent))
				}
			}

			if t := vMean + (1.5 * vSD); t > 1.0 {
				scale = t
			}
		}

		d.drawState.Set(barBufs, barCount, scale)

		select {
		case <-d.ctx.Done():
			return nil
		case <-timer.C:
			timer.Reset(drawDelay)
		}
	}
}

func writeZeroBuf(buf [][]input.Sample) {
	for i := range buf {
		for j := range buf[i] {
			buf[i][j] = 0
		}
	}
}

func (d *Drawer) makeBarBuf() [][]float64 {
	// Allocate a large slice with one large backing array.
	var fullBuf = make([]float64, d.channels*d.cfg.SampleSize)

	// Allocate smaller slice views.
	var barBufs = make([][]float64, d.channels)

	for idx := range barBufs {

		start := idx * d.cfg.SampleSize
		end := (idx + 1) * d.cfg.SampleSize

		barBufs[idx] = fullBuf[start:end]
	}

	return barBufs
}

// bars calculates the number of bars. It is thread-safe.
func (d *Drawer) bars() int {
	return int(d.drawState.Width() / d.binWidth / float64(d.channels))
}
