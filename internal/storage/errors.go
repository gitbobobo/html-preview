package storage

import "errors"

var (
	ErrHTMLTooLarge            = errors.New("html file too large")
	ErrZipTooLarge             = errors.New("zip file too large")
	ErrZipInvalid              = errors.New("invalid zip archive")
	ErrZipTooManyFiles         = errors.New("too many files in zip")
	ErrZipPathTraversal        = errors.New("path traversal")
	ErrZipAbsolutePath         = errors.New("absolute path")
	ErrZipSymlink              = errors.New("symlink not allowed")
	ErrZipNested               = errors.New("nested zip not allowed")
	ErrZipDecompressedTooLarge = errors.New("zip decompressed size too large")
	ErrZipMissingIndex         = errors.New("zip must contain index.html at root")
	ErrZipEmptyPath            = errors.New("empty path")
	ErrZipBackslashPath        = errors.New("backslash in path")
)
