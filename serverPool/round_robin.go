package serverPool

import (
	"sync/atomic"
)

func init() {
	Algmap[ROUNDR] = &RoundR{}
}

type RoundR struct {
	current   uint64
	serverNum uint64
}

func (r *RoundR) NextIndex() int {
	return int(atomic.AddUint64(&r.current, uint64(1)) % r.serverNum)
}

func (r *RoundR) LenInc() {
	r.serverNum++
}

func (r *RoundR) LenDec() {
	r.serverNum--
}
