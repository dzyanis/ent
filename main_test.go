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
	"time"

	"github.com/gorilla/pat"
	"github.com/soundcloud/ent/lib"
)

func TestHandleCreate(t *testing.T) {
	fs := newMockFileSystem()
	b := ent.NewBucket("ent", ent.Owner{})

	r := pat.New()
	r.Post(routeFile, handleCreate(newMockProvider(b), fs))

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

	resp := ent.ResponseCreated{}

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
}

func TestHandleCreateInvalidBucket(t *testing.T) {
	fs := newMockFileSystem()
	r := pat.New()
	r.Post(routeFile, handleCreate(newMockProvider(), fs))
	ts := httptest.NewServer(r)
	defer ts.Close()

	ep := fmt.Sprintf("%s/%s/%s", ts.URL, "fake-bucket", "cat.zip")
	res, err := http.Post(ep, "text/plain", bytes.NewReader([]byte("fake file")))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	resp := ent.ResponseError{}
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("HTTP %d", res.StatusCode)
	}
}

func TestHandleDelete(t *testing.T) {
	var (
		b    = ent.NewBucket("handle-delete", ent.Owner{})
		fs   = newMockFileSystem()
		r    = pat.New()
		file = "./fixture/test.zip"
		key  = filepath.Base(file)
	)

	r.Delete(routeFile, handleDelete(newMockProvider(b), fs))

	ts := httptest.NewServer(r)
	defer ts.Close()

	raw, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	f := newMockFile(raw)
	fs.files[fmt.Sprintf("%s/%s", b.Name, key)] = f

	req, err := http.NewRequest(
		"DELETE",
		fmt.Sprintf("%s/%s/%s", ts.URL, b.Name, key),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	res, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if want, got := http.StatusOK, res.StatusCode; want != got {
		t.Errorf("want %d, got %d", want, got)
	}

	resp := ent.ResponseDeleted{}

	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := (ent.ResponseFile{
		Bucket:       b,
		Key:          key,
		LastModified: f.LastModified(),
	}), resp.File; !reflect.DeepEqual(want, got) {
		t.Errorf("want %v, got %v", want, got)
	}

	if _, have := fs.Open(b, key); !ent.IsFileNotFound(have) {
		t.Errorf("want %s, have %s", ent.ErrFileNotFound, have)
	}
}

func TestHandleGet(t *testing.T) {
	fs := newMockFileSystem()

	b := ent.NewBucket("ent", ent.Owner{})

	r := pat.New()
	r.Get(routeFile, handleGet(newMockProvider(b), fs))
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
	bs := createBuckets(names, t)

	r := pat.New()
	r.Get("/", handleBucketList(newMockProvider(bs...)))
	ts := httptest.NewServer(r)
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	resp := ent.ResponseBucketList{}
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

	if !reflect.DeepEqual(toMap(resp.Buckets), toMap(bs)) {
		t.Errorf("wrong answer")
	}
}

func TestHandleFileList(t *testing.T) {
	name := "master"
	bs := createBuckets([]string{name}, t)
	r := pat.New()
	r.Get(routeBucket, handleFileList(newMockProvider(bs...), newMockFileSystem()))
	ts := httptest.NewServer(r)
	defer ts.Close()

	getFiles(ts.URL+"/"+name+"?limit=1&sort=%2BlastModified", t, 0)
	getFiles(ts.URL+"/"+name+"?prefix=list%2Ffiles&limit=10&sort=%2BlastModified", t, 10)
	getFiles(ts.URL+"/"+name+"?prefix=list%2Ffiles", t, 10)
	listedFiles := getFiles(ts.URL+"/master?prefix=list%2Ffiles&limit=4&sort=-key", t, 4)

	for i, file := range listedFiles {
		expected := fmt.Sprintf("list/filesname%d", i)
		if file.Key != expected {
			t.Errorf("%q != %q", file.Key, expected)
		}
	}

	listedFiles = getFiles(ts.URL+"/master?prefix=list%2Ffiles&limit=4&sort=-key", t, 4)
	for i, file := range listedFiles {
		expected := fmt.Sprintf("list/filesname%d", i)
		if file.Key != expected {
			t.Fatalf("%q != %q", file.Key, expected)
		}
	}
}

func TestHandleInavalidParams(t *testing.T) {
	bs := createBuckets([]string{"master"}, t)
	r := pat.New()
	r.Get(routeBucket, handleFileList(newMockProvider(bs...), newMockFileSystem()))
	ts := httptest.NewServer(r)
	defer ts.Close()

	invalidRequests := []string{
		"/master?prefix=listfiles&limit=4&sort=key",
		"/master?prefix=listfiles&limit=4&sort=-key1",
		"/master?prefix=listfiles&limit=12&sort=-1k2ey",
		"/master?sort=%2BlastModifieddd",
		"/master?limit=-1",
		"/master?limit=asd",
	}
	for _, request := range invalidRequests {
		res, err := http.Get(ts.URL + request)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("Request %s, response code %d != expected code %d", request, res.StatusCode, http.StatusBadRequest)
		}
		res.Body.Close()
	}

	res, err := http.Get(ts.URL + "/invalid")
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("Passing invalid bucket, response code %d != expected code %d", res.StatusCode, http.StatusNotFound)
	}
	res.Body.Close()

}

