// Package model embeds the trained model so the binary works offline.
// Rebuild greeklish.bin with "make model".
package model

import (
	_ "embed"
	"sync"

	"github.com/alex20465/greeklish"
)

//go:embed greeklish.bin
var data []byte

// Load decodes the embedded model once and returns it.
var Load = sync.OnceValues(func() (*greeklish.Model, error) {
	m := &greeklish.Model{}
	return m, m.UnmarshalBinary(data)
})
