// Copyright (c) 2021 6 River Systems
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package actions

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/ent"
)

// this stuff only works in-process, but it helps latency a LOT there
// for h-scale, other retry timers dominate

type publishHook func(uuid.UUID)
type PublishHookHandle *publishHook

type modifyHook func(uuid.UUID, string)

// modifyHookHandle is a private general version of the hook handle. We use it
// internally to enable code reuse, while having separate types externally to
// promote type safety for the semantic differences between subscription and
// topic modify hooks, despite their signatures being the same
type modifyHookHandle = *modifyHook
type TopicModifyHookHandle *modifyHook
type SubscriptionModifyHookHandle *modifyHook

var nmu sync.Mutex

type notifier = chan struct{}
type notifySet = map[notifier]struct{}

type PublishNotifier notifier
type SubModifiedNotifier notifier
type AnySubModifiedNotifier notifier
type TopicModifiedNotifier notifier
type AnyTopicModifiedNotifier notifier

var pubNotifyHooks = map[PublishHookHandle]struct{}{}
var pubWaiters = map[uuid.UUID]map[PublishNotifier]struct{}{}

type modifyKey struct {
	id   uuid.UUID
	name string
}

var topicModifyHooks = map[modifyHookHandle]struct{}{}
var topicModifyWaiters = map[modifyKey]notifySet{}
var anyTopicModifiedWaiters = notifySet{}

var subModifyHooks = map[modifyHookHandle]struct{}{}
var subModifyWaiters = map[modifyKey]notifySet{}
var anySubModifiedWaiters = notifySet{}

func AddPublishHook(hook publishHook) PublishHookHandle {
	if hook == nil {
		panic(errors.New("nil hook"))
	}
	nmu.Lock()
	pubNotifyHooks[&hook] = struct{}{}
	nmu.Unlock()
	return &hook
}

func RemovePublishHook(handle PublishHookHandle) bool {
	if handle == nil || *handle == nil {
		panic(errors.New("nil handle"))
	}
	nmu.Lock()
	_, found := pubNotifyHooks[handle]
	delete(pubNotifyHooks, handle)
	nmu.Unlock()
	return found
}

func AddTopicModifyHook(hook modifyHook) TopicModifyHookHandle {
	if hook == nil {
		panic(errors.New("nil hook"))
	}
	nmu.Lock()
	topicModifyHooks[&hook] = struct{}{}
	nmu.Unlock()
	return &hook
}

func RemoveTopicModifyHook(handle TopicModifyHookHandle) bool {
	if handle == nil || *handle == nil {
		panic(errors.New("nil handle"))
	}
	nmu.Lock()
	_, found := topicModifyHooks[handle]
	delete(topicModifyHooks, handle)
	nmu.Unlock()
	return found
}

func AddSubscriptionModifyHook(hook modifyHook) SubscriptionModifyHookHandle {
	if hook == nil {
		panic(errors.New("nil hook"))
	}
	nmu.Lock()
	subModifyHooks[&hook] = struct{}{}
	nmu.Unlock()
	return &hook
}

func RemoveSubscriptionModifyHook(handle SubscriptionModifyHookHandle) bool {
	if handle == nil || *handle == nil {
		panic(errors.New("nil handle"))
	}
	nmu.Lock()
	_, found := subModifyHooks[handle]
	delete(subModifyHooks, handle)
	nmu.Unlock()
	return found
}

func notifyPublish(tx *ent.Tx, subIDs ...uuid.UUID) {
	tx.OnCommit(func(c ent.Committer) ent.Committer {
		return ent.CommitFunc(func(ctx context.Context, tx *ent.Tx) error {
			err := c.Commit(ctx, tx)
			if err == nil {
				WakePublishListeners(false, subIDs...)
			}
			return err
		})
	})
}

func NotifyModifyTopic(tx *ent.Tx, topicID uuid.UUID, topicName string) {
	tx.OnCommit(func(c ent.Committer) ent.Committer {
		return ent.CommitFunc(func(ctx context.Context, tx *ent.Tx) error {
			err := c.Commit(ctx, tx)
			if err == nil {
				WakeTopicListeners(false, topicID, topicName)
			}
			return err
		})
	})
}

func NotifyModifySubscription(tx *ent.Tx, subscriptionID uuid.UUID, subscriptionName string) {
	tx.OnCommit(func(c ent.Committer) ent.Committer {
		return ent.CommitFunc(func(ctx context.Context, tx *ent.Tx) error {
			err := c.Commit(ctx, tx)
			if err == nil {
				WakeSubscriptionListeners(false, subscriptionID, subscriptionName)
				// internal publish listeners also care about if the subscription
				// they're following was modified. External publish listeners will find
				// out about this by the receipt of the external notification calling
				// this separately
				WakePublishListeners(true, subscriptionID)
			}
			return err
		})
	})
}

// WakePublishListeners notifies about publishes to the given subscription
// IDs
func WakePublishListeners(onlyInternal bool, subIDs ...uuid.UUID) {
	nmu.Lock()
	defer nmu.Unlock()
	if !onlyInternal {
		for h := range pubNotifyHooks {
			for _, subID := range subIDs {
				// if hooks do do long ops, such as DB actions, they _must_ manage
				// pushing them into a goroutine themselves.
				(*h)(subID)
			}
		}
		// don't notify on both internal and external for the same event
		if len(pubNotifyHooks) != 0 {
			return
		}
	}
	for _, subID := range subIDs {
		waitSet := pubWaiters[subID]
		if waitSet == nil {
			return
		}
		for c := range waitSet {
			close(c)
			delete(waitSet, c)
		}
		if len(waitSet) == 0 {
			delete(pubWaiters, subID)
		}
	}
}

