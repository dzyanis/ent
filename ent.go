// Copyright (c) 2014, SoundCloud Ltd.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/ent

package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/pat"
)

const (
	fileRoute = `/{bucket}/{key:[a-zA-Z0-9\-_\.~\+\/]+}`
)

func main() {
	var (
		fsRoot      = flag.String("fs.root", "/tmp", "FileSystem root directory")
		httpAddress = flag.String("http.address", ":5555", "HTTP listen address")
		providerDir = flag.String("provider.dir", "/tmp", "Provider directory with bucket policies")
	)
	flag.Parse()

	p, err := NewDiskProvider(*providerDir)
	if err != nil {
		log.Fatal(err)
	}

	fs := NewDiskFS(*fsRoot)
	r := pat.New()
	r.Get(fileRoute, handleGet(p, fs))
	r.Post(fileRoute, handleCreate(p, fs))
	r.Get("/", handleBucketList(p))

	log.Fatal(http.ListenAndServe(*httpAddress, http.Handler(r)))
}

func handleCreate(p Provider, fs FileSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			began  = time.Now()
			bucket = r.URL.Query().Get(":bucket")
			key    = r.URL.Query().Get(":key")
		)
		defer r.Body.Close()

		b, err := p.Get(bucket)
		if err != nil {
			respondError(w, r.Method, r.URL.String(), err)
			return
		}

		f, err := fs.Create(b, key, r.Body)
		if err != nil {
			respondError(w, r.Method, r.URL.String(), err)
			return
		}
		h, err := f.Hash()
		if err != nil {
			respondError(w, r.Method, r.URL.String(), err)
			return
		}

		respondCreated(w, b, key, h, time.Since(began))
	}
}

func handleGet(p Provider, fs FileSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			bucket = r.URL.Query().Get(":bucket")
			key    = r.URL.Query().Get(":key")
		)

		b, err := p.Get(bucket)
		if err != nil {
			respondError(w, r.Method, r.URL.String(), err)
			return
		}

		f, err := fs.Open(b, key)
		if err != nil {
			respondError(w, r.Method, r.URL.String(), err)
			return
		}

		http.ServeContent(w, r, key, time.Now(), f)
	}
}

func handleBucketList(p Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		began := time.Now()

		bs, err := p.List()
		if err != nil {
			respondError(w, r.Method, r.URL.String(), err)
			return
		}
		respondBucketList(w, bs, time.Since(began))
	}
}

// ResponseCreated is used as the intermediate type to craft a response for
// a successful file upload.
type ResponseCreated struct {
	Duration time.Duration `json:"duration"`
	File     ResponseFile  `json:"file"`
}

func respondCreated(
	w http.ResponseWriter,
	b *Bucket,
	k string,
	h []byte,
	d time.Duration,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(ResponseCreated{
		Duration: d,
		File: ResponseFile{
			Bucket: b,
			Key:    k,
			SHA1:   h,
		},
	})
}

// ResponseBucketList is used as the intermediate type to craft a response for
// the retrieval of all buckets.
type ResponseBucketList struct {
	Count    int           `json:"count"`
	Duration time.Duration `json:"duration"`
	Buckets  []*Bucket     `json:"buckets"`
}

func respondBucketList(w http.ResponseWriter, bs []*Bucket, d time.Duration) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(ResponseBucketList{
		Count:    len(bs),
		Duration: d,
		Buckets:  bs,
	})
}

// ResponseError is used as the intermediate type to craft a response for any
// kind of error condition in the http path. This includes common error cases
// like an entity could not be found.
type ResponseError struct {
	Code        int    `json:"code"`
	Error       string `json:"error"`
	Description string `json:"description"`
}

func respondError(w http.ResponseWriter, method, url string, err error) {
	code := http.StatusInternalServerError

	switch err {
	case ErrBucketNotFound, ErrFileNotFound:
		code = http.StatusNotFound
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ResponseError{
		Code:        code,
		Error:       err.Error(),
		Description: http.StatusText(code),
	})
}

// ResponseFile is used as the intermediate type to return metadata of a File.
type ResponseFile struct {
	Bucket *Bucket
	Key    string
	SHA1   []byte
}

// MarshalJSON returns a ResponseFile JSON encoding with conversion of the
// files SHA1 to hex.
func (r ResponseFile) MarshalJSON() ([]byte, error) {
	return json.Marshal(responseFileWrapper{
		Bucket: r.Bucket,
		Key:    r.Key,
		SHA1:   hex.EncodeToString(r.SHA1),
	})
}

// UnmarshalJSON marshals data into *r with conversion of the hex
// representation of SHA1 into a []byte.
func (r *ResponseFile) UnmarshalJSON(d []byte) error {
	var w responseFileWrapper

	err := json.Unmarshal(d, &w)
	if err != nil {
		return err
	}
	h, err := hex.DecodeString(w.SHA1)
	if err != nil {
		return err
	}

	r.Bucket = w.Bucket
	r.Key = w.Key
	r.SHA1 = h

	return nil
}

type responseFileWrapper struct {
	Bucket *Bucket `json:"bucket"`
	Key    string  `json:"key"`
	SHA1   string  `json:"sha1"`
}
