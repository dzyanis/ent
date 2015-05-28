package ent

// A Provider implements access to a collection of Buckets.
type Provider interface {
	Get(name string) (*Bucket, error)
	List() ([]*Bucket, error)
}

// MemoryProvider is an in-memory Provider implementation.
type MemoryProvider struct {
	buckets map[string]*Bucket
}

// NewMemoryProvider returns a MemoryProvider instance.
func NewMemoryProvider(buckets ...*Bucket) Provider {
	p := &MemoryProvider{
		buckets: map[string]*Bucket{},
	}

	for _, b := range buckets {
		p.buckets[b.Name] = b
	}

	return p
}

// Get returns the Bucket for the given name.
func (p *MemoryProvider) Get(name string) (*Bucket, error) {
	b, ok := p.buckets[name]
	if !ok {
		return nil, ErrBucketNotFound
	}

	return b, nil
}

// Init performs the necessary setup (noop).
func (p *MemoryProvider) Init() error {
	return nil
}

// List returns all stored Buckets.
func (p *MemoryProvider) List() ([]*Bucket, error) {
	bs := []*Bucket{}

	for _, b := range p.buckets {
		bs = append(bs, b)
	}

	return bs, nil
}
