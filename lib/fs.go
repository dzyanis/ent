package ent

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"hash"
	"io"
	"strings"
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

// MemoryFS is an in-memory implementation of FileSystem.
type MemoryFS struct {
	buckets map[*Bucket]map[string]File
}

// NewMemoryFS returns an instance of MemoryFS.
func NewMemoryFS() FileSystem {
	return &MemoryFS{
		buckets: map[*Bucket]map[string]File{},
	}
}

// Create given a Bucket and a key stores the content of src into a
// MemoryFile.
func (fs *MemoryFS) Create(
	bucket *Bucket,
	key string,
	src io.Reader,
) (File, error) {
	f := NewMemoryFile(key, nil)

	_, err := io.Copy(f, src)
	if err != nil {
		return nil, err
	}

	if _, ok := fs.buckets[bucket]; !ok {
		fs.buckets[bucket] = map[string]File{}
	}

	fs.buckets[bucket][f.Key()] = f

	return f, nil
}

// Delete removes the File stored in the given Bucket under key.
func (fs *MemoryFS) Delete(bucket *Bucket, key string) error {
	if _, ok := fs.buckets[bucket]; !ok {
		return nil
	}

	delete(fs.buckets[bucket], key)

	return nil
}

// Open returns the File stored under the key.
func (fs *MemoryFS) Open(bucket *Bucket, key string) (File, error) {
	if _, ok := fs.buckets[bucket]; !ok {
		return nil, ErrFileNotFound
	}

	f, ok := fs.buckets[bucket][key]
	if !ok {
		return nil, ErrFileNotFound
	}

	return f, nil
}

// List returns a list of Files matching the given criteria.
func (fs *MemoryFS) List(
	bucket *Bucket,
	prefix string,
	limit uint64,
	sort SortStrategy,
) (Files, error) {
	files := Files{}

	b, ok := fs.buckets[bucket]
	if !ok {
		return files, nil
	}

	for key, file := range b {
		if !strings.HasPrefix(key, prefix) {
			continue
		}

		files = append(files, file)
	}

	sort.Sort(files)

	if limit < uint64(len(files)) {
		files = files[:limit]
	}

	return files, nil
}

// MemoryFile is an in-memory implementation of the File interface meant for use
// in testing scenarios.
type MemoryFile struct {
	buffer *bytes.Buffer
	hash   hash.Hash
	index  int64
	key    string
	time   time.Time
}

// NewMemoryFile returns a MemoryFile.
func NewMemoryFile(key string, data []byte) File {
	if data == nil {
		data = []byte{}
	}

	f := &MemoryFile{
		buffer: bytes.NewBuffer(data),
		hash:   sha1.New(),
		key:    key,
		time:   time.Now(),
	}

	return f
}

// Close closes the File for further writes.
func (f *MemoryFile) Close() error {
	return nil
}

// Key returns the name of the file.
func (f *MemoryFile) Key() string {
	return f.key
}

// Hash returns the
func (f *MemoryFile) Hash() ([]byte, error) {
	return f.hash.Sum(nil), nil
}

// Read reads up to len(b) from File.
func (f *MemoryFile) Read(b []byte) (int, error) {
	return f.buffer.Read(b)
}

// Seek sets the offset for the next Read or Write on File.
func (f *MemoryFile) Seek(offset int64, whence int) (int64, error) {
	var abs int64

	switch whence {
	case 0:
		abs = offset
	case 1:
		abs = f.index + offset
	case 2:
		abs = int64(f.buffer.Len()) + offset
	default:
		return 0, errors.New("MemoryFile.Seek: invalid whence")
	}

	if abs < 0 {
		return 0, errors.New("MemoryFile.Seek: negative position")
	}

	f.index = abs

	return abs, nil
}

// Write writes len(b) bytes to File.
func (f *MemoryFile) Write(b []byte) (int, error) {
	n, err := f.hash.Write(b)
	if err != nil {
		return n, err
	}

	n, err = f.buffer.Write(b)
	if err != nil {
		return n, err
	}

	f.index = int64(f.buffer.Len())

	return n, nil
}

// LastModified returns the time of last modification.
func (f *MemoryFile) LastModified() time.Time {
	return f.time
}
