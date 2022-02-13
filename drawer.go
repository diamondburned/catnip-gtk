package catnip

import (
	"context"
	"image/color"
	"math"
	"sync"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v3"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
	"github.com/noriah/catnip/dsp"
	"github.com/noriah/catnip/fft"
	"github.com/noriah/catnip/input"

	catniputil "github.com/noriah/catnip/util"
)

type CairoColor [4]float64

func ColorFromGDK(rgba *gdk.RGBA) CairoColor {
	if rgba == nil {
		return CairoColor{}
	}
	return CairoColor{
		rgba.Red(),
		rgba.Green(),
		rgba.Blue(),
		rgba.Alpha(),
	}
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
	parent *gtk.Widget
	handle []glib.SignalHandle

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
	fftBuf   []complex128
	spectrum dsp.Spectrum

	slowWindow *catniputil.MovingWindow
	fastWindow *catniputil.MovingWindow

	background struct {
		surface *cairo.Surface
		width   float64
		height  float64
	}

	shared struct {
		sync.Mutex

		// Input buffers.
		readBuf  [][]input.Sample
		writeBuf [][]input.Sample

		// Output bars.
		barBufs [][]input.Sample

		cairoWidth float64
		barWidth   float64
		barCount   int
		scale      float64
		peak       float64
		quiet      int

		paused bool
	}
}

const (
	quietThreshold = 25
	peakThreshold  = 0.001
)

