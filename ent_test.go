// Copyright (c) 2014, SoundCloud Ltd.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/ent

package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gorilla/pat"
)

func TestHandleCreate(t *testing.T) {
	fs := newMockFileSystem()
	b := NewBucket("ent", Owner{})

	r := pat.New()
	r.Post(fileRoute, handleCreate(newMockProvider(b), fs))

	ts := httptest.NewServer(r)
	defer ts.Close()

	testHash := sha1.New()
	testFile, err := os.Open("./fixture/test.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer testFile.Close()

	var (
		key = "nested/structure/with.file"
		ep  = fmt.Sprintf("%s/%s/%s", ts.URL, b.Name, key)
		tr  = io.TeeReader(testFile, testHash)
	)

	res, err := http.Post(ep, "text/plain", tr)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	resp := ResponseCreated{}

	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusCreated {
		t.Errorf("HTTP %d", res.StatusCode)
	}

	if resp.File.Key != key {
		t.Errorf("keys differ: %s != %s", resp.File.Key, key)
	}

	if !bytes.Equal(resp.File.SHA1, testHash.Sum(nil)) {
		t.Errorf(
			"checksums differ: %s != %s",
			hex.EncodeToString(resp.File.SHA1),
			hex.EncodeToString(testHash.Sum(nil)),
		)
	}

	if *resp.File.Bucket != *b {
		t.Errorf("buckets differ: %s != %s", resp.File.Bucket, b)
	}
}

func TestHandleCreateInvalidBucket(t *testing.T) {
	fs := newMockFileSystem()
	r := pat.New()
	r.Post(fileRoute, handleCreate(newMockProvider(), fs))
	ts := httptest.NewServer(r)
	defer ts.Close()

	ep := fmt.Sprintf("%s/%s/%s", ts.URL, "fake-bucket", "cat.zip")
	res, err := http.Post(ep, "text/plain", bytes.NewReader([]byte("fake file")))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	resp := ResponseError{}

	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("HTTP %d", res.StatusCode)
	}
}

func TestHandleGet(t *testing.T) {
	fs := newMockFileSystem()

	b := NewBucket("ent", Owner{})

	r := pat.New()
	r.Get(fileRoute, handleGet(newMockProvider(b), fs))
	ts := httptest.NewServer(r)
	defer ts.Close()

	testHash := sha1.New()
	raw, err := ioutil.ReadFile("./fixture/test.zip")
	if err != nil {
		t.Fatal(err)
	}

	_, err = testHash.Write(raw)
	if err != nil {
		t.Fatal(err)
	}

	f := newMockFile(raw)
	fs.files["ent/foo.zip"] = f

	ep := fmt.Sprintf("%s/%s/%s", ts.URL, b.Name, "foo.zip")
	res, err := http.Get(ep)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("HTTP %d", res.StatusCode)
	}

	h := sha1.New()
	_, err = io.Copy(h, res.Body)
	if err != nil {
		t.Fatal(err)
	}

	expected := hex.EncodeToString(testHash.Sum(nil))
	got := hex.EncodeToString(h.Sum(nil))

	if got != expected {
		t.Errorf("checksum missmatch %#v != %#v", got, expected)
	}
}

func TestHandleBucketList(t *testing.T) {
	names := []string{"peer", "nxt", "master"}
	bs := []*Bucket{}

	for _, name := range names {
		addr, err := mail.ParseAddress(fmt.Sprintf("%s <%s@ent.io>", name, name))
		if err != nil {
			t.Fatal(err)
		}
		b := NewBucket(name, Owner{*addr})
		bs = append(bs, b)
	}

	r := pat.New()
	r.Get("/", handleBucketList(newMockProvider(bs...)))
	ts := httptest.NewServer(r)
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	resp := ResponseBucketList{}
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("HTTP %d", res.StatusCode)
	}

	if resp.Count != len(bs) {
		t.Errorf("not enough buckets returned: %d != %d", resp.Count, len(bs))
	}

	if !reflect.DeepEqual(resp.Buckets, bs) {
		t.Errorf("wrong answer")
	}
}

type mockFile struct {
	buffer *bytes.Buffer
	data   []byte
	hash   hash.Hash
	reader *bytes.Reader
	writer *bufio.Writer
}

func newMockFile(d []byte) *mockFile {
	if d == nil {
		d = []byte{}
	}

	f := &mockFile{
		data: d,
		hash: sha1.New(),
	}
	f.buffer = bytes.NewBuffer(f.data)
	f.reader = bytes.NewReader(f.data)
	f.writer = bufio.NewWriter(f.buffer)

	return f
}

func (f *mockFile) Close() error {
	return nil
}

func (f *mockFile) Hash() ([]byte, error) {
	return f.hash.Sum(nil), nil
}

func (f *mockFile) Read(p []byte) (int, error) {
	return f.reader.Read(p)
}

func (f *mockFile) Seek(offset int64, whence int) (int64, error) {
	return f.reader.Seek(offset, whence)
}

func (f *mockFile) Write(p []byte) (int, error) {
	n, err := f.hash.Write(p)
	if err != nil {
		return n, err
	}

	return f.writer.Write(p)
}

type mockFileSystem struct {
	files map[string]File
}

func newMockFileSystem() *mockFileSystem {
	return &mockFileSystem{
		files: map[string]File{},
	}
}

func (fs *mockFileSystem) Create(bucket *Bucket, key string, src io.Reader) (File, error) {
	f := newMockFile(nil)
	_, err := io.Copy(f, src)
	if err != nil {
		return nil, err
	}

	fs.files[fmt.Sprintf("%s/%s", bucket.Name, key)] = f

	return f, nil
}

func (fs *mockFileSystem) Open(bucket *Bucket, key string) (File, error) {
	f, ok := fs.files[filepath.Join(bucket.Name, key)]
	if !ok {
		return nil, ErrFileNotFound
	}
	return f, nil
}

type mockProvider struct {
	buckets map[string]*Bucket
}

func newMockProvider(buckets ...*Bucket) Provider {
	p := &mockProvider{
		buckets: map[string]*Bucket{},
	}

	for _, b := range buckets {
		p.buckets[b.Name] = b
	}

	return p
}

func (p *mockProvider) Get(name string) (*Bucket, error) {
	b, ok := p.buckets[name]
	if !ok {
		return nil, ErrBucketNotFound
	}
	return b, nil
}

func (p *mockProvider) Init() error {
	return nil
}

func (p *mockProvider) List() ([]*Bucket, error) {
	bs := []*Bucket{}
	for _, b := range p.buckets {
		bs = append(bs, b)
	}
	return bs, nil
}
