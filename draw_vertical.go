package catnip

import (
	"math"

	"github.com/gotk3/gotk3/cairo"
)

func (d *Drawer) drawVertically(width, height float64, cr *cairo.Context) {
	bins := d.shared.barBufRead
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
