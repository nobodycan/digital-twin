package admin

import (
	"context"
	"errors"
	"sync"

	"github.com/nobodycan/digital-twin/internal/agents"
)

var ErrToolDenied = errors.New("tool denied by policy")

type ApprovalMode string

const (
	ApprovalAuto   ApprovalMode = "auto"
	ApprovalManual ApprovalMode = "manual"
)

type ToolPolicy struct {
	TenantID     string       `json:"tenant_id"`
	PersonaID    string       `json:"persona_id"`
	AllowedTools []string     `json:"allowed_tools"`
	ApprovalMode ApprovalMode `json:"approval_mode"`
}

type ToolPolicyStore interface {
	SaveToolPolicy(ToolPolicy) (ToolPolicy, error)
	GetToolPolicy(tenantID, personaID string) (ToolPolicy, error)
}

type ToolPolicyService struct {
	store ToolPolicyStore
}

func NewToolPolicyService(store ToolPolicyStore) ToolPolicyService {
	return ToolPolicyService{store: store}
}

func (s ToolPolicyService) Save(tenantID string, policy ToolPolicy) (ToolPolicy, error) {
	policy.TenantID = tenantID
	if policy.ApprovalMode == "" {
		policy.ApprovalMode = ApprovalAuto
	}
	return s.store.SaveToolPolicy(policy)
}

func (s ToolPolicyService) Authorize(tenantID, personaID, toolName string) error {
	policy, err := s.store.GetToolPolicy(tenantID, personaID)
	if err != nil {
		return err
	}
	for _, allowed := range policy.AllowedTools {
		if allowed == toolName {
			return nil
		}
	}
	return ErrToolDenied
}

func (s ToolPolicyService) AuthorizeSkill(ctx context.Context, call agents.SkillCall) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.Authorize(call.TenantID, call.PersonaID, call.SkillName)
}

type InMemoryToolPolicyStore struct {
	mu       sync.Mutex
	policies map[string]ToolPolicy
}

func NewInMemoryToolPolicyStore() *InMemoryToolPolicyStore {
	return &InMemoryToolPolicyStore{policies: make(map[string]ToolPolicy)}
}

func (s *InMemoryToolPolicyStore) SaveToolPolicy(policy ToolPolicy) (ToolPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[toolPolicyKey(policy.TenantID, policy.PersonaID)] = policy
	return policy, nil
}

func (s *InMemoryToolPolicyStore) GetToolPolicy(tenantID, personaID string) (ToolPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	policy, ok := s.policies[toolPolicyKey(tenantID, personaID)]
	if !ok {
		return ToolPolicy{}, ErrToolDenied
	}
	return policy, nil
}

func toolPolicyKey(tenantID, personaID string) string {
	return tenantID + "/" + personaID
}
