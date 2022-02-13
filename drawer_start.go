package catnip

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/noriah/catnip/dsp"
	"github.com/noriah/catnip/fft"
	"github.com/noriah/catnip/input"
	"github.com/pkg/errors"

	catniputil "github.com/noriah/catnip/util"
)

// Start starts the area. This function blocks permanently until the audio loop
// is dead, so it should be called inside a goroutine. This function should not
// be called more than once, else it will panic.
//
// The loop will automatically close when the DrawingArea is destroyed.
func (d *Drawer) Start() (err error) {
	if d.shared.barBufs != nil {
		// Panic is reasonable, as calling Start() multiple times (in multiple
		// goroutines) may cause undefined behaviors.
		panic("BUG: catnip.Area is already started.")
	}

	if d.backend == nil {
		d.backend, err = d.cfg.InitBackend()
		if err != nil {
			return errors.Wrap(err, "failed to initialize input backend")
		}
	}
	defer d.backend.Close()

	if d.device == nil {
		d.device, err = d.cfg.InitDevice(d.backend)
		if err != nil {
			return err
		}
	}

	d.shared.scale = d.cfg.Scaling.StaticScale
	if d.shared.scale == 0 {
		var (
			slowMax    = int(d.cfg.Scaling.SlowWindow*d.cfg.SampleRate) / d.cfg.SampleSize * 2
			fastMax    = int(d.cfg.Scaling.FastWindow*d.cfg.SampleRate) / d.cfg.SampleSize * 2
			windowData = make([]float64, slowMax+fastMax)
		)

		d.slowWindow = &catniputil.MovingWindow{
			Data:     windowData[0:slowMax],
			Capacity: slowMax,
		}

		d.fastWindow = &catniputil.MovingWindow{
			Data:     windowData[slowMax : slowMax+fastMax],
			Capacity: fastMax,
		}
	}

	d.spectrum = dsp.Spectrum{
		SampleRate: d.cfg.SampleRate,
		SampleSize: d.cfg.SampleSize,
		Bins:       make([]dsp.Bin, d.cfg.SampleSize),
	}
	d.spectrum.SetSmoothing(d.cfg.SmoothFactor / 100)

	sessionConfig := input.SessionConfig{
		Device:     d.device,
		FrameSize:  int(d.channels),
		SampleSize: d.cfg.SampleSize,
		SampleRate: d.cfg.SampleRate,
	}

	// Allocate buffers.
	d.reallocBarBufs()
	d.reallocFFTBufs()
	d.reallocSpectrumOldValues()
	d.shared.readBuf = input.MakeBuffers(sessionConfig)
	d.shared.writeBuf = input.MakeBuffers(sessionConfig)

	// Initialize the FFT plans.
	d.fftPlans = make([]*fft.Plan, d.channels)
	for idx := range d.fftPlans {
		plan := fft.Plan{
			Input:  d.shared.readBuf[idx],
			Output: d.fftBuf,
		}
		plan.Init()
		d.fftPlans[idx] = &plan
	}

	// Signal the backend to start listening to the microphone.
	session, err := d.backend.Start(sessionConfig)
	if err != nil {
		return errors.Wrap(err, "failed to start the input backend")
	}

	// Free up the device.
	d.device = nil

	// Periodically queue redraw. Note that this is never a perfect rounding:
	// inputting 60Hz will trigger a redraw every 16ms, which is 62.5Hz.
	ms := 1000 / uint(d.cfg.DrawOptions.FrameRate)
	timerHandle := glib.TimeoutAddPriority(ms, glib.PriorityDefault, func() bool {
		if d.processBars() {
			d.parent.QueueDraw()
		}
		return true
	})

	defer glib.SourceRemove(timerHandle)

	// Write to writeBuf, and we can copy from write to read (see Process).
	if err := session.Start(d.ctx, d.shared.writeBuf, d); err != nil {
		return errors.Wrap(err, "failed to start input session")
	}

	return nil
}

// Process processes the internal read buffer and analyzes its spectrum.
func (d *Drawer) Process() {
	d.shared.Lock()
	defer d.shared.Unlock()

	if d.shared.paused {
		writeZeroBuf(d.shared.readBuf)
	} else {
		input.CopyBuffers(d.shared.readBuf, d.shared.writeBuf)
	}
}

func (d *Drawer) processBars() bool {
	d.shared.Lock()
	defer d.shared.Unlock()

	d.shared.peak = 0

	if d.shared.cairoWidth != d.shared.barWidth {
		d.shared.barWidth = d.shared.cairoWidth
		d.shared.barCount = d.spectrum.Recalculate(d.bars(d.shared.barWidth))
	}

	for idx, buf := range d.shared.barBufs {
		d.cfg.WindowFn(d.shared.readBuf[idx])
		d.fftPlans[idx].Execute() // process from readBuf into buf

		for bIdx := range buf[:d.shared.barCount] {
			v := d.spectrum.ProcessBin(idx, bIdx, d.fftBuf)
			buf[bIdx] = v

			if d.shared.peak < v {
				d.shared.peak = v
			}
		}
	}

	if d.slowWindow != nil {
		fastMean, _ := d.fastWindow.Update(d.shared.peak)
		slowMean, slowStddev := d.slowWindow.Update(d.shared.peak)

		if length := d.slowWindow.Len(); length >= d.fastWindow.Cap() {
			if math.Abs(fastMean-slowMean) > (d.cfg.Scaling.ResetDeviation * slowStddev) {
				count := int(float64(length) * d.cfg.Scaling.DumpPercent)
				slowMean, slowStddev = d.slowWindow.Drop(count)
			}
		}

		d.shared.scale = 1
		if t := slowMean + (1.5 * slowStddev); t > 1.0 {
			d.shared.scale = t
		}
	}

	// Draw if peak is over the threshold.
	if d.shared.peak > peakThreshold {
		d.shared.quiet = 0
		return true
	}

	// If we're not over the threshold, then draw until we're quiet for a while.
	if d.shared.quiet < quietThreshold {
		d.shared.quiet++
		return true
	}

	return false
}

var zeroSamples = make([]input.Sample, 512)

func writeZeroBuf(buf [][]input.Sample) {
	for i := range buf {
		// Copy zeroSamples into buf[i] in 512-byte chunks. Go's copy() supports
		// SIMD on most CPUs, so this should be faster than a traditional loop.
		for n := 0; n < len(buf[i]); n += copy(buf[i][n:], zeroSamples) {
		}
	}
}

func (d *Drawer) reallocBarBufs() {
	d.shared.barBufs = allocBarBufs(d.cfg.SampleSize, d.channels)
}

func (d *Drawer) reallocSpectrumOldValues() {
	d.spectrum.OldValues = allocBarBufs(d.cfg.SampleSize, d.channels)
}

func allocBarBufs(sampleSize, channels int) [][]float64 {
	// Allocate a large slice with one large backing array.
	fullBuf := make([]float64, channels*sampleSize)

	// Allocate smaller slice views.
	barBufs := make([][]float64, channels)

	for idx := range barBufs {
		start := idx * sampleSize
		end := (idx + 1) * sampleSize

		barBufs[idx] = fullBuf[start:end]
	}

	return barBufs
}

func (d *Drawer) reallocFFTBufs() {
	d.fftBuf = make([]complex128, d.cfg.SampleSize/2+1)
}

// bars calculates the number of bars. It is thread-safe.
func (d *Drawer) bars(width float64) int {
	var bars = float64(width) / d.binWidth

	if !d.cfg.Monophonic && d.cfg.DrawStyle != DrawHorizontalBars {
		bars /= float64(d.channels)
	}

	return int(math.Ceil(bars))
}
