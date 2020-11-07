package catnip

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gtk"
)

// minClamp is the minimum value for the visualizer before it is clamped to 0.
const minClamp = 1

type drawState struct {
	sync.Mutex

	barBufs  [][]float64
	barCount int
	scale    float64

	width uint32
}

func (s *drawState) SetWidth(width int) {
	atomic.StoreUint32(&s.width, uint32(width))
}

func (s *drawState) Width() float64 {
	return float64(atomic.LoadUint32(&s.width))
}

func (s *drawState) Set(buf [][]float64, bars int, scale float64) {
	s.Lock()
	defer s.Unlock()

	for i := range buf {
		copy(s.barBufs[i][:bars], buf[i][:bars])
	}

	s.barCount = bars
	s.scale = scale
}

func (s *drawState) Invalidate() {
	s.Lock()
	defer s.Unlock()

	s.barBufs = nil
	s.barCount = 0
	s.scale = 0
}

// draw is bound to the draw signal.
func (a *Area) draw(_ *gtk.DrawingArea, cr *cairo.Context) {
	var (
		width  = float64(a.GetAllocatedWidth())
		height = float64(a.GetAllocatedHeight())
	)

	cr.SetSourceRGBA(a.bg[0], a.bg[1], a.bg[2], a.bg[3])
	cr.SetLineWidth(float64(a.cfg.BarWidth) / 2)
	cr.SetLineCap(cairo.LINE_CAP_SQUARE)

	// TODO: maybe reduce lock contention somehow.
	a.drawState.Lock()
	defer a.drawState.Unlock()

	var (
		scale        = height / a.drawState.scale
		spaceWidth   = a.cfg.SpaceWidth * 2
		cPaddedWidth = (a.binWidth * float64(a.drawState.barCount*a.channels)) - spaceWidth
	)

	if cPaddedWidth > width || cPaddedWidth < 0 {
		cPaddedWidth = width
	}

	var (
		xCol  = (width - cPaddedWidth) / 2
		xBin  = 0
		delta = 1
	)

	for _, chBins := range a.drawState.barBufs {
		var (
			stop    = calculateBar(chBins[xBin]*scale, height)
			lCol    = xCol + a.cfg.BarWidth
			lColMax = xCol + (a.binWidth * float64(a.drawState.barCount)) - spaceWidth
		)

		for {
			if xCol >= lCol {
				if xCol >= lColMax {
					break
				}

				if xBin += delta; xBin >= a.drawState.barCount || xBin < 0 {
					break
				}

				stop = calculateBar(chBins[xBin]*scale, height)

				xCol += spaceWidth
				lCol = xCol + a.cfg.BarWidth
			}

			cr.SetSourceRGBA(a.fg[0], a.fg[1], a.fg[2], a.fg[3])
			cr.MoveTo(xCol, height-stop)
			cr.LineTo(xCol, height)
			cr.Stroke()
			cr.SetSourceRGBA(a.bg[0], a.bg[1], a.bg[2], a.bg[3])

			xCol++
		}

		xCol += spaceWidth
		delta = -delta
	}
}

func calculateBar(value, height float64) float64 {
	return math.Max(math.Min(value, height), minClamp) - minClamp
}