type mockFile struct {
	buffer *bytes.Buffer
	data   []byte
	hash   hash.Hash
	reader *bytes.Reader
	writer *bufio.Writer
	time   time.Time
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
	f.time = time.Now()
	return f
}

func (f *mockFile) Close() error {
	return nil
}

func (f *mockFile) Key() string {
	return ""
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

func (f *mockFile) LastModified() time.Time {
	return f.time
}

type mockFileSystem struct {
	files map[string]ent.File
}

func newMockFileSystem() *mockFileSystem {
	return &mockFileSystem{
		files: map[string]ent.File{},
	}
}

func (fs *mockFileSystem) Create(bucket *ent.Bucket, key string, src io.Reader) (ent.File, error) {
	f := newMockFile(nil)
	_, err := io.Copy(f, src)
	if err != nil {
		return nil, err
	}

	fs.files[fmt.Sprintf("%s/%s", bucket.Name, key)] = f

	return f, nil
}

func (fs *mockFileSystem) Delete(bucket *ent.Bucket, key string) error {
	delete(fs.files, fmt.Sprintf("%s/%s", bucket.Name, key))

	return nil
}

func (fs *mockFileSystem) Open(bucket *ent.Bucket, key string) (ent.File, error) {
	f, ok := fs.files[filepath.Join(bucket.Name, key)]
	if !ok {
		return nil, ent.ErrFileNotFound
	}
	return f, nil
}

func (fs *mockFileSystem) List(bucket *ent.Bucket, prefix string, limit uint64, sort ent.SortStrategy) (ent.Files, error) {
	if prefix == "list/files" {
		f, _ := os.Open("fixture/test.zip")
		files := []ent.File{}
		for i := 0; i < 10; i++ {
			files = append(files, newFile(f, prefix+""+fmt.Sprintf("name%d", i)))
		}
		if uint64(len(files)) > limit {
			return files[:limit], nil
		}
		return files, nil
	}
	return []ent.File{}, nil
}

type mockProvider struct {
	buckets map[string]*ent.Bucket
}

func newMockProvider(buckets ...*ent.Bucket) ent.Provider {
	p := &mockProvider{
		buckets: map[string]*ent.Bucket{},
	}

	for _, b := range buckets {
		p.buckets[b.Name] = b
	}

	return p
}

func (p *mockProvider) Get(name string) (*ent.Bucket, error) {
	b, ok := p.buckets[name]
	if !ok {
		return nil, ent.ErrBucketNotFound
	}
	return b, nil
}

func (p *mockProvider) Init() error {
	return nil
}

func (p *mockProvider) List() ([]*ent.Bucket, error) {
	bs := []*ent.Bucket{}
	for _, b := range p.buckets {
		bs = append(bs, b)
	}
	return bs, nil
}

func getFiles(url string, t *testing.T, expectedCount int) []ent.ResponseFile {
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	response := ent.ResponseFileList{}
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}
	listedFiles := response.Files

	if res.StatusCode != http.StatusOK {
		t.Errorf("HTTP %d", res.StatusCode)
	}

	if len(listedFiles) != expectedCount {
		t.Errorf("not write count of files returned: %d != %d for request %s", len(listedFiles), expectedCount, url)
	}
	if response.Count != expectedCount {
		t.Errorf("not metainfo for count of files returned: %d != %d for request %s", response.Count, expectedCount, url)
	}
	return listedFiles
}

func toMap(bucketsList []*ent.Bucket) map[ent.Bucket]int {
	bucketMap := map[ent.Bucket]int{}
	for _, bucket := range bucketsList {
		bucketMap[*bucket]++
	}
	return bucketMap
}

func createBuckets(names []string, t *testing.T) []*ent.Bucket {
	bs := []*ent.Bucket{}
	for _, name := range names {
		addr, err := mail.ParseAddress(fmt.Sprintf("%s <%s@ent.io>", name, name))
		if err != nil {
			t.Fatal(err)
		}
		b := ent.NewBucket(name, ent.Owner{Email: *addr})
		bs = append(bs, b)
	}
	return bs
}
