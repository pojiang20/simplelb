package selector

import "sync/atomic"

type RoundR struct {
	current   uint64
	serverNum uint64
}

func (r *RoundR) NextIndex() int {
	return int(atomic.AddUint64(&r.current, uint64(1)) % r.serverNum)
}
