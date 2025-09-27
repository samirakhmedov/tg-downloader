package entity

import "time"

type SystemInfo struct {
	// Host Information
	Host *HostInfo `json:"host,omitempty"`

	// CPU Information
	CPU *CPUInfo `json:"cpu,omitempty"`

	// Memory Information
	Memory *MemoryInfo `json:"memory,omitempty"`

	// Collection errors (for diagnostics)
	Errors []string `json:"errors,omitempty"`

	// Timestamp when the information was collected
	Timestamp time.Time `json:"timestamp"`
}

type HostInfo struct {
	Hostname        *string `json:"hostname,omitempty"`
	OS              *string `json:"os,omitempty"`
	Platform        *string `json:"platform,omitempty"`
	PlatformFamily  *string `json:"platform_family,omitempty"`
	PlatformVersion *string `json:"platform_version,omitempty"`
	KernelVersion   *string `json:"kernel_version,omitempty"`
	KernelArch      *string `json:"kernel_arch,omitempty"`
	Uptime          *uint64 `json:"uptime_seconds,omitempty"`
	UptimeFormatted *string `json:"uptime_formatted,omitempty"`
	BootTime        *uint64 `json:"boot_time,omitempty"`
}

type CPUInfo struct {
	ModelName     *string    `json:"model_name,omitempty"`
	Family        *string    `json:"family,omitempty"`
	Speed         *float64   `json:"speed_mhz,omitempty"`
	PhysicalCores *int32     `json:"physical_cores,omitempty"`
	LogicalCores  *int32     `json:"logical_cores,omitempty"`
	UsagePercent  *float64   `json:"usage_percent,omitempty"`
	UsagePerCore  []float64  `json:"usage_per_core,omitempty"`
}

type MemoryInfo struct {
	Total       *uint64  `json:"total_bytes,omitempty"`
	Available   *uint64  `json:"available_bytes,omitempty"`
	Used        *uint64  `json:"used_bytes,omitempty"`
	UsedPercent *float64 `json:"used_percent,omitempty"`

	// Swap Information
	SwapTotal   *uint64  `json:"swap_total_bytes,omitempty"`
	SwapUsed    *uint64  `json:"swap_used_bytes,omitempty"`
	SwapPercent *float64 `json:"swap_used_percent,omitempty"`

	// Formatted strings for display
	TotalFormatted     *string `json:"total_formatted,omitempty"`
	AvailableFormatted *string `json:"available_formatted,omitempty"`
	UsedFormatted      *string `json:"used_formatted,omitempty"`
	SwapTotalFormatted *string `json:"swap_total_formatted,omitempty"`
	SwapUsedFormatted  *string `json:"swap_used_formatted,omitempty"`
}
