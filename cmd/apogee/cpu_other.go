//go:build !unix

package main

import "time"

// cpuTime is not measured on this platform: -cpulog then reports 0 %.
func cpuTime() time.Duration { return 0 }
