package lib

import (
	"math/rand/v2"
)

func NewUID() uint32 {
	return rand.Uint32()
}
