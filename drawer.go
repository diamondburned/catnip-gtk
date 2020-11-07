package catnip

import (
	"context"
	"image/color"
	"math"
	"time"

	"github.com/gotk3/gotk3/cairo"
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

// minClamp is the minimum value for the visualizer before it is clamped to 0.
const minClamp = 1

type cairoColor [4]float64

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
	session input.Session

	fg cairoColor
	bg cairoColor

	// total bar + space width
	binWidth float64
	// channels; 1 if monophonic
	channels int

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

		fg: getColor(cfg.ForegroundColor, nil, cairoColor{0, 0, 0, 1}),
		bg: getColor(cfg.BackgroundColor, nil, cairoColor{0, 0, 0, 0}),

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
func getColor(c color.Color, rgba *gdk.RGBA, fallback cairoColor) (cairoC cairoColor) {
	if c != nil {
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

// SetWidgetStyle lets the Drawer take the given widgetStyler's styles.
func (d *Drawer) SetWidgetStyle(widgetStyler StyleContexter) {
	styleCtx, _ := widgetStyler.GetStyleContext()

	d.fg = getColor(d.cfg.ForegroundColor, styleCtx.GetColor(gtk.STATE_FLAG_NORMAL), d.fg)
	d.bg = getColor(d.cfg.BackgroundColor, gdk.NewRGBA(0, 0, 0, 0), d.bg)
}

// Connector is the interface to connect any widget.
type Connector interface {
	gtk.IWidget
	AllocatedSizeGetter
	Connect(string, interface{}, ...interface{}) (glib.SignalHandle, error)
}

var _ Connector = (*gtk.Widget)(nil)

// ConnectDraw connects the given connector to the Draw method.
func (d *Drawer) ConnectDraw(c Connector) {
	c.Connect("draw", d.Draw)
}

// ConnectSizeAllocate connects the given connector to update the width of the
// drawer, and consequently, the number of bars drawn.
func (d *Drawer) ConnectSizeAllocate(c Connector) {
	c.Connect("size-allocate", func() {
		d.drawState.SetWidth(c.GetAllocatedWidth())
	})
}

// ConnectDestroy connects the destroy signal to destroy the drawer as well. If
// this method is used, then the caller does not need to call Stop().
func (d *Drawer) ConnectDestroy(c Connector) {
	c.Connect("destroy", d.cancel)
}

// AllocatedSizeGetter is any widget that can be obtained dimensions of. This is
// used for the Draw method.
type AllocatedSizeGetter interface {
	GetAllocatedWidth() int
	GetAllocatedHeight() int
}

// draw is bound to the draw signal.
func (d *Drawer) Draw(w AllocatedSizeGetter, cr *cairo.Context) {
	var (
		width  = float64(w.GetAllocatedWidth())
		height = float64(w.GetAllocatedHeight())
	)

	cr.SetSourceRGBA(d.bg[0], d.bg[1], d.bg[2], d.bg[3])
	cr.SetLineWidth(float64(d.cfg.BarWidth) / 2)
	cr.SetLineCap(cairo.LINE_CAP_SQUARE)

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
			stop    = calculateBar(chBins[xBin]*scale, height)
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

				stop = calculateBar(chBins[xBin]*scale, height)

				xCol += spaceWidth
				lCol = xCol + d.cfg.BarWidth
			}

			cr.SetSourceRGBA(d.fg[0], d.fg[1], d.fg[2], d.fg[3])
			cr.MoveTo(xCol, height-stop)
			cr.LineTo(xCol, height)
			cr.Stroke()
			cr.SetSourceRGBA(d.bg[0], d.bg[1], d.bg[2], d.bg[3])

			xCol++
		}

		xCol += spaceWidth
		delta = -delta
	}
}

func calculateBar(value, height float64) float64 {
	return math.Max(math.Min(value, height), minClamp) - minClamp
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

	backend, err := initBackend(d.cfg)
	if err != nil {
		return errors.Wrap(err, "failed to initialize input backend")
	}

	d.backend = backend
	defer d.backend.Close()

	device, err := getDevice(backend, d.cfg)
	if err != nil {
		return err
	}

	var sessConfig = input.SessionConfig{
		Device:     device,
		FrameSize:  d.channels,
		SampleSize: d.cfg.SampleSize,
		SampleRate: d.cfg.SampleRate,
	}

	session, err := backend.Start(sessConfig)
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

		slowMax    = int(d.cfg.Scaling.SlowWindow*d.cfg.SampleRate) / d.cfg.SampleSize * 2
		fastMax    = int(d.cfg.Scaling.FastWindow*d.cfg.SampleRate) / d.cfg.SampleSize * 2
		windowData = make([]float64, slowMax+fastMax)

		barBufs = d.makeBarBuf()
		plans   = make([]*fft.Plan, d.channels)
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
	var peak, scale float64

	var timer = time.NewTimer(drawDelay)
	defer timer.Stop()

	defer d.drawState.Invalidate()

	// Periodically queue redraw.
	glib.TimeoutAdd(uint(drawDelay/time.Millisecond), func() bool {
		d.drawQ.QueueDraw()

		select {
		case <-d.ctx.Done():
			return false
		default:
			return true
		}
	})

	for {
		if d.session.ReadyRead() < d.cfg.SampleSize {
			continue
		}

		if err := d.session.Read(d.ctx); err != nil {
			return errors.Wrap(err, "failed to read audio input")
		}

		if barVar := d.bars(); barVar != barCount {
			barCount = spectrum.Recalculate(barVar)
		}

		peak = 0
		scale = 0

		for idx, buf := range barBufs {
			window.CosSum(inputBufs[idx], d.cfg.WinVar)
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
