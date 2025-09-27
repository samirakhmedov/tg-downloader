package repository

import (
	"fmt"
	"tg-downloader/src/features/system/domain/entity"
	systemRepo "tg-downloader/src/features/system/domain/repository"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemRepository struct{}

func NewSystemRepository() systemRepo.ISystemRepository {
	return &SystemRepository{}
}

func (r *SystemRepository) GetSystemInfo() (*entity.SystemInfo, error) {
	systemInfo := &entity.SystemInfo{
		Timestamp: time.Now(),
		Errors:    []string{},
	}

	// Collect host information
	if hostInfo, err := r.getHostInfo(); err != nil {
		systemInfo.Errors = append(systemInfo.Errors, fmt.Sprintf("Host info: %v", err))
	} else {
		systemInfo.Host = hostInfo
	}

	// Collect CPU information
	if cpuInfo, err := r.getCPUInfo(); err != nil {
		systemInfo.Errors = append(systemInfo.Errors, fmt.Sprintf("CPU info: %v", err))
	} else {
		systemInfo.CPU = cpuInfo
	}

	// Collect memory information
	if memInfo, err := r.getMemoryInfo(); err != nil {
		systemInfo.Errors = append(systemInfo.Errors, fmt.Sprintf("Memory info: %v", err))
	} else {
		systemInfo.Memory = memInfo
	}

	return systemInfo, nil
}

func (r *SystemRepository) getHostInfo() (*entity.HostInfo, error) {
	hostStat, err := host.Info()
	if err != nil {
		return nil, err
	}

	hostInfo := &entity.HostInfo{}

	// Set fields only if they're available
	if hostStat.Hostname != "" {
		hostInfo.Hostname = &hostStat.Hostname
	}
	if hostStat.OS != "" {
		hostInfo.OS = &hostStat.OS
	}
	if hostStat.Platform != "" {
		hostInfo.Platform = &hostStat.Platform
	}
	if hostStat.PlatformFamily != "" {
		hostInfo.PlatformFamily = &hostStat.PlatformFamily
	}
	if hostStat.PlatformVersion != "" {
		hostInfo.PlatformVersion = &hostStat.PlatformVersion
	}
	if hostStat.KernelVersion != "" {
		hostInfo.KernelVersion = &hostStat.KernelVersion
	}
	if hostStat.KernelArch != "" {
		hostInfo.KernelArch = &hostStat.KernelArch
	}
	if hostStat.Uptime > 0 {
		hostInfo.Uptime = &hostStat.Uptime
		uptimeFormatted := r.formatDuration(time.Duration(hostStat.Uptime) * time.Second)
		hostInfo.UptimeFormatted = &uptimeFormatted
	}
	if hostStat.BootTime > 0 {
		hostInfo.BootTime = &hostStat.BootTime
	}

	return hostInfo, nil
}

func (r *SystemRepository) getCPUInfo() (*entity.CPUInfo, error) {
	cpuInfo := &entity.CPUInfo{}

	// Get CPU info
	if cpuInfos, err := cpu.Info(); err == nil && len(cpuInfos) > 0 {
		cpuInformation := cpuInfos[0]
		if cpuInformation.ModelName != "" {
			cpuInfo.ModelName = &cpuInformation.ModelName
		}
		if cpuInformation.Family != "" {
			cpuInfo.Family = &cpuInformation.Family
		}
		if cpuInformation.Mhz > 0 {
			cpuInfo.Speed = &cpuInformation.Mhz
		}
	}

	// Get CPU usage with shorter measurement time
	if cpuPercent, err := cpu.Percent(500*time.Millisecond, false); err == nil && len(cpuPercent) > 0 {
		cpuInfo.UsagePercent = &cpuPercent[0]
	}

	// Get per-core usage with shorter measurement time
	if cpuPercentPerCore, err := cpu.Percent(500*time.Millisecond, true); err == nil && len(cpuPercentPerCore) > 0 {
		cpuInfo.UsagePerCore = cpuPercentPerCore
	}

	// Get CPU counts
	if physicalCores, err := cpu.Counts(false); err == nil {
		cores := int32(physicalCores)
		cpuInfo.PhysicalCores = &cores
	}

	if logicalCores, err := cpu.Counts(true); err == nil {
		cores := int32(logicalCores)
		cpuInfo.LogicalCores = &cores
	}

	return cpuInfo, nil
}

func (r *SystemRepository) getMemoryInfo() (*entity.MemoryInfo, error) {
	memInfo := &entity.MemoryInfo{}

	// Virtual memory
	if memStat, err := mem.VirtualMemory(); err == nil {
		if memStat.Total > 0 {
			memInfo.Total = &memStat.Total
			totalFormatted := r.formatBytes(memStat.Total)
			memInfo.TotalFormatted = &totalFormatted
		}
		if memStat.Available > 0 {
			memInfo.Available = &memStat.Available
			availableFormatted := r.formatBytes(memStat.Available)
			memInfo.AvailableFormatted = &availableFormatted
		}
		if memStat.Used > 0 {
			memInfo.Used = &memStat.Used
			usedFormatted := r.formatBytes(memStat.Used)
			memInfo.UsedFormatted = &usedFormatted
		}
		if memStat.UsedPercent > 0 {
			memInfo.UsedPercent = &memStat.UsedPercent
		}
	}

	// Swap memory
	if swapStat, err := mem.SwapMemory(); err == nil {
		if swapStat.Total > 0 {
			memInfo.SwapTotal = &swapStat.Total
			swapTotalFormatted := r.formatBytes(swapStat.Total)
			memInfo.SwapTotalFormatted = &swapTotalFormatted
		}
		if swapStat.Used > 0 {
			memInfo.SwapUsed = &swapStat.Used
			swapUsedFormatted := r.formatBytes(swapStat.Used)
			memInfo.SwapUsedFormatted = &swapUsedFormatted
		}
		if swapStat.UsedPercent > 0 {
			memInfo.SwapPercent = &swapStat.UsedPercent
		}
	}

	return memInfo, nil
}

// Helper functions

func (r *SystemRepository) formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %sB", float64(bytes)/float64(div), "KMGTPE"[exp:exp+1])
}

func (r *SystemRepository) formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}
