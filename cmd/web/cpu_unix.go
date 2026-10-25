//go:build unix

package main

import "syscall"

// readProcessCPUSeconds returns the process CPU time (user plus system) in
// seconds. The Go runtime's /cpu/classes metrics are not populated on every
// platform, notably macOS, so use the OS accounting directly on Unix.
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
