//go:build !unix

package main

import "runtime/metrics"

// readProcessCPUSeconds falls back to the runtime CPU counters on platforms
// without getrusage. Unsupported counters (all zero) are reported as
// unavailable rather than as an idle process.
func readProcessCPUSeconds() (float64, bool) {
	values := []metrics.Sample{
		{Name: "/cpu/classes/total:cpu-seconds"},
		{Name: "/cpu/classes/idle:cpu-seconds"},
	}
	metrics.Read(values)
	if values[0].Value.Kind() != metrics.KindFloat64 || values[1].Value.Kind() != metrics.KindFloat64 {
		return 0, false
	}
	total := values[0].Value.Float64()
	idle := values[1].Value.Float64()
	if total == 0 && idle == 0 {
		return 0, false
	}
	seconds := total - idle
	if seconds < 0 {
		return 0, false
	}
	return seconds, true
}
