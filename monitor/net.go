package monitor

import (
	"time"

	psnet "github.com/shirou/gopsutil/v3/net"
)

// Sample is a point-in-time snapshot of a single interface's byte counters.
type Sample struct {
	BytesRecv uint64
	BytesSent uint64
	At        time.Time
	Iface     string
}

// Speed holds the computed download/upload rate in bytes per second.
type Speed struct {
	Download float64
	Upload   float64
	Iface    string
}

// Snapshot takes an initial reading used as the baseline for the first Poll call.
func Snapshot() Sample {
	name, counter := activeCounters()
	return Sample{
		BytesRecv: counter.BytesRecv,
		BytesSent: counter.BytesSent,
		At:        time.Now(),
		Iface:     name,
	}
}

// Poll computes speed since prev and returns a new sample to use next iteration.
func Poll(prev Sample) (Speed, Sample) {
	name, counter := activeCounters()
	elapsed := time.Since(prev.At).Seconds()
	if elapsed < 0.01 {
		elapsed = 1
	}

	var dl, ul float64
	// Clamp to 0 in case of counter reset or interface switch
	if counter.BytesRecv >= prev.BytesRecv {
		dl = float64(counter.BytesRecv-prev.BytesRecv) / elapsed
	}
	if counter.BytesSent >= prev.BytesSent {
		ul = float64(counter.BytesSent-prev.BytesSent) / elapsed
	}

	next := Sample{
		BytesRecv: counter.BytesRecv,
		BytesSent: counter.BytesSent,
		At:        time.Now(),
		Iface:     name,
	}
	return Speed{Download: dl, Upload: ul, Iface: name}, next
}

// activeCounters picks the most-active non-loopback UP interface and returns its counters.
func activeCounters() (string, psnet.IOCountersStat) {
	ifaces, err := psnet.Interfaces()
	if err != nil {
		return "unknown", psnet.IOCountersStat{}
	}

	// Collect names of interfaces that are up, not loopback, and have an address.
	candidates := make(map[string]bool)
	for _, iface := range ifaces {
		isUp, isLoopback := false, false
		for _, f := range iface.Flags {
			if f == "up" {
				isUp = true
			}
			if f == "loopback" {
				isLoopback = true
			}
		}
		if isUp && !isLoopback && len(iface.Addrs) > 0 {
			candidates[iface.Name] = true
		}
	}

	if len(candidates) == 0 {
		return "unknown", psnet.IOCountersStat{}
	}

	counters, err := psnet.IOCounters(true)
	if err != nil {
		return "unknown", psnet.IOCountersStat{}
	}

	// Pick the candidate with the highest total traffic (most active).
	var bestName string
	var bestTraffic uint64
	counterMap := make(map[string]psnet.IOCountersStat)
	for _, c := range counters {
		counterMap[c.Name] = c
		if candidates[c.Name] {
			total := c.BytesRecv + c.BytesSent
			if total > bestTraffic {
				bestTraffic = total
				bestName = c.Name
			}
		}
	}

	if bestName == "" {
		// Fallback: first candidate
		for name := range candidates {
			if c, ok := counterMap[name]; ok {
				return name, c
			}
		}
		return "unknown", psnet.IOCountersStat{}
	}

	return bestName, counterMap[bestName]
}
