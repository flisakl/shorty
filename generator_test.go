package shorty

import (
	"math"
	"slices"
	"sync"
	"testing"
)


func getGeneratorSize(symbolsLength int, sequenceLength int) int {
	return int(math.Pow(float64(symbolsLength), float64(sequenceLength)))
}

func exhaustGenerator(sg Generator) []string {
	ret := make([]string, 0)
	for {
		v := sg.Get()
		if v != "" {
			ret = append(ret, v)
		} else {
			return ret
		}
	}
}

func TestBasicGeneratorGet(t *testing.T) {
	g := NewBasicGenerator("AB", 2)

	var ok string
	if ok = g.Get(); ok == "" {
		t.Error("Failed to get first value from new basic generator")
	}

	g.Update("BA")

	if ok = g.Get(); ok == "" {
		t.Error("Failed to increment offset to last possible value")
	}

	if ok := g.Get(); ok != "" {
		t.Error("Exhausted generator should return an empty string")
	}
}

func TestBasicGeneratorUpdate(t *testing.T) {
	g := NewBasicGenerator("AB", 2)

	if err := g.Update("BA"); err != nil {
		t.Error("Failed to update basic generator")
	}

	if err := g.Update("BB"); err == nil {
		t.Error("Generator should not be able to update with last possible sequence")
	}

}

func TestNumberOfGeneratedSequences(t *testing.T) {
	var cases = []struct { 
		msg, sym string
		length int
	}{
		{msg: "Length: 3 Symbols: 2", sym: "AB", length: 3,},
		{msg: "Length: 4 Symbols: 3", sym: "ABC", length: 4,},
		{msg: "Length: 5 Symbols: 4", sym: "ABC0", length: 5,},
	}

	for _, c := range cases {
		t.Run(c.msg, func(t *testing.T) {
			gen := NewBasicGenerator(c.sym, c.length)
			items := exhaustGenerator(gen)
			count := len(items)


			expected := getGeneratorSize(len(c.sym), c.length)
			if count != expected {
				t.Errorf("Expected number of sequences: %d, got %d", expected, count)
			} else {
				slices.Sort(items)
				for j := 1; j < len(items); j++ {
					if items[j] == items[j - 1] {
						t.Error("Found duplicate in the sequence")
						return
					}
				}
			}
		})
	}
}

func TestSynchronizedGenerator(t *testing.T) {
	const (
		symbols = "ABC"
		length  = 3
		runs    = 26
	)

	var gen Generator
	gen = NewSynchronizedGenerator(
		NewBasicGenerator(symbols, length),
	)

	results := make(chan string, runs)

	var wg sync.WaitGroup

	for i := 0; i < runs; i++ {
		wg.Go(func() {
			results <- gen.Get()
		})
	}

	wg.Wait()
	close(results)

	seen := make(map[string]bool)

	for result := range results {
		if seen[result] {
			t.Errorf("duplicate result: %s", result)
		}
		seen[result] = true
	}

	if len(seen) != runs {
		t.Errorf("expected %d unique results, got %d", runs, len(seen))
	}

	result := gen.Get()
	if result != "CCC" {
		t.Errorf("expected %s, got %s", "CCC", result)
	}
}
