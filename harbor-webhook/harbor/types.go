package harbor

type Webhook struct {
	Type      string    `json:"type"`
	OccurAt   int64     `json:"occur_at"`
	Operator  string    `json:"operator"`
	EventData EventData `json:"event_data"`
}

type EventData struct {
	Resources  []Resource `json:"resources"`
	Repository Repository `json:"repository"`
	Scan       ScanMeta   `json:"scan"`
}

type Resource struct {
	Digest       string                  `json:"digest"`
	Tag          string                  `json:"tag"`
	ResourceURL  string                  `json:"resource_url"`
	ScanOverview map[string]ScanOverview `json:"scan_overview"`
}

type ScanOverview struct {
	ReportID        string      `json:"report_id"`
	ScanStatus      string      `json:"scan_status"`
	Severity        string      `json:"severity"`
	Duration        int         `json:"duration"`
	Summary         VulnSummary `json:"summary"`
	StartTime       string      `json:"start_time"`
	EndTime         string      `json:"end_time"`
	Scanner         ScannerInfo `json:"scanner"`
	CompletePercent int         `json:"complete_percent"`
}

type VulnSummary struct {
	Total   int            `json:"total"`
	Fixable int            `json:"fixable"`
	Summary map[string]int `json:"summary"`
}

type ScannerInfo struct {
	Name    string `json:"name"`
	Vendor  string `json:"vendor"`
	Version string `json:"version"`
}

type Repository struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	RepoFullName string `json:"repo_full_name"`
	RepoType     string `json:"repo_type"`
}

type ScanMeta struct {
	ScanType string `json:"scan_type"`
}

var SeverityRank = map[string]int{
	"None":     0,
	"Unknown":  0,
	"Low":      1,
	"Medium":   2,
	"High":     3,
	"Critical": 4,
}
