package lib

import (
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixMilli())
}

func NewUID() uint32 {
	return rand.Uint32()
}
