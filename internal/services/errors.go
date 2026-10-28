package services

import "fmt"

// AppError es un error de negocio seguro para mostrar al usuario.
type AppError struct {
	Status  int
	Message string
}

func (e *AppError) Error() string { return e.Message }

func BadRequest(msg string) error   { return &AppError{Status: 400, Message: msg} }
func Unauthorized(msg string) error { return &AppError{Status: 401, Message: msg} }
func Forbidden(msg string) error    { return &AppError{Status: 403, Message: msg} }
func NotFound(msg string) error     { return &AppError{Status: 404, Message: msg} }
func Conflict(msg string) error     { return &AppError{Status: 409, Message: msg} }
func Internal(msg string, err error) error {
	if err == nil {
		return &AppError{Status: 500, Message: msg}
	}
	return &AppError{Status: 500, Message: fmt.Sprintf("%s: %v", msg, err)}
}
