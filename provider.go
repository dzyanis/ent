// Copyright (c) 2014, SoundCloud Ltd.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/ent

package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/soundcloud/ent/lib"
)

const policyExt = ".entpolicy"

type diskProvider struct {
	buckets map[string]*ent.Bucket
	dir     string
}

func newDiskProvider(dir string) (ent.Provider, error) {
	p := &diskProvider{
		buckets: map[string]*ent.Bucket{},
		dir:     dir,
	}

	err := filepath.Walk(p.dir, p.walk)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (p *diskProvider) Get(name string) (*ent.Bucket, error) {
	b, ok := p.buckets[name]
	if !ok {
		return nil, ent.ErrBucketNotFound
	}
	return b, nil
}

func (p *diskProvider) List() ([]*ent.Bucket, error) {
	bs := []*ent.Bucket{}
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

	b := &ent.Bucket{}
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
