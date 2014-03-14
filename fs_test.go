// Copyright (c) 2014, SoundCloud Ltd.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/ent

package main

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestDiskFS(t *testing.T) {
	tmp, err := ioutil.TempDir("", "ent-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	b := NewBucket("create", Owner{})
	fs := NewDiskFS(tmp)

	testFile := "./fixture/test.zip"
	h := sha1.New()
	r, err := os.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	tr := io.TeeReader(r, h)

	_, err = fs.Create(b, filepath.Base(testFile), tr)
	if err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(tmp, b.Name, filepath.Base(testFile))
	_, err = os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}

	f, err := fs.Open(b, filepath.Base(testFile))
	if err != nil {
		t.Fatal(err)
	}
	s := sha1.New()

	_, err = io.Copy(s, f)
	if err != nil {
		t.Fatal(err)
	}

	expected := hex.EncodeToString(h.Sum(nil))
	got := hex.EncodeToString(s.Sum(nil))

	if got != expected {
		t.Errorf("hash miss-match: %s != %s", got, expected)
	}
}

func TestFileHash(t *testing.T) {
	testFile := "./fixture/test.zip"
	h := sha1.New()

	r, err := os.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = io.Copy(h, r)
	if err != nil {
		t.Fatal(err)
	}
	expected := hex.EncodeToString(h.Sum(nil))

	f := newFile(r)
	b, err := f.Hash()
	if err != nil {
		t.Fatal(err)
	}

	got := hex.EncodeToString(b)

	if got != expected {
		t.Errorf("hash miss-match: %s != %s", got, expected)
	}

	// Hash should be cached
	b, err = f.Hash()
	if err != nil {
		t.Fatal(err)
	}

	got = hex.EncodeToString(b)

	if got != expected {
		t.Errorf("hash miss-match: %s != %s", got, expected)
	}
}
