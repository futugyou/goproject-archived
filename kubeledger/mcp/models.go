package mcp

import "time"

type FileReadResult struct {
	Filename string
	Path     string
	Exists   bool
	Payload  []byte
	ErrorMsg string
	MtimeUtc *time.Time
}

func (f *FileReadResult) Ok() bool {
	return f.ErrorMsg == "" && len(f.Payload) > 0
}

type CostModelInfo struct {
	CostModel string `json:"cost_model"`
	Currency  string `json:"currency"`
	Unit      string `json:"unit"`
}

type ScaleInfo struct {
	Granularity        string
	Depth              string
	PointsPerNamespace int
}

type TrendsInfo struct {
	Granularity        string
	Depth              string
	PointsPerNamespace int
}

type EfficiencyInfo struct {
	Aggregated      bool
	HourlyTimeserie bool
}

type DatasetInfo struct {
	MetricsAvailable  []string
	GpuAvailable      bool
	Scales            map[string]ScaleInfo
	Trends            *TrendsInfo
	Dimensions        []string
	Efficiency        EfficiencyInfo
	CostModel         *CostModelInfo
	BillingHourlyRate float64
	DataFreshnessUtc  *time.Time
	Warnings          []string
}

type TrendDataDTO struct {
	Name    string  `json:"name"`
	DateUTC string  `json:"dateUTC"`
	Usage   float64 `json:"usage"`
}
