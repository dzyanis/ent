// Copyright (c) 2014, SoundCloud Ltd.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/ent

package main

import (
	"encoding/json"
	"flag"
	"io"
	logpkg "log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/pat"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/soundcloud/ent/lib"
	"github.com/streadway/handy/report"
)

const (
	routeBucket = `/{bucket}`
	routeFile   = `/{bucket}/{key:[a-zA-Z0-9\-_\.~\+\/]+}`

	paramLimit  = "limit"
	paramPrefix = "prefix"
	paramSort   = "sort"

	orderKey          = "key"
	orderLastModified = "lastModified"
	orderAscending    = "+"
	orderDescending   = "-"

	defaultLimit uint64 = math.MaxUint64
)

var (
	Program = "ent"
	Commit  = "0000000"
	Version = "0.0.0"

	labelNames = []string{"bucket", "method", "operation", "status"}

	requestDurations = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: Program,
			Name:      "requests_duration_nanoseconds",
			Help:      "Amounts of time ent has spent answering requests in nanoseconds.",
		},
		labelNames,
	)
	// Note that the summary 'requestDurations' above will result in metrics
	// 'ent_requests_duration_nanoseconds_count' and
	// 'ent_requests_duration_nanoseconds_sum', counting the total number of
	// requests made and summing up the total amount of time ent has spent
	// to answer requests, respectively.
	requestBytes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Program,
			Name:      "request_bytes_total",
			Help:      "Total volume of request payloads emitted in bytes.",
		},
		labelNames,
	)
	responseBytes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Program,
			Name:      "response_bytes_total",
			Help:      "Total volume of response payloads emitted in bytes.",
		},
		labelNames,
	)

	log = logpkg.New(os.Stdout, "", logpkg.LstdFlags|logpkg.Lmicroseconds)
)

func main() {
	var (
		fsRoot      = flag.String("fs.root", "/tmp", "FileSystem root directory")
		httpAddress = flag.String("http.addr", ":5555", "HTTP listen address")
		providerDir = flag.String("provider.dir", "/tmp", "Provider directory with bucket policies")
	)
	flag.Parse()

	prometheus.MustRegister(requestDurations)
	prometheus.MustRegister(requestBytes)
	prometheus.MustRegister(responseBytes)

	var (
		fs = newDiskFS(*fsRoot)
		r  = pat.New()
	)

	p, err := newDiskProvider(*providerDir)
	if err != nil {
		log.Fatal(err)
	}

	r.Handle("/metrics", prometheus.Handler())
	r.Add(
		"GET",
		routeFile,
		report.JSON(
			os.Stdout,
			metrics(
				"handleGet",
				handleGet(p, fs),
			),
		),
	)
	r.Add(
		"GET",
		routeBucket,
		report.JSON(
			os.Stdout,
			metrics(
				"handleFileList",
				handleFileList(p, fs),
			),
		),
	)
	r.Add(
		"POST",
		routeFile,
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

func handleCreate(p ent.Provider, fs ent.FileSystem) http.HandlerFunc {
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
		defer f.Close()

		h, err := f.Hash()
		if err != nil {
			respondError(w, r, err)
			return
		}

		respondJSON(w, http.StatusCreated, ent.ResponseCreated{
			Duration: time.Since(start),
			File: ent.ResponseFile{
				Key:          key,
				SHA1:         h,
				Bucket:       b,
				LastModified: f.LastModified(),
			},
		})
	}
}

func handleGet(p ent.Provider, fs ent.FileSystem) http.HandlerFunc {
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
		defer f.Close()

		http.ServeContent(w, r, key, time.Now(), f)
	}
}

func handleBucketList(p ent.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			start = time.Now()
		)

		bs, err := p.List()
		if err != nil {
			respondError(w, r, err)
			return
		}

		respondJSON(w, http.StatusOK, ent.ResponseBucketList{
			Count:    len(bs),
			Duration: time.Since(start),
			Buckets:  bs,
		})
	}
}

func handleFileList(p ent.Provider, fs ent.FileSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			start      = time.Now()
			limit      = defaultLimit
			bucket     = r.URL.Query().Get(":bucket")
			limitValue = r.URL.Query().Get(paramLimit)
			prefix     = r.URL.Query().Get(paramPrefix)
			sortValue  = r.URL.Query().Get(paramSort)
		)

		b, err := p.Get(bucket)
		if err != nil {
			respondError(w, r, err)
			return
		}

		if limitValue != "" {
			limit, err = strconv.ParseUint(limitValue, 10, 64)
			if err != nil {
				respondError(w, r, ent.ErrInvalidParam)
				return
			}
		}

		sortStrategy, err := createSortStrategy(sortValue)
		if err != nil {
			respondError(w, r, err)
			return
		}

		files, err := fs.List(b, prefix, limit, sortStrategy)
		if err != nil {
			respondError(w, r, err)
			return
		}

		responseFiles, err := createResponseFiles(files, b)
		if err != nil {
			respondError(w, r, err)
			return
		}
		for _, file := range files {
			defer file.Close()
		}

		respondJSON(w, http.StatusOK, ent.ResponseFileList{
			Count:    len(responseFiles),
			Duration: time.Since(start),
			Bucket:   b,
			Files:    responseFiles,
		})
	}
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

		requestBytes.With(labels).Add(float64(rd.BytesRead))
		requestDurations.With(labels).Observe(float64(d))
		responseBytes.With(labels).Add(float64(rc.size))
	})
}

func respondError(w http.ResponseWriter, r *http.Request, err error) {
	code := http.StatusInternalServerError

	switch err {
	case ent.ErrBucketNotFound, ent.ErrFileNotFound:
		code = http.StatusNotFound
	case ent.ErrInvalidParam:
		code = http.StatusBadRequest
	}

	respondJSON(w, code, ent.ResponseError{
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

func createResponseFiles(files ent.Files, bucket *ent.Bucket) ([]ent.ResponseFile, error) {
	responseFiles := make([]ent.ResponseFile, len(files))
	for i, file := range files {
		h, err := file.Hash()
		if err != nil {
			return nil, err
		}

		responseFiles[i] = ent.ResponseFile{
			Key:          file.Key(),
			SHA1:         h,
			LastModified: file.LastModified(),
			Bucket:       bucket,
		}
	}
	return responseFiles, nil
}

func createSortStrategy(value string) (ent.SortStrategy, error) {
	if value == "" {
		return ent.NoOpStrategy(), nil
	}
	if len(value) == 1 {
		return nil, ent.ErrInvalidParam
	}

	var (
		asc       = true
		order     = value[:1]
		criterion = value[1:]
	)

	// check if the sort param starts the "+" or "-"
	switch order {
	case orderAscending:
		// nothing to do
	case orderDescending:
		asc = false
	default:
		return nil, ent.ErrInvalidParam
	}

	switch criterion {
	case orderKey:
		return ent.ByKeyStrategy(asc), nil
	case orderLastModified:
		return ent.ByLastModifiedStrategy(asc), nil
	default:
		return nil, ent.ErrInvalidParam
	}
}
