package errors

const (
	CodeAuthInvalid        = "AUTH_INVALID"
	CodeAuthExpired        = "AUTH_EXPIRED"
	CodeAuthReplay         = "AUTH_REPLAY"
	CodeServerFull         = "SERVER_FULL"
	CodeRateLimited        = "RATE_LIMITED"
	CodeBadRequest         = "BAD_REQUEST"
	CodeRequestIDConflict  = "REQUEST_ID_CONFLICT"
	CodeUnsupported        = "UNSUPPORTED_OP"
	CodeNotFound           = "NOT_FOUND"
	CodeInsufficient       = "INSUFFICIENT_RESOURCE"
	CodeAlreadyMax         = "ALREADY_MAX"
	CodePreconditionFailed = "PRECONDITION_FAILED"
	CodeInternal           = "INTERNAL_ERROR"
)

type BizError struct {
	Code string
	Msg  string
}

func (e BizError) Error() string { return e.Code + ": " + e.Msg }
