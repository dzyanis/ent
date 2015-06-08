package ent

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryFSCreate(t *testing.T) {
	var (
		b   = NewBucket("create", Owner{})
		fs  = NewMemoryFS()
		h   = sha1.New()
		p   = "../fixture/test.zip"
		key = filepath.Base(p)
	)

	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	tr := io.TeeReader(f, h)

	_, err = fs.Create(b, key, tr)
	if err != nil {
		t.Fatal(err)
	}

	file, err := fs.Open(b, key)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	s := sha1.New()

	_, err = io.Copy(s, file)
	if err != nil {
		t.Fatal(err)
	}

	var (
		have = hex.EncodeToString(s.Sum(nil))
		want = hex.EncodeToString(h.Sum(nil))
	)

	if have != want {
		t.Errorf("have %s, want %s", have, want)
	}
}

func TestMemoryFSDelete(t *testing.T) {
	var (
		b   = NewBucket("delete", Owner{})
		fs  = NewMemoryFS()
		p   = "../fixture/test.zip"
		key = filepath.Base(p)
	)

	_, err := fs.Open(b, key)
	if have, want := err, ErrFileNotFound; !IsFileNotFound(err) {
		t.Errorf("have %v, want %v", have, want)
	}

	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	_, err = fs.Create(b, key, f)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fs.Open(b, key)
	if err != nil {
		t.Fatal(err)
	}

	err = fs.Delete(b, key)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fs.Open(b, key)
	if have, want := err, ErrFileNotFound; !IsFileNotFound(err) {
		t.Errorf("have %v, want %v", have, want)
	}
}
