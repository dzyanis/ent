// Copyright (c) 2014, SoundCloud Ltd.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/ent

package main

import (
	"errors"
)

// Error codes returned by Ent for missing entities.
var (
	ErrBucketNotFound = errors.New("bucket not found")
	ErrFileNotFound   = errors.New("file not found")
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
