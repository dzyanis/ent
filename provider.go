package main

import (
	"encoding/json"
	"fmt"
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

	// TODO(alx): Validate bucket configuration.
	p.buckets[b.Name] = b

	return nil
}

func (p *diskProvider) walk(path string, f os.FileInfo, err error) error {
	if err != nil {
		return fmt.Errorf("walking provider dir: %s", err)
	}
	if path != p.dir && f.IsDir() {
		return filepath.SkipDir
	}
	if filepath.Ext(path) != policyExt {
		return nil
	}

	return p.loadBucket(path)
}
