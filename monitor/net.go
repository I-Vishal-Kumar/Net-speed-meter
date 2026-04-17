package monitor

import (
	"errors"
	"time"

	psnet "github.com/shirou/gopsutil/v3/net"
)

// Sample is a point-in-time snapshot of a single interface's byte counters.
type Sample struct {
	BytesRecv uint64
	BytesSent uint64
	At        time.Time
	Iface     string
	Valid     bool
}

// Speed holds the computed download/upload rate in bytes per second
// along with the raw byte deltas observed since the previous sample.
type Speed struct {
	Download   float64
	Upload     float64
	DeltaRecv  uint64
	DeltaSent  uint64
	Iface      string
	Valid      bool
}

// Snapshot takes an initial reading used as the baseline for the first Poll call.
// Returns a Sample with Valid=false if no active interface could be read.
func Snapshot() Sample {
	name, counter, err := activeCounters()
	if err != nil {
		return Sample{At: time.Now(), Iface: "unknown"}
	}
	return Sample{
		BytesRecv: counter.BytesRecv,
		BytesSent: counter.BytesSent,
		At:        time.Now(),
		Iface:     name,
		Valid:     true,
	}
}

// Poll computes speed since prev and returns a new sample to use next iteration.
// The returned Speed has Valid=false when the current reading failed or when the
// active interface changed (in which case deltas would be meaningless).
func Poll(prev Sample) (Speed, Sample) {
	name, counter, err := activeCounters()
	now := time.Now()
	if err != nil {
		return Speed{Iface: "unknown"}, Sample{At: now, Iface: "unknown"}
	}

	next := Sample{
		BytesRecv: counter.BytesRecv,
		BytesSent: counter.BytesSent,
		At:        now,
		Iface:     name,
		Valid:     true,
	}

	// If we have no valid baseline or the interface changed, emit a zero-speed
	// sample but a valid baseline for the next tick.
	if !prev.Valid || prev.Iface != name {
		return Speed{Iface: name}, next
	}

	elapsed := now.Sub(prev.At).Seconds()
	if elapsed < 0.01 {
		elapsed = 1
	}

	var dRecv, dSent uint64
	if counter.BytesRecv >= prev.BytesRecv {
		dRecv = counter.BytesRecv - prev.BytesRecv
	}
	if counter.BytesSent >= prev.BytesSent {
		dSent = counter.BytesSent - prev.BytesSent
	}

	return Speed{
		Download:  float64(dRecv) / elapsed,
		Upload:    float64(dSent) / elapsed,
		DeltaRecv: dRecv,
		DeltaSent: dSent,
		Iface:     name,
		Valid:     true,
	}, next
}

// activeCounters picks the most-active non-loopback UP interface and returns its counters.
func activeCounters() (string, psnet.IOCountersStat, error) {
	ifaces, err := psnet.Interfaces()
	if err != nil {
		return "", psnet.IOCountersStat{}, err
	}

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
		return "", psnet.IOCountersStat{}, errors.New("no active interface")
	}

	counters, err := psnet.IOCounters(true)
	if err != nil {
		return "", psnet.IOCountersStat{}, err
	}

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
		for name := range candidates {
			if c, ok := counterMap[name]; ok {
				return name, c, nil
			}
		}
		return "", psnet.IOCountersStat{}, errors.New("no counters for active interface")
	}

	return bestName, counterMap[bestName], nil
}
