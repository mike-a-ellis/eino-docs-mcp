package storage

import "errors"

var (
	ErrQdrantUnreachable  = errors.New("qdrant server unreachable")
	ErrCollectionNotFound = errors.New("collection not found")
	ErrDimensionMismatch  = errors.New("embedding dimension mismatch")
)
