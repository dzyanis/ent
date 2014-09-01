package main

import (
	"sort"
)

const (
	key          = "key"
	lastModified = "lastModified"
	ascending    = "+"
	descending   = "-"
)

// SortStrategy implements sorting of Files
type SortStrategy interface {
	Sort(file Files)
}

func createSortStrategy(order string) SortStrategy {
	if order == "" {
		return NoOpStrategy{}
	}

	var (
		asc       = order[:1] == ascending
		criterion = order[1:]
	)

	baseSortStrategy := baseSortStrategy{
		ascending: asc,
	}

	switch criterion {
	case key:
		return &byKey{
			baseSortStrategy: baseSortStrategy,
		}
	case lastModified:
		return &byLastModified{
			baseSortStrategy: baseSortStrategy,
		}
	}

	return nil
}

// NoOpStrategy doesn't change the order of the files
type NoOpStrategy struct{}

// Sort keep the files unchanged
func (s NoOpStrategy) Sort(files Files) {}

type baseSortStrategy struct {
	Files
	ascending bool
}

type byKey struct {
	baseSortStrategy
}

func (s byKey) Sort(files Files) {
	s.Files = files
	sort.Sort(s)
}

func (s byKey) Less(i, j int) bool {
	var (
		iKey = s.Files[i].Key()
		jKey = s.Files[j].Key()
	)

	if s.ascending {
		return iKey < jKey
	}
	return iKey >= jKey
}

type byLastModified struct {
	baseSortStrategy
}

func (s byLastModified) Sort(files Files) {
	s.Files = files
	sort.Sort(s)
}

func (s byLastModified) Less(i, j int) bool {
	var (
		iLastModified = s.Files[i].LastModified()
		jLastModified = s.Files[j].LastModified()
	)

	if s.ascending {
		return iLastModified.Before(jLastModified)
	}
	return iLastModified.After(jLastModified)
}
