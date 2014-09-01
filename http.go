package main

import (
	"encoding/hex"
	"encoding/json"
	"time"
)

const (
	timeFormat = time.RFC3339Nano
)

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
	SHA1         []byte
	LastModified time.Time
	Bucket       *Bucket
}

// MarshalJSON returns a ResponseFile JSON encoding with conversion of the
// files SHA1 to hex.
func (r ResponseFile) MarshalJSON() ([]byte, error) {
	return json.Marshal(responseFileWrapper{
		Key:          r.Key,
		SHA1:         hex.EncodeToString(r.SHA1),
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
	h, err := hex.DecodeString(w.SHA1)
	if err != nil {
		return err
	}

	r.Key = w.Key
	r.SHA1 = h
	r.LastModified, err = time.Parse(timeFormat, w.LastModified)
	r.Bucket = w.Bucket
	return err
}

type responseFileWrapper struct {
	Key          string  `json:"key"`
	SHA1         string  `json:"sha1"`
	LastModified string  `json:"lastModified"`
	Bucket       *Bucket `json:"bucket"`
}
