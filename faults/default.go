package faults

import "sync"

var defaultSet = &Set{sync.RWMutex{}, map[string][]*Description{}}

func DefaultSet() *Set {
	return defaultSet
}

func Check(op string, params Parameters) error {
	return defaultSet.Check(op, params)
}

func Add(d Description) {
	defaultSet.Add(d)
}
