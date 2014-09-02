package main

import (
	"fmt"
	"net/mail"
	"reflect"
	"testing"

	"github.com/soundcloud/ent/lib"
)

func TestDiskProviderInit(t *testing.T) {
	p, err := newDiskProvider("./fixture")
	if err != nil {
		t.Fatal(err)
	}

	names := []string{"bit", "doge", "ripples"}

	for _, name := range names {
		addr, err := mail.ParseAddress(fmt.Sprintf("%s team <%s@bucket.io>", name, name))
		if err != nil {
			t.Fatal(err)
		}

		expected := ent.NewBucket(name, ent.Owner{Email: *addr})
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
	p, err := newDiskProvider("./fixture")
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Get("fake-bucket")
	if !ent.IsBucketNotFound(err) {
		t.Errorf("got wrong error: %s", err)
	}
}
