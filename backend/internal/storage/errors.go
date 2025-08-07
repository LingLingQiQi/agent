package storage

import "errors"

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrMessageNotFound = errors.New("message not found")
	ErrInvalidData     = errors.New("invalid data")
	ErrStorageInit     = errors.New("storage initialization failed")
	ErrFileOperation   = errors.New("file operation failed")
)