package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gorilla/pat"
	"github.com/soundcloud/ent/lib"
)

const fixtureZip = "./fixture/test.zip"

func TestHandleCreate(t *testing.T) {
	var (
		fs = ent.NewMemoryFS()
		b  = ent.NewBucket("ent", ent.Owner{})
	)

	r := pat.New()
	r.Post(ent.RouteFile, handleCreate(ent.NewMemoryProvider(b), fs))

	ts := httptest.NewServer(r)
	defer ts.Close()

	testHash := sha1.New()
	testFile, err := os.Open(fixtureZip)
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
	fs := ent.NewMemoryFS()

	r := pat.New()
	r.Post(ent.RouteFile, handleCreate(ent.NewMemoryProvider(), fs))

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
		b   = ent.NewBucket("handle-delete", ent.Owner{})
		fs  = ent.NewMemoryFS()
		r   = pat.New()
		key = filepath.Base(fixtureZip)
	)

	r.Delete(ent.RouteFile, handleDelete(ent.NewMemoryProvider(b), fs))

	ts := httptest.NewServer(r)
	defer ts.Close()

	f, err := os.Open(fixtureZip)
	if err != nil {
		t.Fatal(err)
	}

	file, err := fs.Create(b, key, f)
	if err != nil {
		t.Fatal(err)
	}

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

	if have, want := res.StatusCode, http.StatusOK; have != want {
		t.Errorf("have %d, want %d", have, want)
	}

	resp := ent.ResponseDeleted{}

	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		t.Fatal(err)
	}

	if have, want := resp.File.Key, key; have != want {
		t.Errorf("want %v, got %v", have, want)
	}

	if have, want := resp.File.LastModified, file.LastModified(); !have.Equal(want) {
		t.Errorf("want %v, got %v", have, want)
	}

	if have, want := resp.File.Bucket, b; !reflect.DeepEqual(have, want) {
		t.Errorf("want %v, got %v", have, want)
	}

	if _, have := fs.Open(b, key); !ent.IsFileNotFound(have) {
		t.Errorf("have %s, want %s", have, ent.ErrFileNotFound)
	}
}

func TestHandleGetLastModifiedReturnsNotModified(t *testing.T) {
	var (
		fs = ent.NewMemoryFS()
		b  = ent.NewBucket("handle-get", ent.Owner{})
		k  = "foo.zip"
		r  = pat.New()
	)

	r.Get(ent.RouteFile, handleGet(ent.NewMemoryProvider(b), fs))

	ts := httptest.NewServer(r)
	defer ts.Close()

	f, err := os.Open(fixtureZip)
	if err != nil {
		t.Fatal(err)
	}

	file, err := fs.Create(b, k, f)
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s/%s", ts.URL, b.Name, k), nil)
	req.Header.Add("If-Modified-Since", file.LastModified().UTC().Format(http.TimeFormat))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if have, want := res.StatusCode, http.StatusNotModified; have != want {
		t.Errorf("have %d, want %d", have, want)
	}
}

func TestHandleGet(t *testing.T) {
	var (
		fs = ent.NewMemoryFS()
		b  = ent.NewBucket("handle-get", ent.Owner{})
		k  = "foo.zip"
		r  = pat.New()
	)

	r.Get(ent.RouteFile, handleGet(ent.NewMemoryProvider(b), fs))

	ts := httptest.NewServer(r)
	defer ts.Close()

	f, err := os.Open(fixtureZip)
	if err != nil {
		t.Fatal(err)
	}

	file, err := fs.Create(b, k, f)
	if err != nil {
		t.Fatal(err)
	}

	ep := fmt.Sprintf("%s/%s/%s", ts.URL, b.Name, k)
	res, err := http.Get(ep)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if have, want := res.StatusCode, http.StatusOK; have != want {
		t.Errorf("have %d, want %d", have, want)
	}

	hash := sha1.New()

	_, err = io.Copy(hash, res.Body)
	if err != nil {
		t.Fatal(err)
	}

	test, err := file.Hash()
	if err != nil {
		t.Fatal(err)
	}

	var (
		have = hex.EncodeToString(hash.Sum(nil))
		want = hex.EncodeToString(test)
	)

	if have != want {
		t.Errorf("have %v, want %v", have, want)
	}
}

