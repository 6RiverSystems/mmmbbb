package ent

// NewNotFoundError creates a new NotFoundError with the given label. This is
// exposed for fault injections.
func NewNotFoundError(label string) *NotFoundError {
	return &NotFoundError{label}
}
