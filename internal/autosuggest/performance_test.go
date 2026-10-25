package autosuggest

import (
	"fmt"
	"testing"
)

func BenchmarkHistoryAdd(b *testing.B) {
	e := &Engine{historyM: make(map[string]struct{}), maxItems: 2000}
	for i := 0; i < 2000; i++ {
		e.Add(fmt.Sprintf("command %d", i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Add("command 1000")
	}
}
