package admin

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type FileToolPolicyStore struct {
	dir string
}

func NewFileToolPolicyStore(dir string) *FileToolPolicyStore {
	return &FileToolPolicyStore{dir: dir}
}

func (s *FileToolPolicyStore) SaveToolPolicy(policy ToolPolicy) (ToolPolicy, error) {
	policies, err := s.load()
	if err != nil {
		return ToolPolicy{}, err
	}
	policies[toolPolicyKey(policy.TenantID, policy.PersonaID)] = policy
	if err := s.save(policies); err != nil {
		return ToolPolicy{}, err
	}
	return policy, nil
}

func (s *FileToolPolicyStore) GetToolPolicy(tenantID, personaID string) (ToolPolicy, error) {
	policies, err := s.load()
	if err != nil {
		return ToolPolicy{}, err
	}
	policy, ok := policies[toolPolicyKey(tenantID, personaID)]
	if !ok {
		return ToolPolicy{}, ErrToolDenied
	}
	return policy, nil
}

func (s *FileToolPolicyStore) load() (map[string]ToolPolicy, error) {
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]ToolPolicy), nil
	}
	if err != nil {
		return nil, err
	}
	var policies map[string]ToolPolicy
	if err := json.Unmarshal(data, &policies); err != nil {
		return nil, err
	}
	if policies == nil {
		policies = make(map[string]ToolPolicy)
	}
	return policies, nil
}

func (s *FileToolPolicyStore) save(policies map[string]ToolPolicy) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(policies, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(), data, 0o600)
}

func (s *FileToolPolicyStore) path() string {
	return filepath.Join(s.dir, "tool_policies.json")
}
