// Copyright (c) 2014, SoundCloud Ltd.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/ent

package main

import (
	"fmt"
	"net/mail"
	"reflect"
	"testing"
)

func TestDiskProviderInit(t *testing.T) {
	p, err := NewDiskProvider("./fixture")
	if err != nil {
		t.Fatal(err)
	}

	names := []string{"bit", "doge", "ripples"}

	for _, name := range names {
		addr, err := mail.ParseAddress(fmt.Sprintf("%s team <%s@bucket.io>", name, name))
		if err != nil {
			t.Fatal(err)
		}

		expected := NewBucket(name, Owner{*addr})
		got, err := p.Get(name)
		if err != nil {
			t.Errorf("error retrieving %s: %s", name, err)
		}

		if got.Name != expected.Name {
			t.Errorf("wrong name: %#v != %#v", got.Name, expected.Name)
		}
		if !reflect.DeepEqual(got.Owner, expected.Owner) {
			t.Errorf("wrong owner: %v != %v", got.Owner, expected.Owner)
		}
	}

	bs, err := p.List()
	if err != nil {
		t.Fatal(err)
	}

	if len(bs) != 3 {
		t.Errorf("wrong number of buckets returned: %d", len(bs))
	}
}

func TestDiskProviderBucketNotFound(t *testing.T) {
	p, err := NewDiskProvider("./fixtures")
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Get("fake-bucket")
	if !IsBucketNotFound(err) {
		t.Errorf("got wrong error: %s", err)
	}
}
