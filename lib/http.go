package ent

import (
	"encoding/json"
	"math"
	"time"
)

// Constants used in HTTP request/responses.
const (
	DefaultLimit uint64 = math.MaxUint64

	HeaderETag         = "ETag"
	HeaderLastModified = "Last-Modified"

	KeyBucket = ":bucket"
	KeyBlob   = ":key"

	OrderKey          = "key"
	OrderLastModified = "lastModified"
	OrderAscending    = "+"
	OrderDescending   = "-"

	ParamLimit  = "limit"
	ParamPrefix = "prefix"
	ParamSort   = "sort"

	RouteBucket = `/{bucket}`
	RouteFile   = `/{bucket}/{key:[a-zA-Z0-9\-_\.~\+\/]+}`

	timeFormat = time.RFC3339Nano
)

// ResponseCreated is used as the intermediate type to craft a response for
// a successful file upload.
type ResponseCreated struct {
	Duration time.Duration `json:"duration"`
	File     ResponseFile  `json:"file"`
}

// ResponseDeleted is used as the intermediate type to craft a response for a
// successfull file deletion
type ResponseDeleted struct {
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

// ResponseFileList is used as the intermediate type to craft a response for
// the retrieval of all files in a bucket.
type ResponseFileList struct {
	Count    int            `json:"count"`
	Duration time.Duration  `json:"duration"`
	Bucket   *Bucket        `json:"bucket"`
	Files    []ResponseFile `json:"files"`
}

// ResponseError is used as the intermediate type to craft a response for any
// kind of error condition in the http path. This includes common error cases
// like an entity could not be found.
type ResponseError struct {
	Code        int    `json:"code"`
	Error       string `json:"error"`
	Description string `json:"description"`
}

// ResponseFile is used as the intermediate type to craft a response for
// the retrieval metadata of a File.
type ResponseFile struct {
	Key          string
	LastModified time.Time
	Bucket       *Bucket
}

// MarshalJSON returns a ResponseFile JSON encoding with conversion of the
// files SHA1 to hex.
func (r ResponseFile) MarshalJSON() ([]byte, error) {
	return json.Marshal(responseFileWrapper{
		Key:          r.Key,
		LastModified: r.LastModified.Format(timeFormat),
		Bucket:       r.Bucket,
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

	r.Key = w.Key
	r.LastModified, err = time.Parse(timeFormat, w.LastModified)
	r.Bucket = w.Bucket
	return err
}

type responseFileWrapper struct {
	Key          string  `json:"key"`
	LastModified string  `json:"lastModified"`
	Bucket       *Bucket `json:"bucket"`
}
