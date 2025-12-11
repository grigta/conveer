package database

import "errors"

var (
	ErrNotFound        = errors.New("document not found")
	ErrDuplicate       = errors.New("duplicate document")
	ErrInvalidID       = errors.New("invalid document ID")
	ErrConnection      = errors.New("database connection error")
	ErrTransaction     = errors.New("transaction error")
	ErrInvalidQuery    = errors.New("invalid query")
	ErrUnauthorized    = errors.New("unauthorized access")
	ErrInvalidData     = errors.New("invalid data")
	ErrOperationFailed = errors.New("operation failed")
)
