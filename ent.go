// Copyright (c) 2014, SoundCloud Ltd.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/ent

package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	logpkg "log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/pat"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/streadway/handy/report"
)

const (
	fileRoute = `/{bucket}/{key:[a-zA-Z0-9\-_\.~\+\/]+}`
)

var (
	Program = "ent"
	Commit  = "0000000"
	Version = "0.0.0"

	requestBytes     = prometheus.NewCounter()
	requestDuration  = prometheus.NewCounter()
	requestDurations = prometheus.NewDefaultHistogram()
	requestTotal     = prometheus.NewCounter()
	responseBytes    = prometheus.NewCounter()

	log = logpkg.New(os.Stdout, "", logpkg.LstdFlags|logpkg.Lmicroseconds)
)

func main() {
	var (
		fsRoot      = flag.String("fs.root", "/tmp", "FileSystem root directory")
		httpAddress = flag.String("http.addr", ":5555", "HTTP listen address")
		providerDir = flag.String("provider.dir", "/tmp", "Provider directory with bucket policies")
	)
	flag.Parse()

	prometheus.Register("ent_requests_total", "Total number of requests made", prometheus.NilLabels, requestTotal)
	prometheus.Register("ent_requests_duration_nanoseconds_total", "Total amount of time ent has spent to answer requests in nanoseconds", prometheus.NilLabels, requestDuration)
	prometheus.Register("ent_requests_duration_nanoseconds", "Amounts of time ent has spent answering requests in nanoseconds", prometheus.NilLabels, requestDurations)
	prometheus.Register("ent_request_bytes_total", "Total volume of request payloads emitted in bytes", prometheus.NilLabels, requestBytes)
	prometheus.Register("ent_response_bytes_total", "Total volume of response payloads emitted in bytes", prometheus.NilLabels, responseBytes)

	var (
		fs = NewDiskFS(*fsRoot)
		r  = pat.New()
	)

	p, err := NewDiskProvider(*providerDir)
	if err != nil {
		log.Fatal(err)
	}

	r.Handle("/metrics", prometheus.DefaultRegistry.Handler())
	r.Add(
		"GET",
		fileRoute,
		report.JSON(
			os.Stdout,
			metrics(
				"handleGet",
				handleGet(p, fs),
			),
		),
	)
	r.Add(
		"POST",
		fileRoute,
		report.JSON(
			os.Stdout,
			metrics(
				"handleCreate",
				handleCreate(p, fs),
			),
		),
	)
	r.Add(
		"GET",
		"/",
		report.JSON(
			os.Stdout,
			metrics(
				"handleBucketList",
				handleBucketList(p),
			),
		),
	)

	log.Printf("listening on %s", *httpAddress)
	log.Fatal(http.ListenAndServe(*httpAddress, http.Handler(r)))
}

func handleCreate(p Provider, fs FileSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			start  = time.Now()
			bucket = r.URL.Query().Get(":bucket")
			key    = r.URL.Query().Get(":key")
		)
		defer r.Body.Close()

		b, err := p.Get(bucket)
		if err != nil {
			respondError(w, r, err)
			return
		}

		f, err := fs.Create(b, key, r.Body)
		if err != nil {
			respondError(w, r, err)
			return
		}
		h, err := f.Hash()
		if err != nil {
			respondError(w, r, err)
			return
		}

		respondJSON(w, http.StatusCreated, ResponseCreated{
			Duration: time.Since(start),
			File: ResponseFile{
				Bucket: b,
				Key:    key,
				SHA1:   h,
			},
		})
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
			respondError(w, r, err)
			return
		}

		f, err := fs.Open(b, key)
		if err != nil {
			respondError(w, r, err)
			return
		}

		http.ServeContent(w, r, key, time.Now(), f)
	}
}

func handleBucketList(p Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			began = time.Now()
		)

		bs, err := p.List()
		if err != nil {
			respondError(w, r, err)
			return
		}

		respondJSON(w, http.StatusOK, ResponseBucketList{
			Count:    len(bs),
			Duration: time.Since(began),
			Buckets:  bs,
		})
	}
}

// ResponseCreated is used as the intermediate type to craft a response for
// a successful file upload.
type ResponseCreated struct {
	Duration time.Duration `json:"duration"`
	File     ResponseFile  `json:"file"`
}

// ResponseBucketList is used as the intermediate type to craft a response for
// the retrieval of all buckets.
type ResponseBucketList struct {
	Count    int           `json:"count"`
	Duration time.Duration `json:"duration"`
	Buckets  []*Bucket     `json:"buckets"`
}

// ResponseError is used as the intermediate type to craft a response for any
// kind of error condition in the http path. This includes common error cases
// like an entity could not be found.
type ResponseError struct {
	Code        int    `json:"code"`
	Error       string `json:"error"`
	Description string `json:"description"`
}

func respondError(w http.ResponseWriter, r *http.Request, err error) {
	code := http.StatusInternalServerError

	switch err {
	case ErrBucketNotFound, ErrFileNotFound:
		code = http.StatusNotFound
	}

	respondJSON(w, code, ResponseError{
		Code:        code,
		Error:       err.Error(),
		Description: http.StatusText(code),
	})
}

func respondJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
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

func metrics(op string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			start = time.Now()
			rd    = &readerDelegator{ReadCloser: r.Body}
			rc    = &responseRecorder{ResponseWriter: w}
		)

		r.Body = rd

		next.ServeHTTP(rc, r)

		d := time.Since(start)
		labels := map[string]string{
			"bucket":    r.URL.Query().Get(":bucket"),
			"method":    strings.ToLower(r.Method),
			"operation": op,
			"status":    strconv.Itoa(rc.status),
		}

		requestBytes.IncrementBy(labels, float64(rd.BytesRead))
		requestTotal.Increment(labels)
		requestDuration.IncrementBy(labels, float64(d))
		requestDurations.Add(labels, float64(d))
		responseBytes.IncrementBy(labels, float64(rc.size))
	})
}

type readerDelegator struct {
	io.ReadCloser
	BytesRead int
}

func (r *readerDelegator) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.BytesRead += n
	return n, err
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.size += n
	return n, err
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
