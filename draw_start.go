package catnip

import (
	"math"
	"time"

	"github.com/gotk3/gotk3/glib"
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
	if d.inputBuf != nil {
		// Panic is reasonable, as calling Start() multiple times (in multiple
		// goroutines) may cause undefined behaviors.
		panic("BUG: catnip.Area is already started.")
	}

	d.spectrum = dsp.Spectrum{
		SampleRate: d.cfg.SampleRate,
		SampleSize: d.cfg.SampleSize,
		Bins:       make(dsp.BinBuf, d.cfg.SampleSize),
	}
	d.spectrum.SetSmoothing(d.cfg.SmoothFactor / 100)
	d.spectrum.SetType(d.cfg.SpectrumType)

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

	if d.backend == nil {
		d.backend, err = initBackend(d.cfg)
		if err != nil {
			return errors.Wrap(err, "failed to initialize input backend")
		}
	}
	defer d.backend.Close()

	if d.device == nil {
		d.device, err = getDevice(d.backend, d.cfg)
		if err != nil {
			return err
		}
	}

	sessionConfig := input.SessionConfig{
		Device:     d.device,
		FrameSize:  int(d.channels),
		SampleSize: d.cfg.SampleSize,
		SampleRate: d.cfg.SampleRate,
	}

	session, err := d.backend.Start(sessionConfig)
	if err != nil {
		return errors.Wrap(err, "failed to start the input backend")
	}

	// Free up the device.
	d.device = nil
	sessionConfig.Device = nil

	d.fftBuf = make([]complex128, d.cfg.SampleSize/2+1)
	d.fftPlans = make([]*fft.Plan, d.channels)

	// Allocate buffers.
	d.inputBuf = input.MakeBuffers(sessionConfig)
	d.shared.barBufRead = d.makeBarBuf()
	d.shared.barBufWrite = d.makeBarBuf()

	for idx, buf := range d.inputBuf {
		d.fftPlans[idx] = &fft.Plan{
			Input:  buf,
			Output: d.fftBuf,
		}
	}

	// DrawDelay is the time we wait between ticks to draw.
	var drawDelay = time.Second / time.Duration(d.cfg.SampleRate/float64(d.cfg.SampleSize))

	// Periodically queue redraw.
	ms := uint(drawDelay / time.Millisecond)
	timerHandle := glib.TimeoutAddPriority(ms, glib.PRIORITY_HIGH_IDLE, func() bool {
		d.shared.Lock()
		peaked := d.shared.peak > 0.01
		defer d.shared.Unlock()

		// Only queue draw if we have a peak noticeable enough.
		if peaked {
			d.drawQ.QueueDraw()
		}

		return true
	})

	defer glib.SourceRemove(timerHandle)

	// Write to inputBufWrite, and we can copy from write to read (see Process).
	if err := session.Start(d.ctx, d.inputBuf, d); err != nil {
		return errors.Wrap(err, "failed to start input session")
	}

	return nil
}

func (d *Drawer) Process() {
	d.shared.Lock()
	paused := d.shared.paused
	barCount := d.shared.barCount
	d.shared.Unlock()

	if paused {
		writeZeroBuf(d.inputBuf)
	}

	peak := 0.0
	scale := 1.0

	for idx, buf := range d.shared.barBufWrite {
		d.cfg.WindowFn(d.inputBuf[idx])
		d.fftPlans[idx].Execute() // process into buf
		d.spectrum.Process(buf, d.fftBuf)

		for _, v := range buf[:barCount] {
			if peak < v {
				peak = v
			}
		}
	}

	// We only need to check for one window to know the other is not nil.
	if d.slowWindow != nil && peak > 0.01 {
		d.fastWindow.Update(peak)
		var vMean, vSD = d.slowWindow.Update(peak)

		if length := d.slowWindow.Len(); length >= d.fastWindow.Cap() {
			if math.Abs(d.fastWindow.Mean()-vMean) > (d.cfg.Scaling.ResetDeviation * vSD) {
				vMean, vSD = d.slowWindow.Drop(int(float64(length) * d.cfg.Scaling.DumpPercent))
			}
		}

		if t := vMean + (1.5 * vSD); t > 1.0 {
			scale = t
		}
	}

	d.shared.Lock()

	d.shared.peak = peak
	d.shared.scale = scale
	input.CopyBuffers(d.shared.barBufRead, d.shared.barBufWrite)

	d.shared.Unlock()
}

func writeZeroBuf(buf [][]input.Sample) {
	for i := range buf {
		for j := range buf[i] {
			buf[i][j] = 0
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
func (d *Drawer) bars(width float64) int {
	var bars = float64(width) / d.binWidth

	if d.cfg.Symmetry == Horizontal {
		bars /= float64(d.channels)
	}

	return int(bars)
}
