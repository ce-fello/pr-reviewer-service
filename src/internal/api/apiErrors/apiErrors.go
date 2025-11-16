package apiErrors

import "fmt"

type ErrorCode string

const (
	TeamExists      ErrorCode = "TEAM_EXISTS"
	PRExists        ErrorCode = "PR_EXISTS"
	PRAlreadyMerged ErrorCode = "PR_MERGED"
	NotAssigned     ErrorCode = "NOT_ASSIGNED"
	NoCandidate     ErrorCode = "NO_CANDIDATE"
	NotFound        ErrorCode = "NOT_FOUND"
	InternalError   ErrorCode = "INTERNAL_ERROR"
)

type APIError struct {
	Code    ErrorCode
	Message string
}

func (e APIError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}
