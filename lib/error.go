package ent

import (
	"errors"
	"fmt"
)

// Error codes returned by Ent for missing entities.
var (
	ErrBucketNotFound = errors.New("bucket not found")
	ErrClient         = errors.New("ent.Client")
	ErrEmptyBucket    = errors.New("bucket not provided")
	ErrEmptyKey       = errors.New("key not provided")
	ErrEmptySource    = errors.New("source not provided")
	ErrFileNotFound   = errors.New("file not found")
	ErrInvalidParam   = errors.New("invalid param")
)

// Error is a wrapper for Ent returned errors.
type Error struct {
	err error
	msg string
}

func newError(err error, msg string) error {
	return &Error{
		err: err,
		msg: msg,
	}
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s %s", e.err, e.msg)
}

// IsBucketNotFound returns a boolean indicating the error is
// ErrBucketNotFound.
func IsBucketNotFound(err error) bool {
	return unwrapErr(err) == ErrBucketNotFound
}

// IsClient returns a boolean indicating if the error is ErrClient.
func IsClient(err error) bool {
	return unwrapErr(err) == ErrClient
}

// IsEmptyBucket returns a boolean indicating if the error is ErrEmptyBucket.
func IsEmptyBucket(err error) bool {
	return unwrapErr(err) == ErrEmptyBucket
}

// IsEmptyKey returns a boolean indicating if the error is ErrEmptyKey.
func IsEmptyKey(err error) bool {
	return unwrapErr(err) == ErrEmptyKey
}

// IsEmptySource returns a boolean indicating if the error is ErrEmptySource
func IsEmptySource(err error) bool {
	return unwrapErr(err) == ErrEmptySource
}

// IsFileNotFound returns a boolean indicating the error is
// ErrFileNotFound.
func IsFileNotFound(err error) bool {
	return unwrapErr(err) == ErrFileNotFound
}

func unwrapErr(err error) error {
	switch e := err.(type) {
	case *Error:
		return e.err
	}
	return err
}
