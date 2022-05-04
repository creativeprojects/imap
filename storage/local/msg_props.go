package local

import "time"

type msgProps struct {
	Size  uint32
	Flags []string
	Hash  []byte
	Date  time.Time
}
