package core

import (
	"errors"
	"fmt"
)

var (
	// ErrAgentNotFound indicates that a requested agent identity is unknown.
	ErrAgentNotFound = errors.New("agent not found")
	// ErrSkillNotFound indicates that a requested skill identity is unknown.
	ErrSkillNotFound = errors.New("skill not found")
	// ErrDuplicateName indicates that a registry name is already in use.
	ErrDuplicateName = errors.New("duplicate name")
	// ErrLLMTimeout indicates that an LLM request exceeded its deadline.
	ErrLLMTimeout = errors.New("llm timeout")
	// ErrProviderFailure indicates that an external provider failed.
	ErrProviderFailure = errors.New("provider failure")
	// ErrStoreFailure indicates that persistence failed.
	ErrStoreFailure = errors.New("store failure")
	// ErrInvalidConfig indicates invalid runtime or domain configuration.
	ErrInvalidConfig = errors.New("invalid config")
	// ErrInvalidInput indicates malformed user or system input.
	ErrInvalidInput = errors.New("invalid input")
	// ErrUnauthorized indicates a denied operation.
	ErrUnauthorized = errors.New("unauthorized")
)

// Result wraps a value with an error for workflows that prefer explicit return envelopes.
type Result[T any] struct {
	Value T
	Err   error
}

// Ok returns a successful Result.
func Ok[T any](value T) Result[T] {
	return Result[T]{Value: value}
}

// Fail returns a failed Result.
func Fail[T any](err error) Result[T] {
	return Result[T]{Err: err}
}

// IsOK reports whether the Result contains no error.
func (r Result[T]) IsOK() bool {
	return r.Err == nil
}

// Unwrap returns the value and error in ordinary Go style.
func (r Result[T]) Unwrap() (T, error) {
	return r.Value, r.Err
}

// WrapError annotates err while preserving errors.Is compatibility.
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	if message == "" {
		return err
	}

	return fmt.Errorf("%s: %w", message, err)
}

// NewNamedError annotates a sentinel error with a resource kind and name.
func NewNamedError(err error, kind, name string) error {
	if err == nil {
		return nil
	}
	if kind == "" && name == "" {
		return err
	}
	return fmt.Errorf("%s %q: %w", kind, name, err)
}

func IsAgentNotFound(err error) bool {
	return errors.Is(err, ErrAgentNotFound)
}

func IsSkillNotFound(err error) bool {
	return errors.Is(err, ErrSkillNotFound)
}

func IsDuplicateName(err error) bool {
	return errors.Is(err, ErrDuplicateName)
}

func IsLLMTimeout(err error) bool {
	return errors.Is(err, ErrLLMTimeout)
}

func IsProviderFailure(err error) bool {
	return errors.Is(err, ErrProviderFailure)
}

func IsStoreFailure(err error) bool {
	return errors.Is(err, ErrStoreFailure)
}

func IsInvalidConfig(err error) bool {
	return errors.Is(err, ErrInvalidConfig)
}

func IsInvalidInput(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}
