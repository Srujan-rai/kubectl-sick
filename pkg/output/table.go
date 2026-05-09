package output

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"kubectl-sick/pkg/types"
)

var (
	criticalColor = color.New(color.FgRed, color.Bold)
	warningColor  = color.New(color.FgYellow)
	infoColor     = color.New(color.FgBlue)
	greenColor    = color.New(color.FgGreen)
)

func PrintTable(issues []types.Issue, noColor bool) {
	if noColor {
		color.NoColor = true
	}

	if len(issues) == 0 {
		greenColor.Println("✓ All resources healthy")
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"SEVERITY", "KIND", "NAMESPACE", "NAME", "REASON"})
	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetColumnSeparator("   ")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(false)
	table.SetNoWhiteSpace(true)
	table.SetTablePadding("  ")

	for _, issue := range issues {
		ns := issue.Namespace
		if ns == "" {
			ns = "<none>"
		}

		severityStr := colorize(issue.Severity, issue.Severity.String())

		table.Append([]string{
			severityStr,
			issue.Kind,
			ns,
			issue.Name,
			issue.Reason,
		})
	}

	table.Render()

	var critical, warning, info int
	for _, issue := range issues {
		switch issue.Severity {
		case types.Critical:
			critical++
		case types.Warning:
			warning++
		case types.Info:
			info++
		}
	}

	summary := fmt.Sprintf("\n── %s · %s · %s ──",
		criticalColor.Sprintf("%d critical", critical),
		warningColor.Sprintf("%d warning", warning),
		infoColor.Sprintf("%d info", info),
	)
	fmt.Println(summary)
}

func colorize(sev types.Severity, s string) string {
	switch sev {
	case types.Critical:
		return criticalColor.Sprint(s)
	case types.Warning:
		return warningColor.Sprint(s)
	default:
		return infoColor.Sprint(s)
	}
}
