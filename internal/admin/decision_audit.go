package admin

import (
	"fmt"

	"github.com/nobodycan/digital-twin/internal/governance"
)

type DecisionAuditExporter struct {
	Audit AuditService
}

func (e DecisionAuditExporter) RecordDecision(decision governance.DecisionRecord) (AuditRecord, error) {
	summary := []string{fmt.Sprintf("governance:%s", decision.Type)}
	if raw, ok := decision.Evidence["decision"]; ok {
		summary = append(summary, fmt.Sprintf("decision:%v", raw))
	}
	return e.Audit.Record(decision.TenantID, AuditRecord{
		ID:             "audit-governance-" + decision.ID,
		ConversationID: "governance-" + decision.ID,
		UserID:         decision.ActorID,
		Status:         AuditStatusCompleted,
		AgentName:      "governance",
		EventSummary:   summary,
		CreatedAt:      decision.CreatedAt,
	})
}
