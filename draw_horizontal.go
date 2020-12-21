package catnip

import (
	"math"

	"github.com/gotk3/gotk3/cairo"
)

func (d *Drawer) drawHorizontally(width, height float64, cr *cairo.Context) {
	var (
		scale        = height / d.shared.scale
		spaceWidth   = d.cfg.SpaceWidth * 2
		cPaddedWidth = (d.binWidth * float64(d.shared.barCount*d.channels)) - spaceWidth
	)

	if cPaddedWidth > width || cPaddedWidth < 0 {
		cPaddedWidth = width
	}

	var (
		xCol  = (width - cPaddedWidth) / 2
		xBin  = 0
		delta = 1
	)

	for _, chBins := range d.shared.barBufRead {
		var (
			stop    = calculateBar(chBins[xBin]*scale, height, d.cfg.MinimumClamp)
			lCol    = xCol + d.cfg.BarWidth
			lColMax = xCol + (d.binWidth * float64(d.shared.barCount)) - spaceWidth
		)

		for {
			if xCol >= lCol {
				if xCol >= lColMax {
					break
				}

				if xBin += delta; xBin >= d.shared.barCount || xBin < 0 {
					break
				}

				stop = calculateBar(chBins[xBin]*scale, height, d.cfg.MinimumClamp)

				xCol += spaceWidth
				lCol = xCol + d.cfg.BarWidth
			}

			// Don't draw if stop is NaN for some reason.
			if !math.IsNaN(stop) {
				d.drawBar(cr, xCol, height, stop)
			}

			xCol++
		}

		xCol += spaceWidth
		delta = -delta
	}
}
