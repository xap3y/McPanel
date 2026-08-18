package mcserver

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// SampleResource reads /proc/<pid>/stat twice with a short delay to
// compute instantaneous CPU%, plus RSS from /proc/<pid>/status. Called
// on an adaptive interval by the caller - not by this package - so it
// never runs on its own timer.
func SampleResource(pid int) (ramBytes uint64, cpuPct float64) {
	ramBytes = readRSS(pid)

	t1, total1 := readCPUTicks(pid)
	time.Sleep(200 * time.Millisecond)
	t2, total2 := readCPUTicks(pid)

	if total2 > total1 {
		cpuPct = 100 * float64(t2-t1) / float64(total2-total1)
	}
	return
}

func readRSS(pid int) uint64 {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				return kb * 1024
			}
		}
	}
	return 0
}

// readCPUTicks returns (processTicks, systemTotalTicks) for a rough
// CPU% estimate between two samples.
func readCPUTicks(pid int) (uint64, uint64) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(string(b))
	if len(fields) < 15 {
		return 0, 0
	}
	utime, _ := strconv.ParseUint(fields[13], 10, 64)
	stime, _ := strconv.ParseUint(fields[14], 10, 64)

	sb, err := os.ReadFile("/proc/stat")
	if err != nil {
		return utime + stime, 0
	}
	line := strings.SplitN(string(sb), "\n", 2)[0]
	cpuFields := strings.Fields(line)[1:]
	var total uint64
	for _, f := range cpuFields {
		v, _ := strconv.ParseUint(f, 10, 64)
		total += v
	}
	return utime + stime, total
}