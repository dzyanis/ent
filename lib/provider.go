package ent

// A Provider implements access to a collection of Buckets.
type Provider interface {
	Get(name string) (*Bucket, error)
	List() ([]*Bucket, error)
}
