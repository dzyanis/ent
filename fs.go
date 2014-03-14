// Copyright (c) 2014, SoundCloud Ltd.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/ent

package main

import (
	"crypto/sha1"
	"errors"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// A FileSystem implements CRUD operations for a collection of named files
// namespaced into buckets.
type FileSystem interface {
	Create(bucket *Bucket, key string, data io.Reader) (File, error)
	Open(bucket *Bucket, key string) (File, error)
}

type diskFS struct {
	root string
}

func (fs *diskFS) Create(bucket *Bucket, key string, r io.Reader) (File, error) {
	destination := filepath.Join(fs.root, bucket.Name, key)

	err := os.MkdirAll(filepath.Dir(destination), 0755)
	if err != nil {
		return nil, err
	}

	tmp, err := ioutil.TempFile(filepath.Join(fs.root, bucket.Name), "pending-")
	if err != nil {
		return nil, err
	}
	defer tmp.Close()

	f := newFile(tmp)

	_, err = io.Copy(f, r)
	if err != nil {
		return nil, errors.New("storing failed")
	}

	err = os.Rename(tmp.Name(), destination)
	if err != nil {
		return nil, errors.New("rename failed")
	}

	f.File, err = os.Open(destination)
	if err != nil {
		return nil, errors.New("open failed")
	}

	return f, nil
}

func (fs *diskFS) Open(bucket *Bucket, key string) (File, error) {
	f, err := os.Open(filepath.Join(fs.root, bucket.Name, key))
	if err != nil {
		return nil, err
	}
	return newFile(f), nil
}

// NewDiskFS returns a new disk backed FileSystem given a rooth path.
func NewDiskFS(root string) FileSystem {
	return &diskFS{
		root: root,
	}
}

// File represents a handle to an open file handle.
type File interface {
	Hash() ([]byte, error)

	io.Closer
	io.Reader
	io.Seeker
	io.Writer
}

type file struct {
	hash   hash.Hash
	hashed int64

	*os.File
}

func (f *file) Hash() ([]byte, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if f.hashed == fi.Size() {
		return f.hash.Sum(nil), nil
	}

	f.hash.Reset()
	f.hashed = 0

	_, err = f.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	n, err := io.Copy(f.hash, f)
	if err != nil {
		return nil, err
	}

	f.hashed += int64(n)

	return f.hash.Sum(nil), nil
}

func (f *file) Write(p []byte) (int, error) {
	n, err := f.hash.Write(p)
	if err != nil {
		return n, err
	}
	f.hashed += int64(n)

	return f.File.Write(p)
}

func newFile(f *os.File) *file {
	return &file{
		hash:   sha1.New(),
		hashed: 0,
		File:   f,
	}
}
