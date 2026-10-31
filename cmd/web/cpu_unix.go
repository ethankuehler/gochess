//go:build unix

package main

import "syscall"

// Use getrusage because runtime CPU metrics are unavailable on some Unix systems.
func readProcessCPUSeconds() (float64, bool) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, false
	}
	seconds := float64(usage.Utime.Sec) + float64(usage.Utime.Usec)/1e6 +
		float64(usage.Stime.Sec) + float64(usage.Stime.Usec)/1e6
	if seconds < 0 {
		return 0, false
	}
	return seconds, true
}