func TestHandleBucketList(t *testing.T) {
	var (
		bs = createBuckets([]string{"peer", "nxt", "master"})
		r  = pat.New()
	)

	r.Get("/", handleBucketList(ent.NewMemoryProvider(bs...)))

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
	var (
		name = "master"
		bs   = createBuckets([]string{name})
		fs   = ent.NewMemoryFS()
		p    = "list/files"
		r    = pat.New()
	)

	r.Get(ent.RouteBucket, handleFileList(ent.NewMemoryProvider(bs...), fs))

	ts := httptest.NewServer(r)
	defer ts.Close()

	f, err := os.Open(fixtureZip)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		_, err := fs.Create(bs[0], fmt.Sprintf("%s/%d", p, i), f)
		if err != nil {
			t.Fatal(err)
		}
	}

	inputs := []struct {
		count int
		vs    url.Values
	}{
		{
			count: 1,
			vs:    url.Values{"limit": []string{"1"}, "sort": []string{"+lastModified"}},
		},
		{
			count: 10,
			vs:    url.Values{"prefix": []string{p}},
		},
		{
			count: 4,
			vs:    url.Values{"limit": []string{"4"}, "prefix": []string{p}, "sort": []string{"-key"}},
		},
	}

	for _, input := range inputs {
		filesURL := fmt.Sprintf("%s/%s?%s", ts.URL, name, input.vs.Encode())

		files, err := getFiles(filesURL)
		if err != nil {
			t.Error(err)
		}

		if have, want := len(files), input.count; have != want {
			t.Logf("%#v", files)
			t.Errorf("have %d, want %d", have, want)
		}
	}
}

func TestHandleFileListInvalidParams(t *testing.T) {
	var (
		name = "master"
		bs   = createBuckets([]string{name})
		fs   = ent.NewMemoryFS()
		p    = "invalid/files"
		r    = pat.New()
	)

	r.Get(ent.RouteBucket, handleFileList(ent.NewMemoryProvider(bs...), fs))

	ts := httptest.NewServer(r)
	defer ts.Close()

	f, err := os.Open(fixtureZip)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		_, err := fs.Create(bs[0], fmt.Sprintf("%s/%d", p, i), f)
		if err != nil {
			t.Fatal(err)
		}
	}

	inputs := []url.Values{
		url.Values{"limit": []string{"-1"}},
		url.Values{"limit": []string{"asd"}},
		url.Values{"limit": []string{"4"}, "prefix": []string{p}, "sort": []string{"key"}},
		url.Values{"limit": []string{"4"}, "prefix": []string{p}, "sort": []string{"-key1"}},
		url.Values{"limit": []string{"12"}, "prefix": []string{p}, "sort": []string{"-1k2ey"}},
		url.Values{"sort": []string{"+LastModified"}},
	}

	for _, input := range inputs {
		filesURL := fmt.Sprintf("%s/%s?%s", ts.URL, name, input.Encode())

		res, err := http.Get(filesURL)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()

		if have, want := res.StatusCode, http.StatusBadRequest; have != want {
			t.Errorf("have %d, want %d", have, want)
		}
	}
}

func TestAddCORSHeaders(t *testing.T) {
	ts := httptest.NewServer(addCORSHeaders(http.HandlerFunc(http.NotFound)))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	for key, want := range map[string]string{
		"Access-Control-Allow-Headers": "Accept, Authorization, Content-Type, Origin",
		"Access-Control-Allow-Methods": "GET, POST, DELETE",
		"Access-Control-Allow-Origin":  "*",
	} {
		if have := res.Header.Get(key); have != want {
			t.Errorf("want %s, have %s", want, have)
		}
	}
}

func getFiles(url string) ([]ent.ResponseFile, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unexpected status: %d\n%s", res.StatusCode, string(body))
	}

	resp := ent.ResponseFileList{}

	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return nil, err
	}

	return resp.Files, nil
}

func toMap(bucketsList []*ent.Bucket) map[ent.Bucket]int {
	bucketMap := map[ent.Bucket]int{}
	for _, bucket := range bucketsList {
		bucketMap[*bucket]++
	}
	return bucketMap
}

func createBuckets(names []string) []*ent.Bucket {
	bs := []*ent.Bucket{}

	for _, name := range names {
		addr, err := mail.ParseAddress(fmt.Sprintf("%s <%s@ent.io>", name, name))
		if err != nil {
			panic(err)
		}

		bs = append(bs, ent.NewBucket(name, ent.Owner{Email: *addr}))
	}

	return bs
}
