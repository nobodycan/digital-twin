package admin

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nobodycan/digital-twin/internal/persona"
)

var ErrPersonaVersionNotFound = errors.New("persona version not found")

type PersonaStatus string

const (
	PersonaDraft     PersonaStatus = "draft"
	PersonaPublished PersonaStatus = "published"
)

type PersonaVersion struct {
	ID          string          `json:"id"`
	TenantID    string          `json:"tenant_id"`
	Persona     persona.Persona `json:"persona"`
	Status      PersonaStatus   `json:"status"`
	Version     int             `json:"version"`
	PublishedAt time.Time       `json:"published_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type PersonaStore interface {
	Save(PersonaVersion) (PersonaVersion, error)
	Get(tenantID, versionID string) (PersonaVersion, error)
	List(tenantID string) ([]PersonaVersion, error)
	SetActive(tenantID, versionID string) error
	Active(tenantID string) (PersonaVersion, error)
}

type PersonaService struct {
	store PersonaStore
	now   func() time.Time
}

func NewPersonaService(store PersonaStore) PersonaService {
	return PersonaService{
		store: store,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func (s PersonaService) SaveDraft(tenantID string, p persona.Persona) (PersonaVersion, error) {
	versions, err := s.store.List(tenantID)
	if err != nil {
		return PersonaVersion{}, err
	}
	now := s.now()
	version := PersonaVersion{
		ID:        fmt.Sprintf("persona-v%d", len(versions)+1),
		TenantID:  tenantID,
		Persona:   p,
		Status:    PersonaDraft,
		Version:   len(versions) + 1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return s.store.Save(version)
}

func (s PersonaService) Publish(tenantID, versionID string) (PersonaVersion, error) {
	version, err := s.store.Get(tenantID, versionID)
	if err != nil {
		return PersonaVersion{}, err
	}
	if err := version.Persona.Validate(); err != nil {
		return PersonaVersion{}, err
	}
	now := s.now()
	version.Status = PersonaPublished
	version.PublishedAt = now
	version.UpdatedAt = now
	if version, err = s.store.Save(version); err != nil {
		return PersonaVersion{}, err
	}
	if err := s.store.SetActive(tenantID, version.ID); err != nil {
		return PersonaVersion{}, err
	}
	return version, nil
}

func (s PersonaService) Rollback(tenantID, versionID string) (PersonaVersion, error) {
	version, err := s.store.Get(tenantID, versionID)
	if err != nil {
		return PersonaVersion{}, err
	}
	if version.Status != PersonaPublished {
		return PersonaVersion{}, persona.ErrInvalidPersona
	}
	if err := s.store.SetActive(tenantID, version.ID); err != nil {
		return PersonaVersion{}, err
	}
	return version, nil
}

func (s PersonaService) Active(tenantID string) (PersonaVersion, error) {
	return s.store.Active(tenantID)
}

type InMemoryPersonaStore struct {
	mu       sync.Mutex
	versions map[string]map[string]PersonaVersion
	active   map[string]string
}

func NewInMemoryPersonaStore() *InMemoryPersonaStore {
	return &InMemoryPersonaStore{
		versions: make(map[string]map[string]PersonaVersion),
		active:   make(map[string]string),
	}
}

func (s *InMemoryPersonaStore) Save(version PersonaVersion) (PersonaVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.versions[version.TenantID]; !ok {
		s.versions[version.TenantID] = make(map[string]PersonaVersion)
	}
	s.versions[version.TenantID][version.ID] = version
	return version, nil
}

func (s *InMemoryPersonaStore) Get(tenantID, versionID string) (PersonaVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	version, ok := s.versions[tenantID][versionID]
	if !ok {
		return PersonaVersion{}, ErrPersonaVersionNotFound
	}
	return version, nil
}

func (s *InMemoryPersonaStore) List(tenantID string) ([]PersonaVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	versions := s.versions[tenantID]
	out := make([]PersonaVersion, 0, len(versions))
	for _, version := range versions {
		out = append(out, version)
	}
	return out, nil
}

func (s *InMemoryPersonaStore) SetActive(tenantID, versionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.versions[tenantID][versionID]; !ok {
		return ErrPersonaVersionNotFound
	}
	s.active[tenantID] = versionID
	return nil
}

func (s *InMemoryPersonaStore) Active(tenantID string) (PersonaVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	versionID := s.active[tenantID]
	if versionID == "" {
		return PersonaVersion{}, ErrPersonaVersionNotFound
	}
	version, ok := s.versions[tenantID][versionID]
	if !ok {
		return PersonaVersion{}, ErrPersonaVersionNotFound
	}
	return version, nil
}
