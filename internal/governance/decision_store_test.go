package governance

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/core"
)

func TestInMemoryDecisionStoreScopesByTenant(t *testing.T) {
	store := NewInMemoryDecisionStore()
	now := time.Now().UTC()

	if err := store.SaveDecision(DecisionRecord{
		ID:        "decision-1",
		TenantID:  "tenant-1",
		Type:      DecisionEval,
		ActorID:   "operator-1",
		CreatedAt: now,
		Evidence:  map[string]any{"case_id": "persona-disclosure"},
	}); err != nil {
		t.Fatalf("SaveDecision tenant-1: %v", err)
	}
	if err := store.SaveDecision(DecisionRecord{
		ID:        "decision-2",
		TenantID:  "tenant-2",
		Type:      DecisionRelease,
		ActorID:   "operator-2",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("SaveDecision tenant-2: %v", err)
	}

	tenantOne, err := store.ListDecisions("tenant-1")
	if err != nil {
		t.Fatalf("ListDecisions tenant-1: %v", err)
	}
	if len(tenantOne) != 1 || tenantOne[0].ID != "decision-1" {
		t.Fatalf("tenant-1 decisions leaked or missing: %#v", tenantOne)
	}

	if _, err := store.GetDecision("tenant-1", "decision-2"); !errors.Is(err, core.ErrStoreFailure) {
		t.Fatalf("GetDecision cross-tenant error = %v, want store failure", err)
	}
}

func TestFileDecisionStorePersistsAndScopesByTenant(t *testing.T) {
	dir := t.TempDir()
	first := NewFileDecisionStore(dir)
	record := DecisionRecord{
		ID:        "decision-1",
		TenantID:  "tenant-1",
		Type:      DecisionPolicy,
		ActorID:   "operator-1",
		CreatedAt: time.Now().UTC(),
		Evidence:  map[string]any{"reason": "prompt_injection"},
	}

	if err := first.SaveDecision(record); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}
	files, err := tempFilesUnder(dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary files left behind: files=%#v err=%v", files, err)
	}

	second := NewFileDecisionStore(dir)
	got, err := second.GetDecision("tenant-1", "decision-1")
	if err != nil {
		t.Fatalf("GetDecision after reopen: %v", err)
	}
	if got.Type != DecisionPolicy || got.Evidence["reason"] != "prompt_injection" {
		t.Fatalf("unexpected decision after reopen: %#v", got)
	}
	if _, err := second.GetDecision("tenant-2", "decision-1"); !errors.Is(err, core.ErrStoreFailure) {
		t.Fatalf("GetDecision cross-tenant error = %v, want store failure", err)
	}
}

func TestDecisionStoreRejectsUnsafeIDs(t *testing.T) {
	store := NewInMemoryDecisionStore()
	err := store.SaveDecision(DecisionRecord{
		ID:        "../escape",
		TenantID:  "tenant-1",
		Type:      DecisionEval,
		ActorID:   "operator-1",
		CreatedAt: time.Now().UTC(),
	})
	if !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("SaveDecision error = %v, want invalid input", err)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}

func tempFilesUnder(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".tmp" {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
