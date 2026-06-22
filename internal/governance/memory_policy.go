package governance

import (
	"errors"
	"regexp"
	"strings"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

var ErrMemoryWriteDenied = errors.New("memory write denied")

type PolicyAction string

const (
	PolicyAllow PolicyAction = "allow"
	PolicyDeny  PolicyAction = "deny"
)

type PolicyDecision struct {
	Action      PolicyAction   `json:"action"`
	Reason      string         `json:"reason,omitempty"`
	Explanation string         `json:"explanation,omitempty"`
	Evidence    types.Metadata `json:"evidence,omitempty"`
}

type MemoryWritePolicy struct{}

var sensitiveMemoryPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bapi[_ -]?key\b`),
	regexp.MustCompile(`(?i)\bpassword\b`),
	regexp.MustCompile(`(?i)\btoken\b`),
	regexp.MustCompile(`(?i)\bsk-[a-z0-9_-]+`),
}

func (MemoryWritePolicy) Decide(summary string) PolicyDecision {
	text := strings.TrimSpace(summary)
	if text == "" {
		return PolicyDecision{Action: PolicyDeny, Reason: "empty_memory", Explanation: "empty memories are not useful enough to persist"}
	}
	for _, pattern := range sensitiveMemoryPatterns {
		if pattern.MatchString(text) {
			return PolicyDecision{
				Action:      PolicyDeny,
				Reason:      "sensitive_secret",
				Explanation: "memory contains secret-like data and must not be persisted",
				Evidence:    types.Metadata{"pattern": pattern.String()},
			}
		}
	}
	return PolicyDecision{Action: PolicyAllow, Reason: "stable_non_sensitive"}
}

func MemoryDeniedError(decision PolicyDecision) error {
	return core.WrapError(ErrMemoryWriteDenied, decision.Reason)
}
