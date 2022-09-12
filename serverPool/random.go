package serverPool

import (
	"math/rand"
)

func init() {
	Algmap[RANDOM] = &Random{}
}

type Random struct {
	serverNum int64
}

func (r *Random) NextIndex() int {
	return rand.Intn(int(r.serverNum))
}

func (r *Random) LenInc() {
	r.serverNum++
}

func (r *Random) LenDec() {

}
