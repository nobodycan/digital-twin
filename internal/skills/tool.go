package skills

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/nobodycan/digital-twin/pkg/types"
)

// ErrDenied marks a policy-denied skill call.
var ErrDenied = errors.New("skill call denied")

type HTTPCallSkill struct {
	allowlist map[string]struct{}
}

func NewHTTPCallSkill(hosts []string) HTTPCallSkill {
	allowlist := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		allowlist[host] = struct{}{}
	}
	return HTTPCallSkill{allowlist: allowlist}
}

func (s HTTPCallSkill) Name() string { return "http_call" }

func (s HTTPCallSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{{Name: "url", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	rawURL := valid["url"].(string)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return types.SkillResult{}, fmt.Errorf("%w: url invalid", ErrInvalidParams)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return types.SkillResult{}, fmt.Errorf("%w: unsupported scheme %s", ErrDenied, parsed.Scheme)
	}
	host := parsed.Hostname()
	if isLocalOrPrivate(host) {
		return types.SkillResult{}, fmt.Errorf("%w: local or private target %s", ErrDenied, host)
	}
	if _, ok := s.allowlist[host]; !ok {
		return types.SkillResult{}, fmt.Errorf("%w: host not allowlisted %s", ErrDenied, host)
	}
	return types.SkillResult{
		SkillName: s.Name(),
		Output:    "allowed",
		Metadata:  types.Metadata{"allowed": true, "host": host},
	}, nil
}

type SearchWebSkill struct{}

func NewSearchWebSkill() SearchWebSkill { return SearchWebSkill{} }
func (s SearchWebSkill) Name() string   { return "search_web" }

func (s SearchWebSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{{Name: "query", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	return types.SkillResult{
		SkillName: s.Name(),
		Output:    []string{},
		Metadata:  types.Metadata{"placeholder": true, "query": valid["query"]},
	}, nil
}

type CalendarSkill struct{}

func NewCalendarSkill() CalendarSkill { return CalendarSkill{} }
func (s CalendarSkill) Name() string  { return "calendar" }

func (s CalendarSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{{Name: "action", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	return types.SkillResult{
		SkillName: s.Name(),
		Output:    valid["action"],
		Metadata:  types.Metadata{"placeholder": true},
	}, nil
}

func isLocalOrPrivate(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return host == "localhost"
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}
