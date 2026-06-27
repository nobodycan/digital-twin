package types

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Metadata carries extension fields across contracts.
type Metadata map[string]any

// Role identifies the author of a message.
type Role string

const (
	// RoleSystem marks instructions supplied by the system.
	RoleSystem Role = "system"
	// RoleUser marks messages supplied by the end user.
	RoleUser Role = "user"
	// RoleAssistant marks messages produced by an agent or model.
	RoleAssistant Role = "assistant"
	// RoleTool marks tool or skill output messages.
	RoleTool Role = "tool"
)

// Valid reports whether role is a known chat role.
func (r Role) Valid() bool {
	switch r {
	case RoleSystem, RoleUser, RoleAssistant, RoleTool:
		return true
	default:
		return false
	}
}

// Confidence is a normalized confidence score from 0 to 1.
type Confidence float64

// Valid reports whether confidence is within the normalized range.
func (c Confidence) Valid() bool {
	return c >= 0 && c <= 1
}

// IntentName identifies a routed user intent.
type IntentName string

const (
	// IntentUnknown marks an unclassified intent.
	IntentUnknown IntentName = "unknown"
	// IntentKnowledgeQuery marks a knowledge retrieval request.
	IntentKnowledgeQuery IntentName = "knowledge.query"
	// IntentMemoryRecall marks a request that should consult memory.
	IntentMemoryRecall IntentName = "memory.recall"
	// IntentTaskExecution marks a task-oriented request.
	IntentTaskExecution IntentName = "task.execute"
	// IntentToolCall marks a request that should call a tool.
	IntentToolCall IntentName = "tool.call"
	// IntentPersonaChat marks small talk or persona fallback.
	IntentPersonaChat IntentName = "persona.chat"
	// IntentSafetyCheck marks a request that should be evaluated by safety logic.
	IntentSafetyCheck IntentName = "safety.check"
)

// Message is the serializable unit of conversation.
type Message struct {
	ID        string    `json:"id"`
	Role      Role      `json:"role"`
	Content   string    `json:"content"`
	Name      string    `json:"name,omitempty"`
	Metadata  Metadata  `json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Conversation groups messages by tenant and user.
type Conversation struct {
	ID        string       `json:"id"`
	TenantID  string       `json:"tenant_id"`
	UserID    string       `json:"user_id"`
	Messages  []Message    `json:"messages"`
	Turns     []TurnRecord `json:"turns,omitempty"`
	Metadata  Metadata     `json:"metadata,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// Intent describes a routed user request.
type Intent struct {
	Name       IntentName `json:"name"`
	Query      string     `json:"query"`
	Confidence Confidence `json:"confidence"`
	Entities   Metadata   `json:"entities,omitempty"`
	Metadata   Metadata   `json:"metadata,omitempty"`
}

// AgentResult is the structured output from an agent.
type AgentResult struct {
	AgentName  string     `json:"agent_name"`
	Message    Message    `json:"message"`
	Confidence Confidence `json:"confidence"`
	Metadata   Metadata   `json:"metadata,omitempty"`
}

// SkillResult is the structured output from a skill.
type SkillResult struct {
	SkillName string   `json:"skill_name"`
	Output    any      `json:"output,omitempty"`
	Metadata  Metadata `json:"metadata,omitempty"`
}

// UserProfile contains user-level personalization fields.
type UserProfile struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Locale      string   `json:"locale,omitempty"`
	Timezone    string   `json:"timezone,omitempty"`
	Metadata    Metadata `json:"metadata,omitempty"`
}

// Tenant contains tenant-level identity fields.
type Tenant struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Metadata Metadata `json:"metadata,omitempty"`
}

type TurnStatus string

const (
	TurnOpen      TurnStatus = "open"
	TurnCompleted TurnStatus = "completed"
	TurnFailed    TurnStatus = "failed"
	TurnCanceled  TurnStatus = "canceled"
)

type AttemptStatus string

const (
	AttemptGenerating AttemptStatus = "generating"
	AttemptCompleted  AttemptStatus = "completed"
	AttemptFailed     AttemptStatus = "failed"
	AttemptCanceled   AttemptStatus = "canceled"
	AttemptAbandoned  AttemptStatus = "abandoned"
	AttemptReplayed   AttemptStatus = "replayed"
)

type TurnRequest struct {
	ConversationID string   `json:"conversation_id"`
	TenantID       string   `json:"tenant_id"`
	UserID         string   `json:"user_id"`
	TurnID         string   `json:"turn_id"`
	AttemptID      string   `json:"attempt_id"`
	Message        Message  `json:"message"`
	Metadata       Metadata `json:"metadata,omitempty"`
}

type TurnAttempt struct {
	ID          string        `json:"id"`
	Status      AttemptStatus `json:"status"`
	RequestID   string        `json:"request_id,omitempty"`
	ErrorCode   string        `json:"error_code,omitempty"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at,omitempty"`
}

