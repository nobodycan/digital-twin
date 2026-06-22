package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nobodycan/digital-twin/internal/core"
)

type ReportPaths struct {
	JSONPath     string
	MarkdownPath string
}

var reportIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func WriteReports(dir string, result SuiteResult) (ReportPaths, error) {
	if result.ID == "" {
		result.ID = "eval-report"
	}
	if !reportIDPattern.MatchString(result.ID) || result.ID == "." || result.ID == ".." {
		return ReportPaths{}, core.NewNamedError(core.ErrInvalidInput, "report_id", result.ID)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ReportPaths{}, err
	}
	paths := ReportPaths{
		JSONPath:     filepath.Join(dir, result.ID+".json"),
		MarkdownPath: filepath.Join(dir, result.ID+".md"),
	}
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return ReportPaths{}, err
	}
	if err := os.WriteFile(paths.JSONPath, append(jsonData, '\n'), 0o644); err != nil {
		return ReportPaths{}, err
	}
	if err := os.WriteFile(paths.MarkdownPath, []byte(formatMarkdownReport(result)), 0o644); err != nil {
		return ReportPaths{}, err
	}
	return paths, nil
}

func formatMarkdownReport(result SuiteResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Eval Report %s\n\n", result.ID)
	fmt.Fprintf(&b, "Status: %s\n\n", result.Status)
	writeVersionMetadata(&b, result)
	if len(result.FailedCaseIDs) == 0 {
		b.WriteString("Failed cases: none\n\n")
	} else {
		fmt.Fprintf(&b, "Failed cases: %s\n\n", strings.Join(result.FailedCaseIDs, ", "))
	}
	b.WriteString("| Case | Check | Status | Message |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, check := range result.Checks {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", check.CaseID, check.Check, check.Status, escapeMarkdownCell(check.Message))
	}
	return b.String()
}

func writeVersionMetadata(b *strings.Builder, result SuiteResult) {
	if len(result.VersionMetadata) == 0 {
		return
	}
	keys := make([]string, 0, len(result.VersionMetadata))
	for key := range result.VersionMetadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		label := strings.TrimSuffix(strings.ReplaceAll(key, "_", " "), " id")
		label = strings.ToUpper(label[:1]) + label[1:]
		fmt.Fprintf(b, "%s: %v\n", label, result.VersionMetadata[key])
	}
	b.WriteString("\n")
}

func escapeMarkdownCell(value string) string {
	if value == "" {
		return ""
	}
	return strings.ReplaceAll(value, "|", "\\|")
}
