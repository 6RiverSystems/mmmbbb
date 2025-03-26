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
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type PruneCommonParams struct {
	MinAge    time.Duration `json:"minAge"`
	MaxDelete int           `json:"maxDelete"`
}

func (p *PruneCommonParams) Validate() error {
	if p.MinAge < 0 {
		return errors.New("minAge must be >= 0")
	}
	if p.MaxDelete <= 0 {
		return errors.New("maxDelete must be > 0")
	}

	return nil
}

type PruneCommonResults struct {
	NumDeleted int `json:"numDeleted"`
}

type pruneAction struct {
	actionBase[PruneCommonParams, PruneCommonResults]
}

var _ ActionData[PruneCommonParams, PruneCommonResults] = (*pruneAction)(nil)

func newPruneAction(params PruneCommonParams) pruneAction {
	if err := params.Validate(); err != nil {
		panic(err)
	}
	return pruneAction{
		actionBase[PruneCommonParams, PruneCommonResults]{
			params: params,
		},
	}
}

func pruneMetrics(name string) (prometheus.Counter, *prometheus.HistogramVec) {
	return actionMetrics("prune_"+name, "items", "deleted")
}
