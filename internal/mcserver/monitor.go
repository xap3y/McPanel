package mcserver

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// SampleResource samples CPU% and RSS for the given process and all its descendants.
func SampleResource(root int) (ramBytes uint64, cpuPct float64) {
	pids := getTree(root)

	if runtime.GOOS == "linux" {
		for _, p := range pids {
			ramBytes += readRSS(p)
		}
		t1, total1 := readTotalCPUTicks(pids)
		time.Sleep(200 * time.Millisecond)
		t2, total2 := readTotalCPUTicks(pids)
		if total2 > total1 {
			cpuPct = 100 * float64(t2-t1) / float64(total2-total1)
		}
		return
	}

	// macOS / Unix fallback
	for _, p := range pids {
		r, c := samplePS(p)
		ramBytes += r
		cpuPct += c
	}
	return
}

func getTree(root int) []int {
	out, err := exec.Command("ps", "-e", "-o", "pid=,ppid=").Output()
	if err != nil {
		return []int{root}
	}

	children := make(map[int][]int)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			pid, _ := strconv.Atoi(fields[0])
			ppid, _ := strconv.Atoi(fields[1])
			children[ppid] = append(children[ppid], pid)
		}
	}

	var result []int
	var walk func(int)
	walk = func(p int) {
		result = append(result, p)
		for _, child := range children[p] {
			walk(child)
		}
	}
	walk(root)
	return result
}

func samplePS(pid int) (ramBytes uint64, cpuPct float64) {
	out, err := exec.Command("ps", "-o", "rss=,pcpu=", "-p", fmt.Sprintf("%d", pid)).Output()
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) >= 2 {
		if kb, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
			ramBytes = kb * 1024
		}
		if pct, err := strconv.ParseFloat(fields[1], 64); err == nil {
			cpuPct = pct
		}
	}
	return
}

func readRSS(pid int) uint64 {
	pidStr := strconv.Itoa(pid)
	b, err := os.ReadFile("/proc/" + pidStr + "/status")
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

func readTotalCPUTicks(pids []int) (uint64, uint64) {
	var totalUtimeStime uint64
	for _, p := range pids {
		pidStr := strconv.Itoa(p)
		b, err := os.ReadFile("/proc/" + pidStr + "/stat")
		if err == nil {
			fields := strings.Fields(string(b))
			if len(fields) >= 15 {
				utime, _ := strconv.ParseUint(fields[13], 10, 64)
				stime, _ := strconv.ParseUint(fields[14], 10, 64)
				totalUtimeStime += utime + stime
			}
		}
	}

	sb, err := os.ReadFile("/proc/stat")
	if err != nil {
		return totalUtimeStime, 0
	}
	line := strings.SplitN(string(sb), "\n", 2)[0]
	cpuFields := strings.Fields(line)[1:]
	var total uint64
	for _, f := range cpuFields {
		v, _ := strconv.ParseUint(f, 10, 64)
		total += v
	}
	return totalUtimeStime, total
}
