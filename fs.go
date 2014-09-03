package main

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/soundcloud/ent/lib"
)

type diskFS struct {
	root string
}

func newDiskFS(root string) ent.FileSystem {
	return &diskFS{
		root: root,
	}
}

func (fs *diskFS) Create(bucket *ent.Bucket, key string, r io.Reader) (ent.File, error) {
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

	f := newFile(tmp, key)

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

	stat, err := f.File.Stat()
	if err != nil {
		return nil, err
	}

	f.lastModified = stat.ModTime()

	return f, nil
}

func (fs *diskFS) Open(bucket *ent.Bucket, key string) (ent.File, error) {
	f, err := os.Open(filepath.Join(fs.root, bucket.Name, key))
	if err != nil {
		if os.IsNotExist(err) {
			err = ent.ErrFileNotFound
		}
		return nil, err
	}
	return newFile(f, key), nil
}

func (fs *diskFS) List(bucket *ent.Bucket, prefix string, limit uint64, sortStrategy ent.SortStrategy) (ent.Files, error) {
	var (
		files      = ent.Files{}
		bucketDir  = filepath.Join(fs.root, bucket.Name)
		prefixGlob = filepath.Join(bucketDir, prefix)
	)

	err := filepath.Walk(bucketDir, listWalk(&files, prefixGlob, bucketDir))
	if err != nil {
		return nil, err
	}

	sortStrategy.Sort(files)

	if limit < uint64(len(files)) {
		files = files[:limit]
	}

	return files, nil
}

type file struct {
	hash         hash.Hash
	hashed       int64
	key          string
	lastModified time.Time

	*os.File
}

func newFile(f *os.File, key string) *file {
	return &file{
		hash:   sha1.New(),
		hashed: 0,
		key:    key,
		File:   f,
	}
}

func (f *file) Key() string {
	return f.key
}

func (f *file) LastModified() time.Time {
	return f.lastModified
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

func listWalk(files *ent.Files, prefix string, bucketDir string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking tree: %s", err)
		}

		if !info.IsDir() && strings.HasPrefix(path, prefix) {
			fd, err := os.Open(path)
			if err != nil {
				return err
			}

			stat, err := fd.Stat()
			if err != nil {
				return err
			}

			// The key is without leading slash.
			f := newFile(fd, strings.TrimPrefix(path, bucketDir+"/"))
			f.lastModified = stat.ModTime()

			*files = append(*files, f)
		}

		return nil
	}
}
