package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestLocalStorePersistsConversationAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	first := NewLocalStore(dir)
	conversation := testConversation("tenant-1", "user-1", "conv-1")

	if err := first.SaveConversation(t.Context(), conversation); err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	if err := first.AppendMessage(t.Context(), conversation.TenantID, conversation.UserID, conversation.ID, types.Message{
		ID:        "msg-2",
		Role:      types.RoleAssistant,
		Content:   "hello",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("append message: %v", err)
	}

	second := NewLocalStore(dir)
	got, err := second.GetConversation(t.Context(), "tenant-1", "user-1", "conv-1")
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if got.ID != conversation.ID || len(got.Messages) != 2 {
		t.Fatalf("unexpected conversation %#v", got)
	}

	messages, err := second.ListMessages(t.Context(), "tenant-1", "user-1", "conv-1")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 || messages[1].Content != "hello" {
		t.Fatalf("unexpected messages %#v", messages)
	}
}

func TestLocalStoreScopesByTenantAndUser(t *testing.T) {
	store := NewLocalStore(t.TempDir())
	if err := store.SaveConversation(t.Context(), testConversation("tenant-1", "user-1", "conv-1")); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := store.GetConversation(t.Context(), "tenant-2", "user-1", "conv-1"); !errors.Is(err, core.ErrConversationNotFound) {
		t.Fatalf("expected conversation not found for missing scoped conversation, got %v", err)
	}
}

func TestLocalStoreRejectsUnsafeIDs(t *testing.T) {
	store := NewLocalStore(t.TempDir())
	err := store.SaveConversation(t.Context(), testConversation("tenant-1", "user-1", "../escape"))
	if !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}

	err = store.SaveConversation(t.Context(), testConversation("tenant-1", "user-1", ".."))
	if !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("expected invalid input for parent-directory ID, got %v", err)
	}
}

func TestLocalStoreReturnsActionableCorruptFileError(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStore(dir)
	conversation := testConversation("tenant-1", "user-1", "conv-1")
	if err := store.SaveConversation(t.Context(), conversation); err != nil {
		t.Fatalf("save: %v", err)
	}

	path := filepath.Join(dir, "tenants", "tenant-1", "users", "user-1", "conversations", "conv-1.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatalf("corrupt file: %v", err)
	}

	_, err := store.GetConversation(t.Context(), "tenant-1", "user-1", "conv-1")
	if !errors.Is(err, core.ErrStoreFailure) {
		t.Fatalf("expected store failure, got %v", err)
	}
}

func TestLocalStoreReturnsStoreFailureForReadErrors(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStore(dir)
	path := filepath.Join(dir, "tenants", "tenant-1", "users", "user-1", "conversations", "conv-1.json")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir path: %v", err)
	}

	_, err := store.GetConversation(t.Context(), "tenant-1", "user-1", "conv-1")
	if !errors.Is(err, core.ErrStoreFailure) {
		t.Fatalf("expected store failure, got %v", err)
	}
}

func TestLocalStoreSaveLeavesNoTemporaryFilesBehind(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStore(dir)
	conversation := testConversation("tenant-1", "user-1", "conv-1")

	if err := store.SaveConversation(t.Context(), conversation); err != nil {
		t.Fatalf("save: %v", err)
	}

	conversation.Messages = append(conversation.Messages, types.Message{
		ID:        "msg-2",
		Role:      types.RoleAssistant,
		Content:   "reply",
		CreatedAt: time.Now().UTC(),
	})
	if err := store.SaveConversation(t.Context(), conversation); err != nil {
		t.Fatalf("save updated: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "tenants", "tenant-1", "users", "user-1", "conversations", "*.tmp*"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no temp files, got %v", matches)
	}
}

func testConversation(tenantID, userID, conversationID string) types.Conversation {
	now := time.Now().UTC()
	return types.Conversation{
		ID:       conversationID,
		TenantID: tenantID,
		UserID:   userID,
		Messages: []types.Message{{
			ID:        "msg-1",
			Role:      types.RoleUser,
			Content:   "hi",
			CreatedAt: now,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}
