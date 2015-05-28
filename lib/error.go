package ent

import (
	"errors"
)

// Error codes returned by Ent for missing entities.
var (
	ErrBucketNotFound = errors.New("bucket not found")
	ErrEmptyBucket    = errors.New("bucket not provided")
	ErrFileNotFound   = errors.New("file not found")
	ErrInvalidParam   = errors.New("invalid param")
)

// IsBucketNotFound returns a boolean indicating the error is
// ErrBucketNotFound.
func IsBucketNotFound(err error) bool {
	switch err.(type) {
	case nil:
		return false
	}
	return err == ErrBucketNotFound
}

// IsFileNotFound returns a boolean indicating the error is
// ErrFileNotFound.
func IsFileNotFound(err error) bool {
	switch err.(type) {
	case nil:
		return false
	}
	return err == ErrFileNotFound
}
