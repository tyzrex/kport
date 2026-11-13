package app

import (
	"context"
	"errors"
	"strings"

	"github.com/tyzrex/kport"
)

type ErrorCode string

const (
	CodeCancelled           ErrorCode = "cancelled"
	CodeTimedOut            ErrorCode = "timed_out"
	CodeDiscoveryFailed     ErrorCode = "discovery_failed"
	CodePermissionDenied    ErrorCode = "permission_denied"
	CodeProtectedTarget     ErrorCode = "protected_target"
	CodeIdentityUnavailable ErrorCode = "identity_unavailable"
	CodeIdentityChanged     ErrorCode = "identity_changed"
	CodeDockerUnavailable   ErrorCode = "docker_unavailable"
	CodeMappingChanged      ErrorCode = "mapping_changed"
	CodeStalePlan           ErrorCode = "stale_plan"
	CodeActionFailed        ErrorCode = "action_failed"
)

func ClassifyError(err error) ErrorCode {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return CodeCancelled
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
		return CodeTimedOut
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "protected"):
		return CodeProtectedTarget
	case strings.Contains(message, "cannot verify"):
		return CodeIdentityUnavailable
	case strings.Contains(message, "identity") && strings.Contains(message, "refresh"), strings.Contains(message, "tree changed"):
		return CodeIdentityChanged
	case strings.Contains(message, "container mapping changed"):
		return CodeMappingChanged
	case strings.Contains(message, "docker") || strings.Contains(message, "container"):
		return CodeDockerUnavailable
	case strings.Contains(message, "permission denied") || strings.Contains(message, "operation not permitted"):
		return CodePermissionDenied
	case strings.Contains(message, "discover") || strings.Contains(message, "lsof") || strings.Contains(message, "netstat"):
		return CodeDiscoveryFailed
	default:
		return CodeActionFailed
	}
}

func setErrorCode(r *killport.KillResult) { r.ErrorCode = string(ClassifyError(r.Error)) }
