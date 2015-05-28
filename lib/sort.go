package ent

import (
	"fmt"
	"sort"
)

// SortStrategy implements sorting of Files
type SortStrategy interface {
	EncodeParam() string
	Sort(file Files)
}

// noOpStrategy doesn't change the order of the files.
type noOpStrategy struct{}

// NoOpStrategy returns a SortStrategy which keeps the order.
func NoOpStrategy() SortStrategy {
	return noOpStrategy{}
}

// Sort is a convenience method.
func (s noOpStrategy) Sort(files Files) {}

// EncodeParam returns the cannonical string used for the strategy when passed
// as a param.
func (s noOpStrategy) EncodeParam() string {
	return ""
}

// byKey orders Files by its key name.
type byKey struct {
	baseSortStrategy
}

// ByKeyStrategy returns a SortStrategy ordering by key name.
func ByKeyStrategy(ascending bool) SortStrategy {
	return byKey{
		baseSortStrategy: baseSortStrategy{
			isAscending: ascending,
		},
	}
}

// EncodeParam returns the cannonical string used for the strategy when passed
// as a param.
func (s byKey) EncodeParam() string {
	order := OrderDescending

	if s.isAscending {
		order = OrderAscending
	}

	return fmt.Sprintf("%s%s", order, OrderKey)
}

// Less reports whether the element with index i should sort before the element
// with index j.
func (s byKey) Less(i, j int) bool {
	var (
		iKey = s.Files[i].Key()
		jKey = s.Files[j].Key()
	)

	if s.isAscending {
		return iKey < jKey
	}
	return iKey >= jKey
}

// Sort is a convenience method.
func (s byKey) Sort(files Files) {
	s.Files = files
	sort.Sort(s)
}

// byLastModified orders Files by their modification date.
type byLastModified struct {
	baseSortStrategy
}

// ByLastModifiedStrategy returns a SortStrategy ordering by a files
// modification time.
func ByLastModifiedStrategy(ascending bool) SortStrategy {
	return byLastModified{
		baseSortStrategy: baseSortStrategy{
			isAscending: ascending,
		},
	}
}

// EncodeParam returns the cannonical string used for the strategy when passed
// as a param.
func (s byLastModified) EncodeParam() string {
	order := OrderDescending

	if s.isAscending {
		order = OrderAscending
	}

	return fmt.Sprintf("%s%s", order, OrderLastModified)
}

// Less reports whether the element with index i should sort before the element
// with index j.
func (s byLastModified) Less(i, j int) bool {
	var (
		iLastModified = s.Files[i].LastModified()
		jLastModified = s.Files[j].LastModified()
	)

	if s.isAscending {
		return iLastModified.Before(jLastModified)
	}
	return iLastModified.After(jLastModified)
}

// Sort is a convenience method.
func (s byLastModified) Sort(files Files) {
	s.Files = files
	sort.Sort(s)
}

type baseSortStrategy struct {
	Files
	isAscending bool
}

func (s baseSortStrategy) Len() int {
	return len(s.Files)
}

func (s baseSortStrategy) Swap(i, j int) {
	s.Files[i], s.Files[j] = s.Files[j], s.Files[i]
}
