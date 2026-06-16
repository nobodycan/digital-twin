package skills

import (
	"errors"
	"fmt"
)

// ErrInvalidParams marks invalid skill parameters.
var ErrInvalidParams = errors.New("invalid skill params")

// ParamType identifies supported skill parameter types.
type ParamType string

const (
	// String requires a string value.
	String ParamType = "string"
	// Bool requires a bool value.
	Bool ParamType = "bool"
	// Number requires an int, float64, or float32 value.
	Number ParamType = "number"
	// Object requires a map value.
	Object ParamType = "object"
	// StringSlice requires a []string value or []any containing only strings.
	StringSlice ParamType = "string_slice"
)

// Param describes one skill parameter.
type Param struct {
	Name     string
	Type     ParamType
	Required bool
	Default  any
}

// Spec validates and normalizes params for a skill.
type Spec struct {
	Params []Param
}

// Validate returns params with defaults applied.
func (s Spec) Validate(params map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(params)+len(s.Params))
	for key, value := range params {
		out[key] = value
	}

	for _, param := range s.Params {
		value, ok := out[param.Name]
		if !ok || value == nil {
			if param.Default != nil {
				out[param.Name] = param.Default
				continue
			}
			if param.Required {
				return nil, fmt.Errorf("%w: %s required", ErrInvalidParams, param.Name)
			}
			continue
		}
		normalized, err := normalizeParam(param.Type, value)
		if err != nil {
			return nil, fmt.Errorf("%w: %s must be %s", ErrInvalidParams, param.Name, param.Type)
		}
		out[param.Name] = normalized
	}

	return out, nil
}

func normalizeParam(kind ParamType, value any) (any, error) {
	switch kind {
	case String:
		v, ok := value.(string)
		if !ok {
			return nil, errors.New("not string")
		}
		return v, nil
	case Bool:
		v, ok := value.(bool)
		if !ok {
			return nil, errors.New("not bool")
		}
		return v, nil
	case Number:
		switch v := value.(type) {
		case int:
			return float64(v), nil
		case float64:
			return v, nil
		case float32:
			return float64(v), nil
		default:
			return nil, errors.New("not number")
		}
	case Object:
		v, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("not object")
		}
		return v, nil
	case StringSlice:
		switch v := value.(type) {
		case []string:
			return v, nil
		case []any:
			out := make([]string, 0, len(v))
			for _, item := range v {
				s, ok := item.(string)
				if !ok {
					return nil, errors.New("not string slice")
				}
				out = append(out, s)
			}
			return out, nil
		default:
			return nil, errors.New("not string slice")
		}
	default:
		return nil, errors.New("unknown param type")
	}
}
