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

package services

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"reflect"
	"time"

	"golang.org/x/sync/errgroup"
)

// parseShardConfig parses the MIRROR_SHARD_CONFIG environment variable, if
// present, to produce a topicFilter predicate. If the environment variable is
// present but invalid, it will panic.
func parseShardConfig() func(string) bool {
	if shardConfig := os.Getenv("MIRROR_SHARD_CONFIG"); shardConfig != "" {
		var thisShard, numShards uint32
		// treat MIRROR_SHARD_CONFIG as "n/m", only process topic names that hash to
		// n % m
		if n, err := fmt.Sscanf(shardConfig, "%u/%u", &thisShard, &numShards); err != nil {
			panic(err)
		} else if n != 2 {
			// should never get here
			panic(errors.New("bad format for MIRROR_SHARD_CONFIG"))
		}
		// else
		return func(topicName string) bool {
			h := crc32.NewIEEE()
			h.Write(([]byte)(topicName)) // nolint:errcheck // crc32 write never fails
			sum := h.Sum32()
			return sum%numShards == thisShard
		}
	}
	return nil
}

type monitoredGroup struct {
	*errgroup.Group
	context.Context
}

func monitor(ctx context.Context, f func(context.Context) error) monitoredGroup {
	subEG, subCtx := errgroup.WithContext(ctx)
	subEG.Go(func() error { return f(subCtx) })
	return monitoredGroup{subEG, subCtx}
}

type waitMonitorResult int

const (
	waitMonitorTimeout = iota
	waitMonitorContextDone
	waitMonitorNotified
	waitMonitorEnded
)

func waitMonitors(
	ctx context.Context,
	timeout time.Duration,
	notifier <-chan struct{},
	mons map[string]monitoredGroup,
) waitMonitorResult {
	selects := make([]reflect.SelectCase, 3, 3+len(mons))
	selects[waitMonitorTimeout] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(time.After(timeout)),
	}
	selects[waitMonitorContextDone] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	}
	selects[waitMonitorNotified] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(notifier)}
	for _, mg := range mons {
		selects = append(selects, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(mg.Done())})
	}

	chosen, _, _ := reflect.Select(selects)
	switch chosen {
	case waitMonitorTimeout, waitMonitorContextDone, waitMonitorNotified:
		return waitMonitorResult(chosen)
	default:
		// returned value is the context err, not the waitgroup err, we didn't track
		// the wg order and return doesn't say which one, so caller has to figure
		// this out for now. the point of this method is mainly to know when to
		// check the monitors not to know a specific one ended
		return waitMonitorEnded
	}
}
