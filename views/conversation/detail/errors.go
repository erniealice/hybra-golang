package detail

import "errors"

var (
	errIDRequired = errors.New("conversation: id is required")
	errNotFound   = errors.New("conversation: not found")
)
