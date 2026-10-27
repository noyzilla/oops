package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestCalculateCPUPercent(t *testing.T) {
	stats := &container.StatsResponse{
		CPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage: 200000000,
			},
			SystemUsage: 2000000000,
			OnlineCPUs:  2,
		},
		PreCPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage: 100000000,
			},
			SystemUsage: 1000000000,
		},
	}

	// cpuDelta = 100M, systemDelta = 1000M (1B) -> ratio = 0.1 * 2 cpus * 100 = 20.00%
	perc := calculateCPUPercent(stats, nil)
	expected := 20.0
	if perc != expected {
		t.Errorf("calculateCPUPercent() = %v, expected %v", perc, expected)
	}

	// Test with previous sample override
	prev := &cpuSample{
		totalUsage:  150000000,
		systemUsage: 1500000000,
	}
	// cpuDelta = 50M, systemDelta = 500M -> ratio = 0.1 * 2 cpus * 100 = 20.00%
	percPrev := calculateCPUPercent(stats, prev)
	if percPrev != expected {
		t.Errorf("calculateCPUPercent() with prev = %v, expected %v", percPrev, expected)
	}
}

func TestCalculateMemUsage(t *testing.T) {
	mem := container.MemoryStats{
		Usage: 104857600, // 100 MiB
		Limit: 209715200, // 200 MiB
		Stats: map[string]uint64{
			"inactive_file": 4194304, // 4 MiB cache
		},
	}

	usage, limit, perc := calculateMemUsage(mem)
	expectedUsage := float64(104857600 - 4194304) // 96 MiB
	expectedLimit := float64(209715200)           // 200 MiB
	expectedPerc := (expectedUsage / expectedLimit) * 100.0

	if usage != expectedUsage {
		t.Errorf("calculateMemUsage() usage = %v, expected %v", usage, expectedUsage)
	}
	if limit != expectedLimit {
		t.Errorf("calculateMemUsage() limit = %v, expected %v", limit, expectedLimit)
	}
	if perc != expectedPerc {
		t.Errorf("calculateMemUsage() perc = %v, expected %v", perc, expectedPerc)
	}
}

func TestCalculateNetworkAndBlockIO(t *testing.T) {
	networks := map[string]container.NetworkStats{
		"eth0": {RxBytes: 1024, TxBytes: 2048},
		"eth1": {RxBytes: 512, TxBytes: 256},
	}
	rx, tx := calculateNetwork(networks)
	if rx != 1536 || tx != 2304 {
		t.Errorf("calculateNetwork() = (%v, %v), expected (1536, 2304)", rx, tx)
	}

	blkio := container.BlkioStats{
		IoServiceBytesRecursive: []container.BlkioStatEntry{
			{Op: "Read", Value: 4096},
			{Op: "Write", Value: 8192},
			{Op: "read", Value: 1024},
		},
	}
	read, write := calculateBlockIO(blkio)
	if read != 5120 || write != 8192 {
		t.Errorf("calculateBlockIO() = (%v, %v), expected (5120, 8192)", read, write)
	}
}

func TestRenderStatsTable(t *testing.T) {
	items := []containerStatsItem{
		{
			ContainerID: "ddadb950dd83",
			Name:        "oops",
			CPUPerc:     "0.00%",
			MemUsage:    "5.258MiB",
			MemLimit:    "128MiB",
			MemPerc:     "4.11%",
			NetIO:       "11.6kB / 12.6kB",
			BlockIO:     "0B / 0B",
			Pids:        "6",
		},
		{
			ContainerID: "98ba9bf3e428",
			Name:        "caddy-proxy",
			CPUPerc:     "0.32%",
			MemUsage:    "50.14MiB",
			MemLimit:    "256MiB",
			MemPerc:     "19.58%",
			NetIO:       "81.7MB / 88.7MB",
			BlockIO:     "180kB / 8.19kB",
			Pids:        "10",
		},
	}

	var buf bytes.Buffer
	renderStatsTable(&buf, items)
	out := buf.String()

	// Verify header contains individual MEM USAGE and MEM LIMIT columns
	header := "CONTAINER ID   NAME          CPU %   MEM USAGE   MEM LIMIT   MEM %    NET I/O           BLOCK I/O        PIDS"
	if !strings.Contains(out, "MEM USAGE") || !strings.Contains(out, "MEM LIMIT") {
		t.Errorf("renderStatsTable header missing MEM USAGE or MEM LIMIT: \n%s", out)
	}
	if !strings.Contains(out, "ddadb950dd83") || !strings.Contains(out, "5.258MiB") || !strings.Contains(out, "128MiB") {
		t.Errorf("renderStatsTable missing oops row data: \n%s", out)
	}
	_ = header
}
