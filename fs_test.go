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

func TestDiskFSCreate(t *testing.T) {
	tmp, err := ioutil.TempDir("", "ent-diskfs-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	var (
		b        = ent.NewBucket("create", ent.Owner{})
		fs       = newDiskFS(tmp)
		h        = sha1.New()
		testFile = "./fixture/test.zip"
	)

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

	var (
		expected = hex.EncodeToString(h.Sum(nil))
		got      = hex.EncodeToString(s.Sum(nil))
	)

	if got != expected {
		t.Errorf("hash miss-match: %s != %s", got, expected)
	}
}

func TestDiskFSDelete(t *testing.T) {
	tmp, err := ioutil.TempDir("", "diskfs-delete")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	var (
		b    = ent.NewBucket("delete", ent.Owner{})
		fs   = newDiskFS(tmp)
		file = "./fixture/test.zip"
		key  = filepath.Base(file)
		dst  = filepath.Join(tmp, b.Name, key)
	)

	r, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	f, err := fs.Create(b, key, r)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}

	err = fs.Delete(b, f.Key())
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(dst)
	if !os.IsNotExist(err) {
		t.Errorf("want %v, got %v", os.ErrNotExist, err)
	}
}

func TestDiskFSDeleteFileNotFound(t *testing.T) {
	tmp, err := ioutil.TempDir("", "diskfs-delete-notfound")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	var (
		b  = ent.NewBucket("delete-notfound", ent.Owner{})
		fs = newDiskFS(tmp)
	)

	err = fs.Delete(b, "non-exisiting-file")

	if want, got := ent.ErrFileNotFound, err; want != got {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestDiskFSFileNotFound(t *testing.T) {
	tmp, err := ioutil.TempDir("", "ent-notfound-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	bucketDir, err := ioutil.TempDir(tmp, "bucket")
	if err != nil {
		t.Fatal(err)
	}

	dir, err := ioutil.TempDir(bucketDir, "dir")
	if err != nil {
		t.Fatal(err)
	}

	var (
		b  = ent.NewBucket(filepath.Base(bucketDir), ent.Owner{})
		fs = newDiskFS(tmp)
	)

	_, err = fs.Open(b, "non-existing.file")
	if !ent.IsFileNotFound(err) {
		t.Errorf("expected %s when opening missing file got %s", ent.ErrFileNotFound, err)
	}

	_, err = fs.Open(b, filepath.Base(dir))
	if !ent.IsFileNotFound(err) {
		t.Errorf("expected %s when opening missing file got %s", ent.ErrFileNotFound, err)
	}
}

func TestDiskFSList(t *testing.T) {
	var (
		tempFiles = []string{
			"one",
			"prefix1",
			"prefix2",
			"prefix3",
			"prefix4",
			"temp1",
			"temp2",
			"temp3",
			"temp4",
			"temp5",
			"test/depth",
		}

		blobsCount = uint64(len(tempFiles))

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

	for i, input := range tempFiles {
		var (
			parts   = strings.SplitN(input, "/", 2)
			newTime = time.Now().Add(time.Second * time.Duration(i*10))
			dir     = bucketDir
			name    = parts[0]

			err error
		)

		if len(parts) == 2 {
			dir, err = ioutil.TempDir(bucketDir, "test")
			if err != nil {
				t.Fatal(err)
			}
			name = parts[1]
		}

		tmp, err := ioutil.TempFile(dir, name)
		if err != nil {
			t.Fatalf("create temp file: %s", err)
		}

		err = os.Chtimes(tmp.Name(), newTime, newTime)
		if err != nil {
			t.Fatalf("change times: %s", err)
		}
	}

	var (
		b           = ent.NewBucket(filepath.Base(bucketDir), ent.Owner{})
		fs          = newDiskFS(tmp)
		emptyBucket = ent.NewBucket("notCreatedDir", ent.Owner{})
	)

	all, err := fs.List(emptyBucket, "", 12, ent.NoOpStrategy())
	if err != nil {
		t.Fatal(err)
	}

	if len(all) != 0 {
		t.Errorf(
			"wrong number of files listing empty bucket:  %d",
			len(all),
		)
	}

	for _, input := range listTestEntries {
		all, err := fs.List(b, input.prefix, input.limit, ent.NoOpStrategy())
		if err != nil {
			t.Fatal(err)
		}

		if len(all) != input.expectedCount {
			t.Errorf(
				"wrong number of files for %q(%d): %d != %d",
				input.prefix,
				input.limit,
				len(all),
				input.expectedCount,
			)
		}

		for _, file := range all {
			if !strings.HasPrefix(file.Key(), input.prefix) {
				t.Errorf("%q should start with %q", file.Key(), input.prefix)
			}
		}
	}

	strategy, err := createSortStrategy("+key")
	if err != nil {
		t.Fatal(err)
	}

	all, err = fs.List(b, "", ent.DefaultLimit, strategy)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(tempFiles) {
		t.Fatalf("wrong number of files: %d != %d", len(all), len(tempFiles))
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Key() > all[i].Key() {
			t.Errorf("not sorted correctly %s > %s ", all[i-1].Key(), all[i].Key())
			break
		}
	}

	strategy, err = createSortStrategy("-key")
	if err != nil {
		t.Fatal(err)
	}

	all, err = fs.List(b, "", ent.DefaultLimit, strategy)
	if err != nil {
		t.Fatal(err)
	}

	if len(all) != len(tempFiles) {
		t.Fatalf("wrong number of files: %d != %d", len(all), len(tempFiles))
	}

	for i := 1; i < len(all); i++ {
		if all[i-1].Key() < all[i].Key() {
			t.Errorf("not sorted correctly %s < %s ", all[i-1].Key(), all[i].Key())
			break
		}
	}

	strategy, err = createSortStrategy("+lastModified")
	if err != nil {
		t.Fatal(err)
	}

	all, err = fs.List(b, "", ent.DefaultLimit, strategy)
	if err != nil {
		t.Fatal(err)
	}

	if len(all) != len(tempFiles) {
		t.Fatalf("wrong number of files: %d != %d", len(all), len(tempFiles))
	}

	for i := 1; i < len(all); i++ {
		if !all[i-1].LastModified().Before(all[i].LastModified()) {
			t.Errorf("not sorted correctly %s after %s ", all[i-1].LastModified(), all[i].LastModified())
			break
		}
	}

	strategy, err = createSortStrategy("-lastModified")
	if err != nil {
		t.Fatal(err)
	}

	all, err = fs.List(b, "", ent.DefaultLimit, strategy)
	if err != nil {
		t.Fatal(err)
	}

	if len(all) != len(tempFiles) {
		t.Fatalf("wrong number of files actual %d !=  expected %d", len(all), len(tempFiles))
	}

	for i := 1; i < len(all); i++ {
		if !all[i-1].LastModified().After(all[i].LastModified()) {
			t.Errorf("not sorted correctly %s before %s ", all[i-1].LastModified(), all[i].LastModified())
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
