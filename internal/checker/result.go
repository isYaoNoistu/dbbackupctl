package checker

import "time"

// CheckStatus represents the status of a check item
type CheckStatus string

const (
	CheckOK   CheckStatus = "ok"
	CheckWarn CheckStatus = "warn"
	CheckFail CheckStatus = "fail"
)

// CheckItem represents a single check result
type CheckItem struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	Message string      `json:"message"`
	Detail  string      `json:"detail,omitempty"`
}

// CheckReport represents the full check report
type CheckReport struct {
	StartedAt  time.Time   `json:"started_at"`
	FinishedAt time.Time   `json:"finished_at"`
	Items      []CheckItem `json:"items"`
}

// HasFailure returns true if any check failed
func (r CheckReport) HasFailure() bool {
	for _, item := range r.Items {
		if item.Status == CheckFail {
			return true
		}
	}
	return false
}

// HasWarning returns true if any check has a warning
func (r CheckReport) HasWarning() bool {
	for _, item := range r.Items {
		if item.Status == CheckWarn {
			return true
		}
	}
	return false
}

// Summary returns a summary of the check report
func (r CheckReport) Summary() string {
	ok, warn, fail := 0, 0, 0
	for _, item := range r.Items {
		switch item.Status {
		case CheckOK:
			ok++
		case CheckWarn:
			warn++
		case CheckFail:
			fail++
		}
	}

	if fail > 0 {
		return "FAILED"
	}
	if warn > 0 {
		return "PASSED with warnings"
	}
	return "PASSED"
}