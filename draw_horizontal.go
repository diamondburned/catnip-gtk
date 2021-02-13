package catnip

import (
	"math"

	"github.com/gotk3/gotk3/cairo"
)

func (d *Drawer) drawHorizontally(width, height float64, cr *cairo.Context) {
	bins := d.shared.barBufRead
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