type TurnRecord struct {
	ID                 string        `json:"id"`
	UserMessageID      string        `json:"user_message_id"`
	AssistantMessageID string        `json:"assistant_message_id,omitempty"`
	Status             TurnStatus    `json:"status"`
	Attempts           []TurnAttempt `json:"attempts"`
	Result             *AgentResult  `json:"result,omitempty"`
}

type StreamEventName string

const (
	StreamEventRequestStarted   StreamEventName = "request_started"
	StreamEventRouteSelected    StreamEventName = "route_selected"
	StreamEventAgentSelected    StreamEventName = "agent_selected"
	StreamEventAssistantDelta   StreamEventName = "assistant_text_delta"
	StreamEventFallbackSelected StreamEventName = "fallback_selected"
	StreamEventMessageCompleted StreamEventName = "message_completed"
	StreamEventCanceled         StreamEventName = "canceled"
	StreamEventError            StreamEventName = "error"
	StreamEventDone             StreamEventName = "done"
)

type StreamEvent struct {
	Name           StreamEventName `json:"name"`
	RequestID      string          `json:"request_id"`
	TenantID       string          `json:"tenant_id"`
	UserID         string          `json:"user_id"`
	ConversationID string          `json:"conversation_id"`
	TurnID         string          `json:"turn_id"`
	AttemptID      string          `json:"attempt_id"`
	Sequence       uint64          `json:"sequence"`
	Timestamp      time.Time       `json:"timestamp"`
	Payload        Metadata        `json:"payload,omitempty"`
	Metadata       Metadata        `json:"metadata,omitempty"`
}

var contractIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func (r TurnRequest) Validate() error {
	for field, value := range map[string]string{
		"conversation_id": r.ConversationID,
		"tenant_id":       r.TenantID,
		"user_id":         r.UserID,
		"turn_id":         r.TurnID,
		"attempt_id":      r.AttemptID,
		"message.id":      r.Message.ID,
	} {
		if !validContractID(value) {
			return fmt.Errorf("%s: invalid id", field)
		}
	}
	if r.Message.Role != RoleUser {
		return fmt.Errorf("message.role: expected %q", RoleUser)
	}
	if strings.TrimSpace(r.Message.Content) == "" {
		return fmt.Errorf("message.content: required")
	}
	return nil
}

func (r TurnRecord) Validate() error {
	if !validContractID(r.ID) || !validContractID(r.UserMessageID) {
		return fmt.Errorf("turn record ids: invalid")
	}
	if !r.Status.Valid() {
		return fmt.Errorf("status: invalid")
	}
	for _, attempt := range r.Attempts {
		if err := attempt.Validate(); err != nil {
			return err
		}
	}
	if r.Result != nil && r.Result.Message.Role != RoleAssistant {
		return fmt.Errorf("result.message.role: expected %q", RoleAssistant)
	}
	return nil
}

func (a TurnAttempt) Validate() error {
	if !validContractID(a.ID) {
		return fmt.Errorf("attempt.id: invalid")
	}
	if !a.Status.Valid() {
		return fmt.Errorf("attempt.status: invalid")
	}
	return nil
}

func (e StreamEvent) Validate() error {
	if !e.Name.Valid() {
		return fmt.Errorf("name: invalid")
	}
	for field, value := range map[string]string{
		"request_id":      e.RequestID,
		"tenant_id":       e.TenantID,
		"user_id":         e.UserID,
		"conversation_id": e.ConversationID,
		"turn_id":         e.TurnID,
		"attempt_id":      e.AttemptID,
	} {
		if !validContractID(value) {
			return fmt.Errorf("%s: invalid id", field)
		}
	}
	if e.Sequence == 0 {
		return fmt.Errorf("sequence: required")
	}
	if e.Timestamp.IsZero() {
		return fmt.Errorf("timestamp: required")
	}
	return nil
}

func (s TurnStatus) Valid() bool {
	switch s {
	case TurnOpen, TurnCompleted, TurnFailed, TurnCanceled:
		return true
	default:
		return false
	}
}

func (s AttemptStatus) Valid() bool {
	switch s {
	case AttemptGenerating, AttemptCompleted, AttemptFailed, AttemptCanceled, AttemptAbandoned, AttemptReplayed:
		return true
	default:
		return false
	}
}

func (n StreamEventName) Valid() bool {
	switch n {
	case StreamEventRequestStarted,
		StreamEventRouteSelected,
		StreamEventAgentSelected,
		StreamEventAssistantDelta,
		StreamEventFallbackSelected,
		StreamEventMessageCompleted,
		StreamEventCanceled,
		StreamEventError,
		StreamEventDone:
		return true
	default:
		return false
	}
}

func validContractID(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, `/\`) && contractIDPattern.MatchString(value)
}
