package catnip

import (
	"sync"
	"sync/atomic"
)

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
