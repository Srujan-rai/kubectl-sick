package types

import (
	"sort"
	"time"
)

type Severity int

const (
	Info Severity = iota
	Warning
	Critical
)

func (s Severity) String() string {
	switch s {
	case Critical:
		return "CRITICAL"
	case Warning:
		return "WARNING"
	default:
		return "INFO"
	}
}

type Issue struct {
	Severity  Severity
	Kind      string
	Namespace string
	Name      string
	Reason    string
	Since     time.Time
}

func SortIssues(issues []Issue) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Severity != issues[j].Severity {
			return issues[i].Severity > issues[j].Severity
		}
		if issues[i].Kind != issues[j].Kind {
			return issues[i].Kind < issues[j].Kind
		}
		return issues[i].Name < issues[j].Name
	})
}
