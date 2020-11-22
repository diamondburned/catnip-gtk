package catnip

import (
	"context"
	"image/color"
	"math"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/noriah/catnip/dsp"
	"github.com/noriah/catnip/input"
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

	backend  input.Backend
	device   input.Device
	session  input.Session
	spectrum dsp.Spectrum

	fg CairoColor
	bg CairoColor

	// total bar + space width
	binWidth float64
	// channels; 1 if monophonic
	channels int

	paused bool

	barBufs  [][]float64
	oldWidth float64
	barCount int
	scale    float64
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

	// Allocate a bar buffer.
	drawer.barBufs = drawer.makeBarBuf()

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

// ConnectDestroy connects the destroy signal to destroy the drawer as well. If
// this method is used, then the caller does not need to call Stop().
func (d *Drawer) ConnectDestroy(c Connector) (glib.SignalHandle, error) {
	return c.Connect("destroy", d.cancel)
}

// SetPaused will silent all inputs if true.
func (d *Drawer) SetPaused(paused bool) {
	d.paused = paused
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

	cr.SetLineWidth(d.cfg.BarWidth / 2)
	cr.SetLineJoin(d.cfg.LineJoin)
	cr.SetLineCap(d.cfg.LineCap)

	if width != d.oldWidth {
		d.barCount = d.spectrum.Recalculate(d.bars(width))
		d.oldWidth = width
	}

	switch d.cfg.Symmetry {
	case Vertical:
		d.drawVertically(width, height, cr)
	case Horizontal:
		d.drawHorizontally(width, height, cr)
	}
}

func (d *Drawer) drawBar(cr *cairo.Context, xCol, to, from float64) {
	cr.SetSourceRGBA(d.fg[0], d.fg[1], d.fg[2], d.fg[3])
	cr.MoveTo(d.cfg.Offsets.apply(xCol, d.cfg.round(from)))
	cr.LineTo(d.cfg.Offsets.apply(xCol, d.cfg.round(to)))
	cr.Stroke()
	cr.SetSourceRGBA(d.bg[0], d.bg[1], d.bg[2], d.bg[3])
}

func calculateBar(value, height, clamp float64) float64 {
	bar := math.Max(math.Min(value, height), clamp) - clamp
	// Rescale the lost value.
	bar += bar * (clamp / height)

	return height - bar
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
