package app

import (
	"fmt"
	"testing"

	"github.com/tyzrex/kport"
)

// BenchmarkExpandTargets measures the plan's socket grouping cost as the
// listener snapshot grows, without depending on machine-specific open ports.
func BenchmarkExpandTargets(b *testing.B) {
	for _, count := range []int{10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("sockets_%d", count), func(b *testing.B) {
			all := make([]killport.Process, count)
			for i := range all {
				pid := i/4 + 100
				all[i] = killport.Process{PID: pid, StartID: fmt.Sprint(pid), Port: 1000 + i, Protocol: killport.ProtocolTCP}
			}
			targets := all[:min(4, len(all))]
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = ExpandTargets(targets, all)
			}
		})
	}
}