func WakeTopicListeners(onlyInternal bool, topicID uuid.UUID, topicName string) {
	wakeModifyListeners(topicModifyHooks, topicModifyWaiters, anyTopicModifiedWaiters, onlyInternal, topicID, topicName)
}

func WakeSubscriptionListeners(
	onlyInternal bool,
	subscriptionID uuid.UUID,
	subscriptionName string,
) {
	wakeModifyListeners(subModifyHooks, subModifyWaiters, anySubModifiedWaiters, onlyInternal, subscriptionID, subscriptionName)
}

func wakeModifyListeners(
	hooks map[modifyHookHandle]struct{},
	waiters map[modifyKey]notifySet,
	anyWaiters notifySet,
	onlyInternal bool,
	id uuid.UUID,
	name string,
) {
	nmu.Lock()
	defer nmu.Unlock()
	if !onlyInternal {
		for h := range hooks {
			// if hooks do do long ops, such as DB actions, they _must_ manage pushing
			// them into a goroutine themselves.
			(*h)(id, name)
		}
		// don't notify on both internal and external for the same event
		if len(hooks) != 0 {
			return
		}
	}
	// allow listeners that only care about uuid or name too
	mks := []modifyKey{
		{id, name},
		{id: id},
		{name: name},
	}
	for _, mk := range mks {
		waitSet := waiters[mk]
		if waitSet != nil {
			for c := range waitSet {
				close(c)
				delete(waitSet, c)
			}
			if len(waitSet) == 0 {
				delete(waiters, mk)
			}
		}
	}
	for c := range anyWaiters {
		close(c)
		delete(anyWaiters, c)
	}
}

// this has to return a two-way channel so that it can be passed to cancelAwaiter
func PublishAwaiter(subID uuid.UUID) PublishNotifier {
	c := make(PublishNotifier)
	nmu.Lock()
	defer nmu.Unlock()
	waitSet := pubWaiters[subID]
	if waitSet == nil {
		waitSet = make(map[PublishNotifier]struct{})
		pubWaiters[subID] = waitSet
	}
	waitSet[c] = struct{}{}
	return c
}

// this has to take a two-way channel so that it can find it in the map
func CancelPublishAwaiter(subID uuid.UUID, c PublishNotifier) {
	if c == nil {
		return
	}
	nmu.Lock()
	defer nmu.Unlock()
	waiters := pubWaiters[subID]
	if waiters != nil {
		delete(waiters, c)
	}
}

// this has to return a two-way channel so that it can be passed to cancelSubModifiedAwaiter
func SubModifiedAwaiter(subID uuid.UUID, subName string) SubModifiedNotifier { // nolint:deadcode,unused
	c := make(chan struct{})
	mk := modifyKey{subID, subName}
	nmu.Lock()
	defer nmu.Unlock()
	waitSet := subModifyWaiters[mk]
	if waitSet == nil {
		waitSet = make(map[chan struct{}]struct{})
		subModifyWaiters[mk] = waitSet
	}
	waitSet[c] = struct{}{}
	return c
}

// this has to take a two-way channel so that it can find it in the map
func CancelSubModifiedAwaiter(subID uuid.UUID, subName string, c SubModifiedNotifier) { // nolint:deadcode,unused
	if c == nil {
		return
	}
	mk := modifyKey{subID, subName}
	nmu.Lock()
	defer nmu.Unlock()
	waiters := subModifyWaiters[mk]
	if waiters != nil {
		delete(waiters, c)
	}
}

// this has to return a two-way channel so that it can be passed to CancelAnySubModifiedAwaiter
func AnySubModifiedAwaiter() AnySubModifiedNotifier {
	c := make(chan struct{})
	nmu.Lock()
	defer nmu.Unlock()
	anySubModifiedWaiters[c] = struct{}{}
	return c
}

// this has to take a two-way channel so that it can find it in the map
func CancelAnySubModifiedAwaiter(c AnySubModifiedNotifier) {
	if c == nil {
		return
	}
	nmu.Lock()
	defer nmu.Unlock()
	delete(anySubModifiedWaiters, c)
}

// this has to return a two-way channel so that it can be passed to cancelTopicModifiedAwaiter
func TopicModifiedAwaiter(topicID uuid.UUID, topicName string) TopicModifiedNotifier {
	c := make(chan struct{})
	mk := modifyKey{topicID, topicName}
	nmu.Lock()
	defer nmu.Unlock()
	waitSet := topicModifyWaiters[mk]
	if waitSet == nil {
		waitSet = make(map[chan struct{}]struct{})
		topicModifyWaiters[mk] = waitSet
	}
	waitSet[c] = struct{}{}
	return c
}

// this has to take a two-way channel so that it can find it in the map
func CancelTopicModifiedAwaiter(topicID uuid.UUID, topicName string, c TopicModifiedNotifier) {
	if c == nil {
		return
	}
	mk := modifyKey{topicID, topicName}
	nmu.Lock()
	defer nmu.Unlock()
	waiters := topicModifyWaiters[mk]
	if waiters != nil {
		delete(waiters, c)
	}
}

// this has to return a two-way channel so that it can be passed to CancelAnyTopicModifiedAwaiter
func AnyTopicModifiedAwaiter() AnyTopicModifiedNotifier {
	c := make(chan struct{})
	nmu.Lock()
	defer nmu.Unlock()
	anyTopicModifiedWaiters[c] = struct{}{}
	return c
}

// this has to take a two-way channel so that it can find it in the map
func CancelAnyTopicModifiedAwaiter(c AnyTopicModifiedNotifier) {
	if c == nil {
		return
	}
	nmu.Lock()
	defer nmu.Unlock()
	delete(anyTopicModifiedWaiters, c)
}
