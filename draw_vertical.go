package catnip

import (
	"math"

	"github.com/gotk3/gotk3/cairo"
)

func (d *Drawer) drawVertically(width, height float64, cr *cairo.Context) {
	var (
		bins = d.shared.barBufRead

		center     = (height - d.cfg.MinimumClamp) / 2
		scale      = center / d.shared.scale
		spaceWidth = d.cfg.SpaceWidth * 2
	)

	if center < 0 {
		center = 0
	}

	var (
		xCol = (width - ((d.binWidth * float64(d.shared.barCount*d.channels)) - spaceWidth)) / 2
		xBin = 0
	)

	if xCol < 0 {
		xCol = 0
	}

	var (
		lCol    = xCol + d.cfg.BarWidth
		lColMax = xCol + (d.binWidth * float64(d.shared.barCount)) - spaceWidth
	)

	var (
		lBins = bins[0]
		rBins = bins[1%len(bins)]

		lStop = calculateBar(lBins[xBin]*scale, center, d.cfg.MinimumClamp)
		rStop = calculateBar(rBins[xBin]*scale, center, d.cfg.MinimumClamp)
	)

	for {
		if xCol >= lCol {
			if xCol >= lColMax {
				break
			}

			if xBin++; xBin >= d.shared.barCount || xBin < 0 {
				break
			}

			lStop = calculateBar(lBins[xBin]*scale, center, d.cfg.MinimumClamp)
			rStop = calculateBar(rBins[xBin]*scale, center, d.cfg.MinimumClamp)

			xCol += spaceWidth
			lCol = xCol + d.cfg.BarWidth
		}

		if !math.IsNaN(lStop) && !math.IsNaN(rStop) {
			d.drawBar(cr, xCol, lStop, height-rStop)
		} else if d.cfg.MinimumClamp > 0 {
			d.drawBar(cr, xCol, center, center+d.cfg.MinimumClamp)
		}

		xCol++
	}
}
