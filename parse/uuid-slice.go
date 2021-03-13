package parse

import "github.com/google/uuid"

func UUIDsFromStrings(values []string) ([]uuid.UUID, error) {
	if values == nil {
		return nil, nil
	}
	ids := make([]uuid.UUID, len(values))
	var err error
	for i, s := range values {
		if ids[i], err = uuid.Parse(s); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

func UUIDLess(a, b uuid.UUID) bool {
	for n := 0; n < len(a); n++ {
		if a[n] < b[n] {
			return true
		}
		if b[n] < a[n] {
			return false
		}
	}
	return false
}
