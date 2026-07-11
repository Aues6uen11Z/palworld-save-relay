package storage

import "errors"

// ErrNotFound is returned by Storage.Get/Delete when an object does not exist.
var ErrNotFound = errors.New("storage: object not found")
