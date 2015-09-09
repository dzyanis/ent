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
	"github.com/soundcloud/ent/lib"
	"github.com/streadway/handy/report"
)

// Buildtime variables
var (
	Program = "ent"
	Commit  = "0000000"
	Version = "0.0.0"
)

// Telemetry
var (
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

	// GET /metrics
	r.Handle("/metrics", prometheus.Handler())

	// DELETE /$bucket/$file
	r.Add(
		"DELETE",
		ent.RouteFile,
		report.JSON(
			os.Stdout,
			metrics(
				"handleDelete",
				handleDelete(p, fs),
			),
		),
	)
	// GET /$bucket/$file
	r.Add(
		"GET",
		ent.RouteFile,
		report.JSON(
			os.Stdout,
			metrics(
				"handleGet",
				addCORSHeaders(
					handleGet(p, fs),
				),
			),
		),
	)
	// HEAD /$bucket/$file
	r.Add(
		"HEAD",
		ent.RouteFile,
		report.JSON(
			os.Stdout,
			metrics(
				"handleExists",
				handleExists(p, fs),
			),
		),
	)
	// POST /$bucket/$file
	r.Add(
		"POST",
		ent.RouteFile,
		report.JSON(
			os.Stdout,
			metrics(
				"handleCreate",
				addCORSHeaders(
					handleCreate(p, fs),
				),
			),
		),
	)

	// GET /$bucket
	r.Add(
		"GET",
		ent.RouteBucket,
		report.JSON(
			os.Stdout,
			metrics(
				"handleFileList",
				addCORSHeaders(
					handleFileList(p, fs),
				),
			),
		),
	)

	// GET /
	r.Add(
		"GET",
		"/",
		report.JSON(
			os.Stdout,
			metrics(
				"handleBucketList",
				addCORSHeaders(
					handleBucketList(p),
				),
			),
		),
	)

	r.Add(
		"OPTIONS",
		"/{.*}",
		report.JSON(
			os.Stdout,
			metrics(
				"handleOptions",
				addCORSHeaders(
					handleOptions(),
				),
			),
		),
	)

	log.Printf("ent %s listening on %s", Version, *httpAddress)
	log.Fatal(http.ListenAndServe(*httpAddress, http.Handler(r)))
}

func handleCreate(p ent.Provider, fs ent.FileSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			bucket = r.URL.Query().Get(ent.KeyBucket)
			key    = r.URL.Query().Get(ent.KeyBlob)
			start  = time.Now()
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

		err = writeBlobHeaders(w, f)
		if err != nil {
			respondError(w, r, err)
			return
		}
		respondJSON(w, http.StatusCreated, ent.ResponseCreated{
			Duration: time.Since(start),
			File: ent.ResponseFile{
				Key:          key,
				Bucket:       b,
				LastModified: f.LastModified(),
			},
		})
	}
}

func handleDelete(p ent.Provider, fs ent.FileSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			bucket = r.URL.Query().Get(ent.KeyBucket)
			key    = r.URL.Query().Get(ent.KeyBlob)
			start  = time.Now()
		)
		defer r.Body.Close()

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

		err = fs.Delete(b, key)
		if err != nil {
			respondError(w, r, err)
			return
		}

		respondJSON(w, http.StatusOK, ent.ResponseCreated{
			Duration: time.Since(start),
			File: ent.ResponseFile{
				Bucket:       b,
				Key:          key,
				LastModified: f.LastModified(),
			},
		})
	}
}

func handleExists(p ent.Provider, fs ent.FileSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			bucket = r.URL.Query().Get(ent.KeyBucket)
			key    = r.URL.Query().Get(ent.KeyBlob)
		)

		b, err := p.Get(bucket)
		if err != nil {
			respondHEAD(w, errorStatusCode(err))
			return
		}

		f, err := fs.Open(b, key)
		if err != nil {
			respondHEAD(w, errorStatusCode(err))
			return
		}
		defer f.Close()

		err = writeBlobHeaders(w, f)
		if err != nil {
			respondHEAD(w, errorStatusCode(err))
			return
		}

		respondHEAD(w, http.StatusOK)
	}
}

func handleGet(p ent.Provider, fs ent.FileSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			bucket = r.URL.Query().Get(ent.KeyBucket)
			key    = r.URL.Query().Get(ent.KeyBlob)
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

		err = writeBlobHeaders(w, f)
		if err != nil {
			respondError(w, r, err)
			return
		}

		http.ServeContent(w, r, key, f.LastModified(), f)
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
			limit      = ent.DefaultLimit
			bucket     = r.URL.Query().Get(ent.KeyBucket)
			limitValue = r.URL.Query().Get(ent.ParamLimit)
			prefix     = r.URL.Query().Get(ent.ParamPrefix)
			sortValue  = r.URL.Query().Get(ent.ParamSort)
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

func handleOptions() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func addCORSHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		next.ServeHTTP(w, r)
	})
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
			"bucket":    r.URL.Query().Get(ent.KeyBucket),
			"method":    strings.ToLower(r.Method),
			"operation": op,
			"status":    strconv.Itoa(rc.status),
		}

		requestBytes.With(labels).Add(float64(rd.BytesRead))
		requestDurations.With(labels).Observe(float64(d))
		responseBytes.With(labels).Add(float64(rc.size))
	})
}

func errorStatusCode(err error) int {
	code := http.StatusInternalServerError
	switch err {
	case ent.ErrBucketNotFound, ent.ErrFileNotFound:
		code = http.StatusNotFound
	case ent.ErrInvalidParam:
		code = http.StatusBadRequest
	}
	return code
}

func respondError(w http.ResponseWriter, r *http.Request, err error) {
	code := errorStatusCode(err)
	respondJSON(w, code, ent.ResponseError{
		Code:        code,
		Error:       err.Error(),
		Description: http.StatusText(code),
	})
}

func respondHEAD(w http.ResponseWriter, code int) {
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(code)
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
		responseFiles[i] = ent.ResponseFile{
			Key:          file.Key(),
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
	case ent.OrderAscending:
		// nothing to do
	case ent.OrderDescending:
		asc = false
	default:
		return nil, ent.ErrInvalidParam
	}

	switch criterion {
	case ent.OrderKey:
		return ent.ByKeyStrategy(asc), nil
	case ent.OrderLastModified:
		return ent.ByLastModifiedStrategy(asc), nil
	default:
		return nil, ent.ErrInvalidParam
	}
}

func writeBlobHeaders(w http.ResponseWriter, f ent.File) error {
	h, err := f.Hash()
	if err != nil {
		return err
	}

	w.Header().Add(ent.HeaderETag, hex.EncodeToString(h))
	w.Header().Add(ent.HeaderLastModified, f.LastModified().Format(time.RFC3339Nano))
	return nil
}
