package ent

import (
	"net/mail"
)

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
