package evals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestWriteReportsWritesJSONAndMarkdownWithVersionMetadata(t *testing.T) {
	dir := t.TempDir()
	result := SuiteResult{
		ID:     "run-1",
		Status: SuiteFailed,
		VersionMetadata: types.Metadata{
			"tenant_id":                "tenant-1",
			"persona_version_id":       "persona-v1",
			"tool_policy_version_id":   "tools-v1",
			"knowledge_version_id":     "knowledge-v1",
			"memory_policy_version_id": "memory-v1",
			"model_policy_version_id":  "model-v1",
		},
		Checks: []CheckResult{
			{CaseID: "persona-disclosure", Check: "persona", Status: CheckFailed, Message: "missing AI disclosure"},
			{CaseID: "tool-denied-http", Check: "rag", Status: CheckSkipped, Message: "no RAG expectation"},
		},
		FailedCaseIDs: []string{"persona-disclosure"},
	}

	paths, err := WriteReports(dir, result)
	if err != nil {
		t.Fatalf("WriteReports returned error: %v", err)
	}

	rawJSON, err := os.ReadFile(paths.JSONPath)
	if err != nil {
		t.Fatalf("read JSON report: %v", err)
	}
	var decoded SuiteResult
	if err := json.Unmarshal(rawJSON, &decoded); err != nil {
		t.Fatalf("decode JSON report: %v", err)
	}
	if decoded.ID != "run-1" || decoded.VersionMetadata["persona_version_id"] != "persona-v1" {
		t.Fatalf("decoded report = %#v, want suite and context metadata", decoded)
	}

	rawMarkdown, err := os.ReadFile(paths.MarkdownPath)
	if err != nil {
		t.Fatalf("read Markdown report: %v", err)
	}
	markdown := string(rawMarkdown)
	for _, want := range []string{
		"# Eval Report run-1",
		"Status: failed",
		"Persona version: persona-v1",
		"Failed cases: persona-disclosure",
		"| persona-disclosure | persona | failed | missing AI disclosure |",
		"| tool-denied-http | rag | skipped | no RAG expectation |",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown report missing %q:\n%s", want, markdown)
		}
	}
	if filepath.Base(paths.JSONPath) != "run-1.json" || filepath.Base(paths.MarkdownPath) != "run-1.md" {
		t.Fatalf("report paths = %#v, want run ID file names", paths)
	}
}

func TestWriteReportsRejectsUnsafeRunID(t *testing.T) {
	_, err := WriteReports(t.TempDir(), SuiteResult{ID: "../escape", Status: SuitePassed})
	if err == nil {
		t.Fatal("WriteReports returned nil error, want unsafe run id rejection")
	}
}
