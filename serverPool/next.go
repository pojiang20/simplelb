package serverPool

type Next interface {
	NextIndex() int
	LenInc()
	LenDec()
}
