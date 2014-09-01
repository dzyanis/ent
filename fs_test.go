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
	"strings"
	"testing"
	"time"

	"github.com/soundcloud/ent/lib"
)

func TestDiskFS(t *testing.T) {
	tmp, err := ioutil.TempDir("", "ent-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	b := ent.NewBucket("create", ent.Owner{})
	fs := newDiskFS(tmp)

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

func TestDiskFSList(t *testing.T) {
	var (
		tempFiles = []string{
			"temp1",
			"temp2",
			"temp3",
			"temp4",
			"temp5",
			"prefix1",
			"prefix2",
			"prefix3",
			"prefix4",
			"one",
		}

		blobsCount = uint64(len(tempFiles)) + 1

		listTestEntries = []struct {
			prefix        string
			limit         uint64
			expectedCount int
		}{
			{"test", 1, 1},
			{"temp", 1, 1},
			{"temp", 13, 5},
			{"unexistedPrefix", 1000, 0},
			{"", blobsCount, int(blobsCount)},
			{"", blobsCount + 1, int(blobsCount)},
			{"", blobsCount - 1, int(blobsCount - 1)},
			{"one", 1, 1},
			{"one", 20, 1},
			{"o", 1, 1},
			{"o", 20, 1},
		}
	)

	tmp, err := ioutil.TempDir("", "ent-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	bucketDir, err := ioutil.TempDir(tmp, "")
	if err != nil {
		t.Fatal(err)
	}

	for i, name := range tempFiles {
		tmp, err := ioutil.TempFile(bucketDir, name)
		if err != nil {
			t.Fatalf("Could not setup env %s", err)
		}
		newTime := time.Now().Add(time.Second * time.Duration(i*10))
		os.Chtimes(tmp.Name(), newTime, newTime)
	}

	tmpDir, err := ioutil.TempDir(bucketDir, "test")
	if err != nil {
		t.Fatal(err)
	}

	tmpFile, err := ioutil.TempFile(tmpDir, "prefix")
	if err != nil {
		t.Fatal(err)
	}
	newTime := time.Now().Add(time.Second * time.Duration((len(tempFiles)+1)*10))
	os.Chtimes(tmpFile.Name(), newTime, newTime)

	bucketName := bucketDir[len(tmp)+1:]
	b := ent.NewBucket(bucketName, ent.Owner{})
	fs := newDiskFS(tmp)

	for _, input := range listTestEntries {
		strategy, err := createSortStrategy("")
		if err != nil {
			t.Fatal(err)
		}
		all, err := fs.List(b, input.prefix, input.limit, strategy)
		if err != nil {
			t.Fatal(err)
		}

		if len(all) != input.expectedCount {
			t.Errorf("Wrong number of files actual %d != expected %d for prefix %s and limit %d",
				len(all),
				input.expectedCount,
				input.prefix,
				input.limit,
			)
		}

		for _, file := range all {
			if !strings.HasPrefix(file.Key(), input.prefix) {
				t.Errorf("File %q should start with %q", file.Key(), input.prefix)
			}
		}
	}

	strategy, err := createSortStrategy("+key")
	if err != nil {
		t.Fatal(err)
	}

	all, err := fs.List(b, "", defaultLimit, strategy)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(tempFiles)+1 {
		t.Fatalf("Wrong number of files actual %d !=  expected %d", len(all), len(tempFiles))
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Key() > all[i].Key() {
			t.Errorf("Not sorted correctly %s > %s ", all[i-1].Key(), all[i].Key())
			break
		}
	}

	strategy, err = createSortStrategy("-key")
	if err != nil {
		t.Fatal(err)
	}

	all, err = fs.List(b, "", defaultLimit, strategy)
	if err != nil {
		t.Fatal(err)
	}

	if len(all) != len(tempFiles)+1 {
		t.Fatalf("Wrong number of files actual %d !=  expected %d", len(all), len(tempFiles))
	}

	for i := 1; i < len(all); i++ {
		if all[i-1].Key() < all[i].Key() {
			t.Errorf("Not sorted correctly %s < %s ", all[i-1].Key(), all[i].Key())
			break
		}
	}

	strategy, err = createSortStrategy("+lastModified")
	if err != nil {
		t.Fatal(err)
	}

	all, err = fs.List(b, "", defaultLimit, strategy)
	if err != nil {
		t.Fatal(err)
	}

	if len(all) != len(tempFiles)+1 {
		t.Fatalf("Wrong number of files actual %d !=  expected %d", len(all), len(tempFiles))
	}

	for i := 1; i < len(all); i++ {
		if !all[i-1].LastModified().Before(all[i].LastModified()) {
			t.Errorf("Not sorted correctly %s after %s ", all[i-1].LastModified(), all[i].LastModified())
			break
		}
	}

	strategy, err = createSortStrategy("-lastModified")
	if err != nil {
		t.Fatal(err)
	}

	all, err = fs.List(b, "", defaultLimit, strategy)
	if err != nil {
		t.Fatal(err)
	}

	if len(all) != len(tempFiles)+1 {
		t.Fatalf("Wrong number of files actual %d !=  expected %d", len(all), len(tempFiles))
	}

	for i := 1; i < len(all); i++ {
		if !all[i-1].LastModified().After(all[i].LastModified()) {
			t.Errorf("Not sorted correctly %s before %s ", all[i-1].LastModified(), all[i].LastModified())
		}
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

	f := newFile(r, "key")
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
