package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/moby/term"
	"github.com/noyzilla/oops/internal/docker"
	"github.com/spf13/cobra"
)

type containerStatsItem struct {
	ContainerID string
	Name        string
	CPUPerc     string
	MemLimit    string
	MemUsage    string
	MemPerc     string
	NetRx       string
	NetTx       string
	BlockIn     string
	BlockOut    string
	Pids        string
}

type cpuSample struct {
	totalUsage  uint64
	systemUsage uint64
}

func newStatsCmd() *cobra.Command {
	var noStream bool
	var showAll bool

	cmd := &cobra.Command{
		Use:   "stats [targets...]",
		Short: "Display resource usage statistics for containers",
		Long:  "Display a formatted, real-time stream of container resource usage statistics (CPU %, MEM LM, MEM US, MEM %, NET RX, NET TX, BLOCK RD, BLOCK WR, PIDs) with right-aligned metrics and 2-decimal formatting.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStats(cmd, args, noStream, showAll)
		},
	}

	cmd.Flags().BoolVar(&noStream, "no-stream", false, "Disable streaming stats and only pull the first result")
	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all running containers on daemon (default filters active box)")

	return cmd
}

func calculateCPUPercent(stats *container.StatsResponse, prev *cpuSample) float64 {
	var (
		cpuDelta    float64
		systemDelta float64
	)

	if prev != nil && prev.systemUsage > 0 {
		cpuDelta = float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(prev.totalUsage)
		systemDelta = float64(stats.CPUStats.SystemUsage) - float64(prev.systemUsage)
	} else if stats.PreCPUStats.SystemUsage > 0 {
		cpuDelta = float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
		systemDelta = float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)
	}

	onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0.0 {
		onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if onlineCPUs == 0.0 {
		onlineCPUs = 1.0
	}

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		return (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}
	return 0.0
}

func calculateMemUsage(mem container.MemoryStats) (float64, float64, float64) {
	usage := float64(mem.Usage)
	// Deduct page cache / inactive_file following docker stats convention
	if v, ok := mem.Stats["inactive_file"]; ok && mem.Usage >= v {
		usage = float64(mem.Usage - v)
	} else if v, ok := mem.Stats["total_inactive_file"]; ok && mem.Usage >= v {
		usage = float64(mem.Usage - v)
	}

	limit := float64(mem.Limit)
	var perc float64
	if limit > 0 {
		perc = (usage / limit) * 100.0
	}
	return usage, limit, perc
}

func calculateNetwork(networks map[string]container.NetworkStats) (float64, float64) {
	var rx, tx float64
	for _, v := range networks {
		rx += float64(v.RxBytes)
		tx += float64(v.TxBytes)
	}
	return rx, tx
}

func calculateBlockIO(blkio container.BlkioStats) (float64, float64) {
	var read, write float64
	for _, entry := range blkio.IoServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			read += float64(entry.Value)
		case "write":
			write += float64(entry.Value)
		}
	}
	return read, write
}

// formatBinary formats byte sizes (base 1024) with minimum unit KB and exactly 2 decimal places (e.g. "256.00 MB", "  2.44 GB", "  0.00 KB").
func formatBinary(b float64) string {
	if b < 0 {
		b = 0
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	unitIdx := 0
	val := b / 1024.0
	for val >= 1024.0 && unitIdx < len(units)-1 {
		val /= 1024.0
		unitIdx++
	}
	return fmt.Sprintf("%6.2f %s", val, units[unitIdx])
}

// formatDecimal formats byte sizes (base 1000) with minimum unit KB and exactly 2 decimal places (e.g. "  3.29 MB", "352.26 KB", "  0.00 KB").
func formatDecimal(b float64) string {
	if b < 0 {
		b = 0
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	unitIdx := 0
	val := b / 1000.0
	for val >= 1000.0 && unitIdx < len(units)-1 {
		val /= 1000.0
		unitIdx++
	}
	return fmt.Sprintf("%6.2f %s", val, units[unitIdx])
}

func renderStatsTable(out io.Writer, items []containerStatsItem) {
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%8s\t%9s\t%9s\t%8s\t%9s\t%9s\t%9s\t%9s\t%5s\n",
		"CONTAINER ID",
		"NAME",
		"CPU %",
		"MEM LM",
		"MEM US",
		"MEM %",
		"NET RX",
		"NET TX",
		"BLOCK RD",
		"BLOCK WR",
		"PIDS",
	)
	for _, it := range items {
		fmt.Fprintf(w, "%s\t%s\t%8s\t%9s\t%9s\t%8s\t%9s\t%9s\t%9s\t%9s\t%5s\n",
			it.ContainerID,
			it.Name,
			it.CPUPerc,
			it.MemLimit,
			it.MemUsage,
			it.MemPerc,
			it.NetRx,
			it.NetTx,
			it.BlockIn,
			it.BlockOut,
			it.Pids,
		)
	}
	w.Flush()
}

func fetchSingleContainerStats(ctx context.Context, dockerCli *client.Client, c container.Summary, prevSample *cpuSample) (*containerStatsItem, *cpuSample, error) {
	resp, err := dockerCli.ContainerStats(ctx, c.ID, false)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var stats container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, nil, err
	}

	cpuPercent := calculateCPUPercent(&stats, prevSample)
	memUsage, memLimit, memPercent := calculateMemUsage(stats.MemoryStats)
	rx, tx := calculateNetwork(stats.Networks)
	blkRead, blkWrite := calculateBlockIO(stats.BlkioStats)

	cName := ""
	if len(c.Names) > 0 {
		cName = strings.TrimPrefix(c.Names[0], "/")
	}
	if cName == "" && stats.Name != "" {
		cName = strings.TrimPrefix(stats.Name, "/")
	}

	pidsStr := fmt.Sprintf("%d", stats.PidsStats.Current)
	if stats.PidsStats.Current == 0 && stats.NumProcs > 0 {
		pidsStr = fmt.Sprintf("%d", stats.NumProcs)
	}

	memLimitStr := "-"
	if memLimit > 0 {
		memLimitStr = formatBinary(memLimit)
	}

	item := &containerStatsItem{
		ContainerID: shortContainerID(c.ID),
		Name:        cName,
		CPUPerc:     fmt.Sprintf("%.2f %%", cpuPercent),
		MemLimit:    memLimitStr,
		MemUsage:    formatBinary(memUsage),
		MemPerc:     fmt.Sprintf("%.2f %%", memPercent),
		NetRx:       formatDecimal(rx),
		NetTx:       formatDecimal(tx),
		BlockIn:     formatDecimal(blkRead),
		BlockOut:    formatDecimal(blkWrite),
		Pids:        pidsStr,
	}

	newSample := &cpuSample{
		totalUsage:  stats.CPUStats.CPUUsage.TotalUsage,
		systemUsage: stats.CPUStats.SystemUsage,
	}

	return item, newSample, nil
}

