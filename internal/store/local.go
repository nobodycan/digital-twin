package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// LocalStore persists conversations as JSON files under a data directory.
type LocalStore struct {
	root string
	mu   sync.Mutex
}

// NewLocalStore creates a local filesystem-backed store.
func NewLocalStore(root string) *LocalStore {
	return &LocalStore{root: root}
}

// SaveConversation writes a full conversation document.
func (s *LocalStore) SaveConversation(ctx context.Context, conversation types.Conversation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateConversationIDs(conversation.TenantID, conversation.UserID, conversation.ID); err != nil {
		return err
	}
	if conversation.UpdatedAt.IsZero() {
		conversation.UpdatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.conversationPath(conversation.TenantID, conversation.UserID, conversation.ID)
	return writeJSONAtomic(path, conversation)
}

// GetConversation reads a persisted conversation document.
func (s *LocalStore) GetConversation(ctx context.Context, tenantID, userID, conversationID string) (types.Conversation, error) {
	if err := ctx.Err(); err != nil {
		return types.Conversation{}, err
	}
	if err := validateConversationIDs(tenantID, userID, conversationID); err != nil {
		return types.Conversation{}, err
	}

	path := s.conversationPath(tenantID, userID, conversationID)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return types.Conversation{}, core.WrapError(core.ErrConversationNotFound, "conversation missing")
		}
		return types.Conversation{}, core.WrapError(core.ErrStoreFailure, "read conversation")
	}

	var conversation types.Conversation
	if err := json.Unmarshal(data, &conversation); err != nil {
		return types.Conversation{}, core.WrapError(core.ErrStoreFailure, "decode conversation")
	}
	return conversation, nil
}

// AppendMessage appends a message to a persisted conversation.
func (s *LocalStore) AppendMessage(ctx context.Context, tenantID, userID, conversationID string, message types.Message) error {
	conversation, err := s.GetConversation(ctx, tenantID, userID, conversationID)
	if err != nil {
		return err
	}
	conversation.Messages = append(conversation.Messages, message)
	conversation.UpdatedAt = time.Now().UTC()
	return s.SaveConversation(ctx, conversation)
}

// ListMessages returns a copy of a conversation's messages.
func (s *LocalStore) ListMessages(ctx context.Context, tenantID, userID, conversationID string) ([]types.Message, error) {
	conversation, err := s.GetConversation(ctx, tenantID, userID, conversationID)
	if err != nil {
		return nil, err
	}
	messages := make([]types.Message, len(conversation.Messages))
	copy(messages, conversation.Messages)
	return messages, nil
}

func (s *LocalStore) conversationPath(tenantID, userID, conversationID string) string {
	return filepath.Join(s.root, "tenants", tenantID, "users", userID, "conversations", conversationID+".json")
}

func validateConversationIDs(values ...string) error {
	for _, value := range values {
		if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\`) || !safeIDPattern.MatchString(value) {
			return core.NewNamedError(core.ErrInvalidInput, "id", value)
		}
	}
	return nil
}

func writeJSONAtomic(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return core.WrapError(core.ErrStoreFailure, "create store directory")
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return core.WrapError(core.ErrStoreFailure, "encode conversation")
	}
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return core.WrapError(core.ErrStoreFailure, "create temp conversation")
	}
	tmp := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmp)
		return core.WrapError(core.ErrStoreFailure, "write conversation")
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmp)
		return core.WrapError(core.ErrStoreFailure, "close conversation")
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return core.WrapError(core.ErrStoreFailure, "replace conversation")
	}
	return nil
}

// InMemoryStore is a lightweight fake Store for tests.
type InMemoryStore struct {
	mu            sync.Mutex
	conversations map[string]types.Conversation
}

// NewInMemoryStore creates an empty in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{conversations: make(map[string]types.Conversation)}
}

// SaveConversation stores a conversation in memory.
func (s *InMemoryStore) SaveConversation(_ context.Context, conversation types.Conversation) error {
	if err := validateConversationIDs(conversation.TenantID, conversation.UserID, conversation.ID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conversations[storeKey(conversation.TenantID, conversation.UserID, conversation.ID)] = conversation
	return nil
}

// GetConversation reads a conversation from memory.
func (s *InMemoryStore) GetConversation(_ context.Context, tenantID, userID, conversationID string) (types.Conversation, error) {
	if err := validateConversationIDs(tenantID, userID, conversationID); err != nil {
		return types.Conversation{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	conversation, ok := s.conversations[storeKey(tenantID, userID, conversationID)]
	if !ok {
		return types.Conversation{}, core.WrapError(core.ErrConversationNotFound, "conversation missing")
	}
	return conversation, nil
}

// AppendMessage appends a message to a stored conversation.
func (s *InMemoryStore) AppendMessage(ctx context.Context, tenantID, userID, conversationID string, message types.Message) error {
	conversation, err := s.GetConversation(ctx, tenantID, userID, conversationID)
	if err != nil {
		return err
	}
	conversation.Messages = append(conversation.Messages, message)
	return s.SaveConversation(ctx, conversation)
}

// ListMessages returns messages from a stored conversation.
func (s *InMemoryStore) ListMessages(ctx context.Context, tenantID, userID, conversationID string) ([]types.Message, error) {
	conversation, err := s.GetConversation(ctx, tenantID, userID, conversationID)
	if err != nil {
		return nil, err
	}
	messages := make([]types.Message, len(conversation.Messages))
	copy(messages, conversation.Messages)
	return messages, nil
}

func storeKey(tenantID, userID, conversationID string) string {
	return tenantID + "/" + userID + "/" + conversationID
}

var _ Store = (*LocalStore)(nil)
var _ Store = (*InMemoryStore)(nil)
