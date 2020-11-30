package catnip

import (
	"math"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/noriah/catnip/dsp"
	"github.com/noriah/catnip/fft"
	"github.com/noriah/catnip/input"
	"github.com/noriah/catnip/util"
	"github.com/pkg/errors"
)

// Start starts the area. This function blocks permanently until the audio loop
// is dead, so it should be called inside a goroutine. This function should not
// be called more than once, else it will panic.
//
// The loop will automatically close when the DrawingArea is destroyed.
func (d *Drawer) Start() error {
	if d.session != nil {
		// Panic is reasonable, as calling Start() multiple times (in multiple
		// goroutines) may cause undefined behaviors.
		panic("BUG: catnip.Area is already started.")
	}

	var (
		fftBuf   = make([]complex128, d.cfg.SampleSize/2+1)
		spBinBuf = make(dsp.BinBuf, d.cfg.SampleSize)
		plans    = make([]*fft.Plan, d.channels)
	)

	// DrawDelay is the time we wait between ticks to draw.
	var drawDelay = time.Second / time.Duration(d.cfg.SampleRate/float64(d.cfg.SampleSize))

	// Make a spectrum
	d.spectrum = dsp.Spectrum{
		SampleRate: d.cfg.SampleRate,
		SampleSize: d.cfg.SampleSize,
		Bins:       spBinBuf,
	}

	d.spectrum.SetSmoothing(d.cfg.SmoothFactor / 100)
	d.spectrum.SetType(d.cfg.SpectrumType)

	// Recreate

	if d.backend == nil {
		backend, err := initBackend(d.cfg)
		if err != nil {
			return errors.Wrap(err, "failed to initialize input backend")
		}

		d.backend = backend
	}

	defer d.backend.Close()

	if d.device == nil {
		device, err := getDevice(d.backend, d.cfg)
		if err != nil {
			return err
		}
		d.device = device
	}

	session, err := d.backend.Start(input.SessionConfig{
		Device:     d.device,
		FrameSize:  d.channels,
		SampleSize: d.cfg.SampleSize,
		SampleRate: d.cfg.SampleRate,
	})
	if err != nil {
		return errors.Wrap(err, "failed to start the input backend")
	}

	d.session = session
	defer d.session.Stop()

	if err := session.Start(); err != nil {
		return errors.Wrap(err, "failed to start input session")
	}

	var inputBufs = d.session.SampleBuffers()

	for idx, buf := range inputBufs {
		plans[idx] = &fft.Plan{
			Input:  buf,
			Output: fftBuf,
		}
	}

	var peak float64
	var slowWindow, fastWindow *util.MovingWindow

	d.scale = d.cfg.Scaling.StaticScale
	if d.scale == 0 {
		var (
			slowMax    = int(d.cfg.Scaling.SlowWindow*d.cfg.SampleRate) / d.cfg.SampleSize * 2
			fastMax    = int(d.cfg.Scaling.FastWindow*d.cfg.SampleRate) / d.cfg.SampleSize * 2
			windowData = make([]float64, slowMax+fastMax)
		)

		slowWindow = &util.MovingWindow{
			Data:     windowData[0:slowMax],
			Capacity: slowMax,
		}

		fastWindow = &util.MovingWindow{
			Data:     windowData[slowMax : slowMax+fastMax],
			Capacity: fastMax,
		}
	}

	var errCh = make(chan error, 1)

	// Periodically queue redraw.
	ms := uint(drawDelay / time.Millisecond)
	tk := time.NewTicker(drawDelay)

	timerHandle, _ := glib.TimeoutAdd(ms, func() bool {
		select {
		case <-tk.C:
			// continue
		default:
			// Not time yet. Continue.
			return true
		}

		if d.paused {
			writeZeroBuf(inputBufs)

		} else {
			if d.session.ReadyRead() < d.cfg.SampleSize {
				return true
			}

			if err := d.session.Read(d.ctx); err != nil {
				errCh <- errors.Wrap(err, "failed to read audio input")
				return false
			}

			peak = 0
		}

		for idx, buf := range d.barBufs {
			d.cfg.WindowFn(inputBufs[idx])
			plans[idx].Execute()
			d.spectrum.Process(buf, fftBuf)

			for _, v := range buf[:d.barCount] {
				if peak < v {
					peak = v
				}
			}
		}

		// We only need to check for one window to know the other is not nil.
		if slowWindow != nil && peak > 0 {
			// Set scale to a default 1.
			d.scale = 1

			fastWindow.Update(peak)
			var vMean, vSD = slowWindow.Update(peak)

			if length := slowWindow.Len(); length >= fastWindow.Cap() {
				if math.Abs(fastWindow.Mean()-vMean) > (d.cfg.Scaling.ResetDeviation * vSD) {
					vMean, vSD = slowWindow.Drop(int(float64(length) * d.cfg.Scaling.DumpPercent))
				}
			}

			if t := vMean + (1.5 * vSD); t > 1.0 {
				d.scale = t
			}
		}

		d.drawQ.QueueDraw()

		return true
	})

	defer tk.Stop()
	defer glib.SourceRemove(timerHandle)

	select {
	case <-d.ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
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
