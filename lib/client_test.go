package ent

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/pat"
)

func TestClientCreate(t *testing.T) {
	var (
		body   = "file content goes here"
		bucket = "create"
		key    = "test.zip"
		r      = pat.New()
		start  = time.Now()
	)

	r.Post(RouteFile, func(w http.ResponseWriter, r *http.Request) {
		if have, want := r.URL.Query().Get(KeyBucket), bucket; have != want {
			t.Fatalf("have %s, want %s", have, want)
		}
		if have, want := r.URL.Query().Get(KeyBlob), key; have != want {
			t.Fatalf("have %s, want %s", have, want)
		}

		defer r.Body.Close()

		raw, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		if have, want := string(raw), body; have != want {
			t.Fatalf("have %s, want %s", have, want)
		}

		respondJSON(w, http.StatusCreated, ResponseCreated{
			Duration: time.Since(start),
			File: ResponseFile{
				Key:          r.URL.Query().Get(KeyBlob),
				Bucket:       NewBucket(r.URL.Query().Get(KeyBucket), Owner{}),
				LastModified: start,
			},
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	client := New(ts.URL, nil)

	file, err := client.Create(bucket, key, bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}

	if have, want := file.Key, key; have != want {
		t.Errorf("have %s, want %s", have, want)
	}
}

func TestClientCreateInvalid(t *testing.T) {
	client := New("lolcathost.org", nil)

	_, err := client.Create("", "blob", nil)
	if have, want := err, ErrEmptyBucket; !IsEmptyBucket(err) {
		t.Errorf("have %v, want %v", have, want)
	}

	_, err = client.Create("bucket", "", nil)
	if have, want := err, ErrEmptyKey; !IsEmptyKey(err) {
		t.Errorf("have %v, want %v", have, want)
	}

	_, err = client.Create("bucket", "blob", nil)
	if have, want := err, ErrEmptySource; !IsEmptySource(err) {
		t.Errorf("have %v, want %v", have, want)
	}
}

func TestClientGet(t *testing.T) {
	var (
		body   = "content is here"
		bucket = "get"
		key    = "content.log"
		r      = pat.New()
	)

	r.Get(RouteFile, func(w http.ResponseWriter, r *http.Request) {
		if have, want := r.URL.Query().Get(KeyBucket), bucket; have != want {
			t.Fatalf("have %s, want %s", have, want)
		}
		if have, want := r.URL.Query().Get(KeyBlob), key; have != want {
			t.Fatalf("have %s, want %s", have, want)
		}

		defer r.Body.Close()

		http.ServeContent(w, r, key, time.Now(), bytes.NewReader([]byte(body)))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	client := New(ts.URL, nil)

	file, err := client.Get(bucket, key)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	raw, err := ioutil.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	if have, want := string(raw), body; have != want {
		t.Errorf("have %v, want %v", have, want)
	}
}

func TestClientListFiles(t *testing.T) {
	var (
		bucket = "files"
		limit  = 5
		prefix = "list"
		r      = pat.New()
	)

	r.Get(RouteBucket, func(w http.ResponseWriter, r *http.Request) {
		if have, want := r.URL.Query().Get(KeyBucket), bucket; have != want {
			t.Fatalf("have %s, want %s", have, want)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}

		var (
			b          = NewBucket(bucket, Owner{})
			limitValue = r.Form.Get("limit")
			list       = ResponseFileList{
				Bucket:   b,
				Count:    5,
				Duration: time.Millisecond,
				Files:    []ResponseFile{},
			}
		)

		for i := 0; i < 10; i++ {
			list.Files = append(list.Files, ResponseFile{
				Key:          strconv.Itoa(i),
				LastModified: time.Now(),
				Bucket:       b,
			})
		}

		if limitValue != "" {
			limit, err := strconv.Atoi(limitValue)
			if err != nil {
				t.Fatal(err)
			}

			if limit < len(list.Files) {
				list.Files = list.Files[:limit]
			}
		}

		respondJSON(w, http.StatusOK, list)
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	client := New(ts.URL, nil)

	files, err := client.List(bucket, nil)
	if err != nil {
		t.Fatal(err)
	}

	if have, want := len(files), 10; have != want {
		t.Errorf("have %v, want %v", have, want)
	}

	files, err = client.List(bucket, &ListOptions{
		Limit:  uint64(limit),
		Prefix: prefix,
		Sort:   ByKeyStrategy(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	if have, want := len(files), 5; have != want {
		t.Errorf("have %d, want %d", have, want)
	}
}

func TestClientListFilesInvalid(t *testing.T) {
	client := New("lolcathost.org", nil)

	_, err := client.List("", nil)
	if have, want := err, ErrEmptyBucket; !IsEmptyBucket(err) {
		t.Errorf("have %v, want %v", have, want)
	}
}

func TestRequestError(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusBadRequest, &ResponseError{
				Code:  http.StatusBadRequest,
				Error: http.StatusText(http.StatusBadRequest),
			})
		}),
	)
	defer ts.Close()

	_, err := New(ts.URL, nil).request("GET", "/", nil, &struct{}{})
	if have, want := err, ErrClient; !IsClient(err) {
		t.Errorf("have %v, want %v", have, want)
	}
}

func respondJSON(w http.ResponseWriter, code int, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	err := json.NewEncoder(w).Encode(obj)
	if err != nil {
		panic(err)
	}
}
