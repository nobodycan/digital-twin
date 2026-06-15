package types

import "time"

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
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	UserID    string    `json:"user_id"`
	Messages  []Message `json:"messages"`
	Metadata  Metadata  `json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
