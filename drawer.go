package catnip

import (
	"context"
	"image/color"
	"math"
	"sync"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/noriah/catnip/dsp"
	"github.com/noriah/catnip/fft"
	"github.com/noriah/catnip/input"

	catniputil "github.com/noriah/catnip/util"
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

	fg CairoColor
	bg CairoColor

	// total bar + space width
	binWidth float64
	// channels; 1 if monophonic
	channels int

	backend  input.Backend
	device   input.Device
	inputCfg input.SessionConfig

	fftPlans []*fft.Plan
	fftBufs  [][]complex128
	barBufs  [][]input.Sample
	spectrum dsp.Spectrum

	slowWindow *catniputil.MovingWindow
	fastWindow *catniputil.MovingWindow

	oldWidth float64
	scale    float64
	barCount int

	shared struct {
		sync.Mutex
		paused bool
		reproc bool

		// Input buffers.
		readBuf  [][]input.Sample
		writeBuf [][]input.Sample
	}
}

// NewDrawer creates a separated drawer state. The given drawQueuer will be
// called every redrawn frame.
func NewDrawer(drawQ DrawQueuer, cfg Config) *Drawer {
	ctx, cancel := context.WithCancel(context.Background())

	d := &Drawer{
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
		d.channels = 1
	}

	return d
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
	Connect(string, interface{}) glib.SignalHandle
}

var _ Connector = (*gtk.Widget)(nil)

// ConnectDraw connects the given connector to the Draw method.
func (d *Drawer) ConnectDraw(c Connector) glib.SignalHandle {
	return c.Connect("draw", d.Draw)
}

// ConnectDestroy connects the destroy signal to destroy the drawer as well. If
// this method is used, then the caller does not need to call Stop().
func (d *Drawer) ConnectDestroy(c Connector) glib.SignalHandle {
	return c.Connect("destroy", func(c Connector) { d.cancel() })
}

// SetPaused will silent all inputs if true.
func (d *Drawer) SetPaused(paused bool) {
	d.shared.Lock()
	d.shared.paused = paused
	d.shared.Unlock()
}

// AllocatedSizeGetter is any widget that can be obtained dimensions of. This is
// used for the Draw method.
type AllocatedSizeGetter interface {
	GetAllocatedWidth() int
	GetAllocatedHeight() int
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

// Draw is bound to the draw signal. Although Draw won't crash if Drawer is not
// started yet, the drawn result is undefined.
func (d *Drawer) Draw(w AllocatedSizeGetter, cr *cairo.Context) {
	// now := time.Now()
	// defer func() {
	// 	log.Printf(
	// 		"visualizer took %12dus to draw\n", time.Now().Sub(now).Microseconds())
	// }()

	var (
		width  = float64(d.cfg.even(w.GetAllocatedWidth()))
		height = float64(d.cfg.even(w.GetAllocatedHeight()))
	)

	cr.SetSourceRGBA(d.bg[0], d.bg[1], d.bg[2], d.bg[3])

	// cr.Save()
	// defer cr.Restore()

	cr.SetAntialias(d.cfg.AntiAlias)
	cr.SetLineWidth(d.cfg.BarWidth)
	cr.SetLineJoin(d.cfg.LineJoin)
	cr.SetLineCap(d.cfg.LineCap)

	if d.oldWidth != width {
		d.oldWidth = width
		d.barCount = d.spectrum.Recalculate(d.bars(width))
	}

	cr.SetSourceRGBA(d.fg[0], d.fg[1], d.fg[2], d.fg[3])

	switch d.cfg.Symmetry {
	case Vertical:
		d.drawVertically(width, height, cr)
	case Horizontal:
		d.drawHorizontally(width, height, cr)
	}
}

func (d *Drawer) drawVertically(width, height float64, cr *cairo.Context) {
	bins := d.barBufs
	center := (height - d.cfg.MinimumClamp) / 2
	scale := center / d.scale

	if center < 0 {
		center = 0
	}

	// Round up the width so we don't draw a partial bar.
	xColMax := math.Round(width/d.binWidth) * d.binWidth

	// Calculate the starting position so it's in the middle.
	xCol := d.binWidth/2 + (width-xColMax)/2

	lBins := bins[0]
	rBins := bins[1%len(bins)]

	for xBin := 0; xBin < d.barCount && xCol < xColMax; xBin++ {
		lStop := calculateBar(lBins[xBin]*scale, center, d.cfg.MinimumClamp)
		rStop := calculateBar(rBins[xBin]*scale, center, d.cfg.MinimumClamp)

		if !math.IsNaN(lStop) && !math.IsNaN(rStop) {
			d.drawBar(cr, xCol, lStop, height-rStop)
		} else if d.cfg.MinimumClamp > 0 {
			d.drawBar(cr, xCol, center, center+d.cfg.MinimumClamp)
		}

		xCol += d.binWidth
	}
}

func (d *Drawer) drawHorizontally(width, height float64, cr *cairo.Context) {
	bins := d.barBufs
	scale := height / d.scale

	delta := 1

	// Round up the width so we don't draw a partial bar.
	xColMax := math.Round(width/d.binWidth) * d.binWidth

	xBin := 0
	xCol := (d.binWidth)/2 + (width-xColMax)/2

	for _, chBins := range bins {
		for xBin < d.barCount && xBin >= 0 && xCol < xColMax {
			stop := calculateBar(chBins[xBin]*scale, height, d.cfg.MinimumClamp)

			// Don't draw if stop is NaN for some reason.
			if !math.IsNaN(stop) {
				d.drawBar(cr, xCol, height, stop)
			}

			xCol += d.binWidth
			xBin += delta
		}

		delta = -delta
		xBin += delta // ensure xBin is not out of bounds first.
	}
}

func (d *Drawer) drawBar(cr *cairo.Context, xCol, to, from float64) {
	cr.MoveTo(d.cfg.Offsets.apply(xCol, d.cfg.round(from)))
	cr.LineTo(d.cfg.Offsets.apply(xCol, d.cfg.round(to)))
	cr.Stroke()
}

func calculateBar(value, height, clamp float64) float64 {
	bar := math.Max(math.Min(value, height), clamp) - clamp
	// Rescale the lost value.
	bar += bar * (clamp / height)

	return height - bar
}
