package slicepool

import (
	"sync"
)

// SlicePool provides pooled slice management for frequently allocated slices
// to reduce garbage collection pressure in batch operations
type SlicePool struct {
	stringSlices sync.Pool
	anySlices    sync.Pool
}

// NewSlicePool creates a new slice pool with proper initialization
func NewSlicePool() *SlicePool {
	return &SlicePool{
		stringSlices: sync.Pool{
			New: func() any {
				// Start with reasonable capacity to avoid early reallocations
				slice := make([]string, 0, 16)
				return &slice
			},
		},
		anySlices: sync.Pool{
			New: func() any {
				// Start with reasonable capacity to avoid early reallocations
				slice := make([]any, 0, 16)
				return &slice
			},
		},
	}
}

// GetStringSlice retrieves a pooled string slice with at least the specified capacity
// The returned slice will have length 0 but capacity >= the requested capacity
func (sp *SlicePool) GetStringSlice(minCapacity int) []string {
	slicePtr := sp.stringSlices.Get().(*[]string)
	slice := *slicePtr

	// Reset length to 0 but preserve capacity
	slice = slice[:0]

	// Ensure we have enough capacity, reallocate if needed
	if cap(slice) < minCapacity {
		slice = make([]string, 0, minCapacity)
		*slicePtr = slice
	} else {
		*slicePtr = slice
	}

	return slice
}

// PutStringSlice returns a string slice to the pool for reuse
// The slice is cleaned (length set to 0) and returned to the pool
func (sp *SlicePool) PutStringSlice(s []string) {
	if cap(s) == 0 {
		return // Don't pool zero-capacity slices
	}

	// Clear the slice contents for security (avoid data leaks)
	for i := range s {
		s[i] = ""
	}

	// Reset to zero length but keep capacity
	s = s[:0]

	// Store pointer to slice in pool
	sp.stringSlices.Put(&s)
}

// GetAnySlice retrieves a pooled interface{} slice with at least the specified capacity
// The returned slice will have length 0 but capacity >= the requested capacity
func (sp *SlicePool) GetAnySlice(minCapacity int) []any {
	slicePtr := sp.anySlices.Get().(*[]any)
	slice := *slicePtr

	// Reset length to 0 but preserve capacity
	slice = slice[:0]

	// Ensure we have enough capacity, reallocate if needed
	if cap(slice) < minCapacity {
		slice = make([]any, 0, minCapacity)
		*slicePtr = slice
	} else {
		*slicePtr = slice
	}

	return slice
}

// PutAnySlice returns an interface{} slice to the pool for reuse
// The slice is cleaned (length set to 0, elements cleared) and returned to the pool
func (sp *SlicePool) PutAnySlice(s []any) {
	if cap(s) == 0 {
		return // Don't pool zero-capacity slices
	}

	// Clear the slice contents to avoid memory leaks
	for i := range s {
		s[i] = nil
	}

	// Reset to zero length but keep capacity
	s = s[:0]

	// Store pointer to slice in pool
	sp.anySlices.Put(&s)
}

// GetStringSliceWithLength retrieves a pooled string slice set to the specified length
// This is a convenience method for cases where you need a slice of a specific length
func (sp *SlicePool) GetStringSliceWithLength(length int) []string {
	slice := sp.GetStringSlice(length)

	// Extend slice to requested length (elements will be zero-valued)
	if cap(slice) >= length {
		slice = slice[:length]
	} else {
		// This shouldn't happen due to GetStringSlice logic, but handle gracefully
		slice = make([]string, length)
	}

	return slice
}

// GetAnySliceWithLength retrieves a pooled interface{} slice set to the specified length
// This is a convenience method for cases where you need a slice of a specific length
func (sp *SlicePool) GetAnySliceWithLength(length int) []any {
	slice := sp.GetAnySlice(length)

	// Extend slice to requested length (elements will be zero-valued/nil)
	if cap(slice) >= length {
		slice = slice[:length]
	} else {
		// This shouldn't happen due to GetAnySlice logic, but handle gracefully
		slice = make([]any, length)
	}

	return slice
}
