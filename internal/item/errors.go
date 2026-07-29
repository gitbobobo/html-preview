package item

import "errors"

var (
	ErrNotFound            = errors.New("item not found")
	ErrConflict            = errors.New("item state conflict")
	ErrBadStatus           = errors.New("invalid status")
	ErrFilenameRequired    = errors.New("filename is required")
	ErrUnsupportedFileType = errors.New("unsupported file type")
	ErrInvalidExpiresAt    = errors.New("invalid expires_at")
	ErrInvalidExpiresIn    = errors.New("invalid expires_in")
	ErrInvalidThumbVariant = errors.New("invalid thumb variant")
)