func filterRunningContainers(containers []container.Summary, workDir string, args []string, showAll bool) []container.Summary {
	var targetMap map[string]bool
	if len(args) > 0 {
		resolved, err := docker.ResolveTargets(workDir, args)
		if err == nil && len(resolved) > 0 {
			targetMap = make(map[string]bool)
			for _, r := range resolved {
				targetMap[r.ContainerName] = true
				targetMap[r.ServiceName] = true
				targetMap["/"+r.ServiceName] = true
			}
		}
	}

	var filtered []container.Summary
	for _, c := range containers {
		if c.State != "running" {
			continue
		}

		cName := ""
		if len(c.Names) > 0 {
			cName = strings.TrimPrefix(c.Names[0], "/")
		}
		svcName := c.Labels["com.docker.compose.service"]
		stackName := extractContainerStackName(c.Labels, cName)

		if len(args) > 0 {
			matched := false
			if targetMap != nil {
				matched = targetMap[cName] || targetMap[svcName] || targetMap[c.ID[:min(12, len(c.ID))]]
			}
			if !matched {
				for _, arg := range args {
					argClean := strings.TrimPrefix(arg, "/")
					if strings.EqualFold(cName, argClean) ||
						strings.EqualFold(svcName, argClean) ||
						strings.HasPrefix(c.ID, arg) ||
						strings.Contains(strings.ToLower(cName), strings.ToLower(argClean)) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		} else if !showAll {
			// If not --all, check if container belongs to this workspace
			if stackName == "%" && !strings.HasPrefix(cName, "oops-tunnel-") && cName != "oops" && cName != "caddy-proxy" {
				continue
			}
		}

		filtered = append(filtered, c)
	}

	// Fallback: if box filter yielded nothing but containers are running, show all running
	if len(filtered) == 0 && len(args) == 0 && len(containers) > 0 {
		for _, c := range containers {
			if c.State == "running" {
				filtered = append(filtered, c)
			}
		}
	}

	return filtered
}

func runStats(cmd *cobra.Command, args []string, noStream bool, showAll bool) error {
	workDir := ResolveWorkDir(targetDir)
	_ = ValidateActiveBox(workDir)

	dockerCli, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed connecting to Docker daemon: %w", err)
	}
	defer dockerCli.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	isTerminal := term.IsTerminal(os.Stdout.Fd())
	prevCPU := make(map[string]*cpuSample)
	firstRender := true

	for {
		containers, err := dockerCli.ContainerList(ctx, container.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed listing Docker containers: %w", err)
		}

		targetContainers := filterRunningContainers(containers, workDir, args, showAll)
		if len(targetContainers) == 0 {
			if noStream {
				fmt.Fprintln(os.Stdout, "No running containers found.")
				return nil
			}
			if isTerminal {
				fmt.Print("\033[H\033[2J")
			}
			fmt.Fprintln(os.Stdout, "No running containers found.")
		} else {
			var wg sync.WaitGroup
			type result struct {
				idx  int
				item *containerStatsItem
				cpu  *cpuSample
				id   string
			}
			resChan := make(chan result, len(targetContainers))

			for i, c := range targetContainers {
				wg.Add(1)
				go func(idx int, ctr container.Summary) {
					defer wg.Done()
					item, newCPU, err := fetchSingleContainerStats(ctx, dockerCli, ctr, prevCPU[ctr.ID])
					if err == nil && item != nil {
						resChan <- result{idx: idx, item: item, cpu: newCPU, id: ctr.ID}
					}
				}(i, c)
			}
			wg.Wait()
			close(resChan)

			var items []containerStatsItem
			for r := range resChan {
				items = append(items, *r.item)
				if r.cpu != nil {
					prevCPU[r.id] = r.cpu
				}
			}

			sort.Slice(items, func(i, j int) bool {
				return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
			})

			if !firstRender && isTerminal && !noStream {
				fmt.Print("\033[H\033[2J")
			}
			renderStatsTable(os.Stdout, items)
			firstRender = false
		}

		if noStream || !isTerminal {
			return nil
		}

		select {
		case <-sigChan:
			fmt.Println()
			return nil
		case <-time.After(1 * time.Second):
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
