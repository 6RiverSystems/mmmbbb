package faults

import "sync"

var defaultSet = &Set{sync.RWMutex{}, map[string][]*Description{}, 0}

func DefaultSet() *Set {
	return defaultSet
}

func Prune() {
	defaultSet.Prune()
}

func Check(op string, params Parameters) error {
	return defaultSet.Check(op, params)
}

func Add(d Description) {
	defaultSet.Add(d)
}
