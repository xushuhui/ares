package errors

import (
	"net/http"
	"strconv"
)

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// implement error interface
func (e *Error) Error() string {
	return "code=" + strconv.Itoa(e.Code) + ", message=" + e.Message
}

func New(code int, message string) error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

func BadRequest(message string) error {
	return New(http.StatusBadRequest, message)
}

func NotFound(message string) error {
	return New(http.StatusNotFound, message)
}

func InternalError(message string) error {
	return New(http.StatusInternalServerError, message)
}

func Forbidden(message string) error {
	return New(http.StatusForbidden, message)
}

func Unauthorized(message string) error {
	return New(http.StatusUnauthorized, message)
}
