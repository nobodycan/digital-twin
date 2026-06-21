package admin

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type FilePersonaStore struct {
	dir string
}

type personaStoreDocument struct {
	Versions []PersonaVersion  `json:"versions"`
	Active   map[string]string `json:"active"`
}

func NewFilePersonaStore(dir string) *FilePersonaStore {
	return &FilePersonaStore{dir: dir}
}

func (s *FilePersonaStore) Save(version PersonaVersion) (PersonaVersion, error) {
	doc, err := s.load()
	if err != nil {
		return PersonaVersion{}, err
	}
	replaced := false
	for index, existing := range doc.Versions {
		if existing.TenantID == version.TenantID && existing.ID == version.ID {
			doc.Versions[index] = version
			replaced = true
			break
		}
	}
	if !replaced {
		doc.Versions = append(doc.Versions, version)
	}
	if err := s.save(doc); err != nil {
		return PersonaVersion{}, err
	}
	return version, nil
}

func (s *FilePersonaStore) Get(tenantID, versionID string) (PersonaVersion, error) {
	doc, err := s.load()
	if err != nil {
		return PersonaVersion{}, err
	}
	for _, version := range doc.Versions {
		if version.TenantID == tenantID && version.ID == versionID {
			return version, nil
		}
	}
	return PersonaVersion{}, ErrPersonaVersionNotFound
}

func (s *FilePersonaStore) List(tenantID string) ([]PersonaVersion, error) {
	doc, err := s.load()
	if err != nil {
		return nil, err
	}
	versions := make([]PersonaVersion, 0, len(doc.Versions))
	for _, version := range doc.Versions {
		if version.TenantID == tenantID {
			versions = append(versions, version)
		}
	}
	return versions, nil
}

func (s *FilePersonaStore) SetActive(tenantID, versionID string) error {
	if _, err := s.Get(tenantID, versionID); err != nil {
		return err
	}
	doc, err := s.load()
	if err != nil {
		return err
	}
	doc.Active[tenantID] = versionID
	return s.save(doc)
}

func (s *FilePersonaStore) Active(tenantID string) (PersonaVersion, error) {
	doc, err := s.load()
	if err != nil {
		return PersonaVersion{}, err
	}
	versionID := doc.Active[tenantID]
	if versionID == "" {
		return PersonaVersion{}, ErrPersonaVersionNotFound
	}
	for _, version := range doc.Versions {
		if version.TenantID == tenantID && version.ID == versionID {
			return version, nil
		}
	}
	return PersonaVersion{}, ErrPersonaVersionNotFound
}

func (s *FilePersonaStore) load() (personaStoreDocument, error) {
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return personaStoreDocument{Active: make(map[string]string)}, nil
	}
	if err != nil {
		return personaStoreDocument{}, err
	}
	var doc personaStoreDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return personaStoreDocument{}, err
	}
	if doc.Active == nil {
		doc.Active = make(map[string]string)
	}
	return doc, nil
}

func (s *FilePersonaStore) save(doc personaStoreDocument) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(), data, 0o600)
}

func (s *FilePersonaStore) path() string {
	return filepath.Join(s.dir, "personas.json")
}
