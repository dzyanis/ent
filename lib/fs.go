package ent

import (
	"io"
	"time"
)

// A FileSystem implements CRUD operations for a collection of named files
// namespaced into buckets.
type FileSystem interface {
	Create(bucket *Bucket, key string, data io.Reader) (File, error)
	Delete(bucket *Bucket, key string) error
	Open(bucket *Bucket, key string) (File, error)
	List(bucket *Bucket, prefix string, limit uint64, sort SortStrategy) (Files, error)
}

// File represents a handle to an open file handle.
type File interface {
	Hash() ([]byte, error)
	Key() string
	LastModified() time.Time

	io.Closer
	io.Reader
	io.Seeker
	io.Writer
}

// Files represents group of file
type Files []File
