package ent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

var defaultListOptions = &ListOptions{
	Limit:  DefaultLimit,
	Prefix: "",
	Sort:   NoOpStrategy(),
}

// Client provides an interface to interact with Ent over HTTP.
type Client struct {
	addr   string
	client *http.Client
}

// New returns a new Client instance given an address and an http.Client,
// http.DefaultClient is used if client is not passed.
func New(addr string, client *http.Client) *Client {
	if client == nil {
		client = http.DefaultClient
	}

	return &Client{
		addr:   addr,
		client: client,
	}
}

// Create stores or replaces the blob under key with the content of src.
func (c *Client) Create(
	bucket, key string,
	src io.Reader,
) (*ResponseFile, error) {
	if bucket == "" {
		return nil, ErrEmptyBucket
	}

	if key == "" {
		return nil, ErrEmptyKey
	}

	if src == nil {
		return nil, ErrEmptySource
	}

	var (
		r = &ResponseCreated{}
		u = fmt.Sprintf("%s/%s", bucket, key)
	)

	_, err := c.request("POST", u, src, r)
	if err != nil {
		return nil, err
	}

	return &r.File, nil
}

// Get returns the file stored under bucket and key.
func (c *Client) Get(bucket, key string) (io.ReadCloser, error) {
	if bucket == "" {
		return nil, ErrEmptyBucket
	}

	if key == "" {
		return nil, ErrEmptyKey
	}

	u := fmt.Sprintf("%s/%s", bucket, key)

	return c.request("GET", u, nil, nil)
}

// List returns the list of ResponseFiles for a bucket potentially
// filtered by the provided options.
func (c *Client) List(
	bucket string,
	opts *ListOptions,
) ([]ResponseFile, error) {
	if bucket == "" {
		return nil, ErrEmptyBucket
	}

	if opts == nil {
		opts = defaultListOptions
	}

	var (
		l = ResponseFileList{}
		u = fmt.Sprintf("%s?%s", bucket, opts.EncodeParams())
	)

	_, err := c.request("GET", u, nil, &l)
	if err != nil {
		return nil, err
	}

	return l.Files, nil
}

func (c *Client) request(
	method string,
	uri string,
	body io.Reader,
	obj interface{},
) (io.ReadCloser, error) {
	req, err := http.NewRequest(method, fmt.Sprintf("%s/%s", c.addr, uri), body)
	if err != nil {
		return nil, newError(ErrClient, err.Error())
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, newError(ErrClient, err.Error())
	}

	if res.StatusCode >= 400 {
		rErr := &ResponseError{}

		err := json.NewDecoder(res.Body).Decode(rErr)
		if err != nil {
			return nil, newError(ErrClient, err.Error())
		}

		return nil, newError(
			ErrClient,
			fmt.Sprintf("response %d: %s", rErr.Code, rErr.Error),
		)
	}

	if obj != nil {
		defer res.Body.Close()

		if res.Header.Get("Content-Type") != "application/json" {
			return nil, newError(
				ErrClient,
				fmt.Sprintf("unexpected content-type: %s", res.Header.Get("Content-Type")),
			)
		}

		err = json.NewDecoder(res.Body).Decode(obj)
		if err != nil {
			return nil, newError(ErrClient, fmt.Sprintf("decode: %s", err))
		}

		return nil, nil
	}

	return res.Body, nil
}

// ListOptions specifies the details of a listing like prefix to filter, amount
// of files to return.
type ListOptions struct {
	Limit  uint64
	Prefix string
	Sort   SortStrategy
}

// EncodeParams returns a string that can be used as URL params.
func (o ListOptions) EncodeParams() string {
	vs := url.Values{}

	if o.Limit > 0 && o.Limit < DefaultLimit {
		vs.Set(ParamLimit, fmt.Sprintf("%d", o.Limit))
	}

	if o.Prefix != "" {
		vs.Set(ParamPrefix, o.Prefix)
	}

	if o.Sort != nil {
		if p := o.Sort.EncodeParam(); p != "" {
			vs.Set(ParamSort, p)
		}
	}

	return vs.Encode()
}
