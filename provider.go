// Copyright (c) 2014, SoundCloud Ltd.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/ent

package main

import (
	"encoding/json"
	"net/mail"
	"os"
	"path/filepath"
)

const policyExt = ".entpolicy"

// A Provider implements access to a collection of Buckets.
type Provider interface {
	Get(name string) (*Bucket, error)
	List() ([]*Bucket, error)
}

type diskProvider struct {
	buckets map[string]*Bucket
	dir     string
}

func (p *diskProvider) Get(name string) (*Bucket, error) {
	b, ok := p.buckets[name]
	if !ok {
		return nil, ErrBucketNotFound
	}
	return b, nil
}

func (p *diskProvider) List() ([]*Bucket, error) {
	bs := []*Bucket{}
	for _, b := range p.buckets {
		bs = append(bs, b)
	}
	return bs, nil
}

func (p *diskProvider) loadBucket(name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}

	b := &Bucket{}
	err = json.NewDecoder(f).Decode(b)
	if err != nil {
		return err
	}

	p.buckets[b.Name] = b

	return nil
}

func (p *diskProvider) walk(path string, f os.FileInfo, err error) error {
	if path != p.dir && f.IsDir() {
		return filepath.SkipDir
	}
	if filepath.Ext(path) != policyExt {
		return nil
	}

	return p.loadBucket(path)
}

// NewDiskProvider returns a new disk backed Provider given a path to directory
// storing bucket configuration files in the format of .entpolicy.
func NewDiskProvider(dir string) (Provider, error) {
	p := &diskProvider{
		buckets: map[string]*Bucket{},
		dir:     dir,
	}

	err := filepath.Walk(p.dir, p.walk)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// A Bucket carries configuration for namespaces like ownership and
// restrictions.
type Bucket struct {
	Name  string `json:"name"`
	Owner Owner  `json:"owner"`
}

// NewBucket returns a new Bucket given a name and an Owner.
func NewBucket(name string, owner Owner) *Bucket {
	return &Bucket{
		Name:  name,
		Owner: owner,
	}
}

// An Owner represents the identity of a person or group.
type Owner struct {
	Email mail.Address `json:"email"`
}
