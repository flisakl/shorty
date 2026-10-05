package shorty

import (
	"errors"
	"sync"
)

type Generator interface {
	// Get returns an unique sequence of fixed length or empty string if generator
	// has been exhausted. 
	Get() string
	// Update changes the generator's internal state using provided string. The purpose
	// of this method is to allow generating sequence from a certain point. Error is
	// returned if operation fails.
	Update(lastSequence string) error
}

var Alphanumeric = "qwfpbjluyarstgmneioxcdvzkhQWFPBJLUYARSTGMNEIOXCDVZKH0123456789"
var Alpha = "qwfpbjluyarstgmneioxcdvzkhQWFPBJLUYARSTGMNEIOXCDVZKH"
var Numeric = "0123456789"


type BasicGenerator struct {
	symbols  string
	offsets  []uint8
	output   []byte
	maxValue uint8
	exhausted bool
}

func NewBasicGenerator(symbols string, length int) *BasicGenerator {
	return &BasicGenerator{
		symbols:  symbols,
		offsets:  make([]uint8, length),
		output:   make([]byte, length),
		maxValue: uint8(len(symbols) - 1),
	}
}

func (gen *BasicGenerator) Get() string {
	if gen.exhausted { return "" }

	for i, offset := range gen.offsets {
		gen.output[i] = gen.symbols[offset]
	}

	result := string(gen.output)

	for i := len(gen.offsets) - 1; i >= 0; i-- {
		if gen.offsets[i] == gen.maxValue {
			gen.offsets[i] = 0
		} else {
			gen.offsets[i]++
			return result
		}
	}

	gen.exhausted = true
	return result
}

var UnknownSymbolError = errors.New("shorty: unknown symbol")
var GeneratorExhaustedError = errors.New("shorty: generator exhausted")

func (gen *BasicGenerator) Update(last string) error {
	indexes := make(map[byte]uint8)
	for idx, r := range gen.symbols {
		indexes[byte(r)] = uint8(idx)
	}

	for idx, r := range last {
		off, ok := indexes[byte(r)]
		if ok {
			gen.offsets[idx] = off
		} else {
			return UnknownSymbolError
		}
	}

	// Update offsets
	for i := len(gen.offsets) - 1; i >= 0; i-- {
		if gen.offsets[i] == gen.maxValue {
			gen.offsets[i] = 0
		} else {
			gen.offsets[i]++
			return nil
		}
	}

	return GeneratorExhaustedError
}

// SynchronizedGenerator implements Generator interface and provides a safe way
// to call Generator methods in concurrent environment.
type SynchronizedGenerator struct {
	mu sync.Mutex
	inner Generator
}

func NewSynchronizedGenerator(gen Generator) *SynchronizedGenerator {
	return &SynchronizedGenerator{mu: sync.Mutex{}, inner: gen}
}

func (gen *SynchronizedGenerator) Get() string {
	gen.mu.Lock()
	defer gen.mu.Unlock()
	return gen.inner.Get()	
}

func (gen *SynchronizedGenerator) Update(lastSequence string) error {
	gen.mu.Lock()
	defer gen.mu.Unlock()
	return gen.inner.Update(lastSequence)
}