// NewDrawer creates a separated drawer state. The given drawQueuer will be
// called every redrawn frame.
func NewDrawer(widget gtk.Widgetter, cfg Config) *Drawer {
	ctx, cancel := context.WithCancel(context.Background())

	d := &Drawer{
		parent: gtk.BaseWidget(widget),
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

	w := gtk.BaseWidget(widget)

	d.handle = []glib.SignalHandle{
		w.Connect("draw", d.Draw),
		w.Connect("destroy", d.Stop),
		w.ConnectStyleUpdated(func() {
			// Invalidate the background.
			d.background.surface = nil

			styleCtx := w.StyleContext()
			transparent := gdk.NewRGBA(0, 0, 0, 0)

			d.fg = getColor(d.cfg.Colors.Foreground, styleCtx.Color(gtk.StateFlagNormal), d.fg)
			d.bg = getColor(d.cfg.Colors.Background, &transparent, d.bg)
		}),
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
		return ColorFromGDK(rgba)
	}

	return fallback
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
	for _, handle := range d.handle {
		d.parent.HandlerDisconnect(handle)
	}
}

// Draw is bound to the draw signal. Although Draw won't crash if Drawer is not
// started yet, the drawn result is undefined.
func (d *Drawer) Draw(widget gtk.Widgetter, cr *cairo.Context) {
	w := gtk.BaseWidget(widget)

	alloc := w.Allocation()
	width := float64(d.cfg.even(alloc.Width()))
	height := float64(d.cfg.even(alloc.Height()))

	cr.SetAntialias(d.cfg.AntiAlias)
	cr.SetLineWidth(d.cfg.BarWidth)
	cr.SetLineJoin(d.cfg.LineJoin)
	cr.SetLineCap(d.cfg.LineCap)

	cr.SetSourceRGBA(d.bg[0], d.bg[1], d.bg[2], d.bg[3])
	cr.Paint()

	if d.background.surface == nil || d.background.width != width || d.background.height != height {
		// Render the background onto the surface and use that as the source
		// surface for our context.
		surface := cr.GetTarget().CreateSimilar(cairo.CONTENT_COLOR_ALPHA, int(width), int(height))

		cr := cairo.Create(surface)

		// Draw the user-requested line color.
		cr.SetSourceRGBA(d.fg[0], d.fg[1], d.fg[2], d.fg[3])
		cr.Paint()

		// Draw the CSS background.
		gtk.RenderBackground(w.StyleContext(), cairo.Create(surface), 0, 0, width, height)

		d.background.width = width
		d.background.height = height
		d.background.surface = surface
	}

	cr.SetSourceSurface(d.background.surface, 0, 0)

	d.shared.Lock()
	defer d.shared.Unlock()

	d.shared.cairoWidth = width

	switch d.cfg.DrawStyle {
	case DrawVerticalBars:
		d.drawVertically(width, height, cr)
	case DrawHorizontalBars:
		d.drawHorizontally(width, height, cr)
	case DrawLines:
		d.drawLines(width, height, cr)
	}
}

func (d *Drawer) drawVertically(width, height float64, cr *cairo.Context) {
	bins := d.shared.barBufs
	center := (height - d.cfg.MinimumClamp) / 2
	scale := center / d.shared.scale

	if center < 0 {
		center = 0
	}

	// Round up the width so we don't draw a partial bar.
	xColMax := math.Round(width/d.binWidth) * d.binWidth

	// Calculate the starting position so it's in the middle.
	xCol := d.binWidth/2 + (width-xColMax)/2

	lBins := bins[0]
	rBins := bins[1%len(bins)]

	for xBin := 0; xBin < d.shared.barCount && xCol < xColMax; xBin++ {
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
	bins := d.shared.barBufs
	scale := height / d.shared.scale

	delta := 1

	// Round up the width so we don't draw a partial bar.
	xColMax := math.Round(width/d.binWidth) * d.binWidth

	xBin := 0
	xCol := (d.binWidth)/2 + (width-xColMax)/2

	for _, chBins := range bins {
		for xBin < d.shared.barCount && xBin >= 0 && xCol < xColMax {
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

func (d *Drawer) drawLines(width, height float64, cr *cairo.Context) {
	bins := d.shared.barBufs
	ceil := calculateBar(0, height, d.cfg.MinimumClamp)
	scale := height / d.shared.scale

	// Override the bar buffer with the scaled values.
	for _, ch := range bins {
		for bar := 0; bar < d.shared.barCount; bar++ {
			v := calculateBar(ch[bar]*scale, height, d.cfg.MinimumClamp)
			if math.IsNaN(v) {
				v = ceil
			}

			ch[bar] = v
		}
	}

	// Flip this to iterate backwards and draw the other channel.
	delta := +1

	// Round up the width so we don't draw a partial bar.
	xMax := math.Round(width/d.binWidth) * d.binWidth
	x := (d.binWidth)/2 + (width-xMax)/2

	// Move to the initial position at the bottom-left corner.
	// cr.MoveTo(d.cfg.Offsets.apply(x, height-d.cfg.MinimumClamp))

	var bar int
	var first bool

	for _, ch := range bins {
		// If we're iterating backwards, then check the lower bound, or
		// if we're iterating forwards, then check the upper bound.
		for bar >= 0 && bar < d.shared.barCount && x < xMax {
			y := ch[bar]

			if first {
				cr.MoveTo(x, y)
			} else {
				if next := bar + delta; next >= 0 && next < len(ch)-1 {
					// Average out the middle Y point with the next one for
					// smoothing.
					quadCurve(cr, x, y, x+d.binWidth, (y+ch[next])/2)
				} else {
					// Draw towards the last point.
					cr.LineTo(x, y)
				}
			}

			x += d.binWidth
			bar += delta
		}

		delta = -delta
		bar += delta
	}

	// Commit the line.
	cr.Stroke()
}

// quadCurve draws a quadratic bezier curve into the given Cairo context.
func quadCurve(t *cairo.Context, p1x, p1y, p2x, p2y float64) {
	p0x, p0y := t.GetCurrentPoint()

	// https://stackoverflow.com/a/55034115
	cp1x := p0x + ((2.0 / 3.0) * (p1x - p0x))
	cp1y := p0y + ((2.0 / 3.0) * (p1y - p0y))

	cp2x := p2x + ((2.0 / 3.0) * (p1x - p2x))
	cp2y := p2y + ((2.0 / 3.0) * (p1y - p2y))

	t.CurveTo(cp1x, cp1y, cp2x, cp2y, p2x, p2y)
}
