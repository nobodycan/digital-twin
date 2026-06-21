package admin

import (
	"testing"

	"github.com/nobodycan/digital-twin/internal/persona"
)

func TestPersonaServicePublishesDraftAndReturnsActiveVersion(t *testing.T) {
	service := NewPersonaService(NewInMemoryPersonaStore())
	draft := validAdminPersona()

	version, err := service.SaveDraft("tenant-1", draft)
	if err != nil {
		t.Fatalf("SaveDraft returned error: %v", err)
	}
	if version.Status != PersonaDraft {
		t.Fatalf("status = %q, want draft", version.Status)
	}

	published, err := service.Publish("tenant-1", version.ID)
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	if published.Status != PersonaPublished {
		t.Fatalf("published status = %q", published.Status)
	}

	active, err := service.Active("tenant-1")
	if err != nil {
		t.Fatalf("Active returned error: %v", err)
	}
	if active.ID != published.ID || active.Persona.Identity != draft.Identity {
		t.Fatalf("active = %#v, want published draft", active)
	}
}

func TestPersonaServiceRejectsInvalidDraftPublish(t *testing.T) {
	service := NewPersonaService(NewInMemoryPersonaStore())
	version, err := service.SaveDraft("tenant-1", persona.Persona{Identity: "Ava"})
	if err != nil {
		t.Fatalf("SaveDraft returned error: %v", err)
	}

	if _, err := service.Publish("tenant-1", version.ID); err == nil {
		t.Fatalf("expected invalid draft publish to fail")
	}
}

func TestPersonaServiceRollsBackToPreviousPublishedVersion(t *testing.T) {
	service := NewPersonaService(NewInMemoryPersonaStore())
	first, err := service.SaveDraft("tenant-1", validAdminPersona())
	if err != nil {
		t.Fatalf("SaveDraft first: %v", err)
	}
	if _, err := service.Publish("tenant-1", first.ID); err != nil {
		t.Fatalf("Publish first: %v", err)
	}
	secondPersona := validAdminPersona()
	secondPersona.Identity = "Nova"
	second, err := service.SaveDraft("tenant-1", secondPersona)
	if err != nil {
		t.Fatalf("SaveDraft second: %v", err)
	}
	if _, err := service.Publish("tenant-1", second.ID); err != nil {
		t.Fatalf("Publish second: %v", err)
	}

	rolledBack, err := service.Rollback("tenant-1", first.ID)
	if err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}
	if rolledBack.ID != first.ID {
		t.Fatalf("rolled back ID = %q, want %q", rolledBack.ID, first.ID)
	}

	active, err := service.Active("tenant-1")
	if err != nil {
		t.Fatalf("Active returned error: %v", err)
	}
	if active.Persona.Identity != "Ava" {
		t.Fatalf("active identity = %q, want Ava", active.Persona.Identity)
	}
}

func TestFilePersonaStorePersistsPublishedActiveVersion(t *testing.T) {
	dir := t.TempDir()
	first := NewPersonaService(NewFilePersonaStore(dir))
	draft, err := first.SaveDraft("tenant-1", validAdminPersona())
	if err != nil {
		t.Fatalf("SaveDraft returned error: %v", err)
	}
	published, err := first.Publish("tenant-1", draft.ID)
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	second := NewPersonaService(NewFilePersonaStore(dir))
	active, err := second.Active("tenant-1")
	if err != nil {
		t.Fatalf("Active after reopen returned error: %v", err)
	}
	if active.ID != published.ID || active.Persona.Identity != "Ava" {
		t.Fatalf("active after reopen = %#v", active)
	}
}

func validAdminPersona() persona.Persona {
	return persona.Persona{
		ID:            "advisor",
		Identity:      "Ava",
		Role:          "professional digital advisor",
		Tone:          []string{"calm", "precise"},
		Boundaries:    []string{"state uncertainty when confidence is low"},
		AllowedClaims: []string{"can explain planning tradeoffs"},
		Locale:        "en-US",
	}
}
