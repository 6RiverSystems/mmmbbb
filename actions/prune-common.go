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
		return errors.New("MinAge must be >= 0")
	}
	if p.MaxDelete <= 0 {
		return errors.New("MaxDelete must be > 0")
	}

	return nil
}

func (p *PruneCommonParams) ToInterface() map[string]interface{} {
	return map[string]interface{}{
		"minAge":    p.MinAge,
		"maxDelete": p.MaxDelete,
	}
}

type PruneCommonResults struct {
	NumDeleted int `json:"numDeleted"`
}

func (r *PruneCommonResults) ToInterface() map[string]interface{} {
	return map[string]interface{}{
		"numDeleted": r.NumDeleted,
	}
}

type pruneAction struct {
	params  PruneCommonParams
	results *PruneCommonResults
}

func newPruneAction(params PruneCommonParams) *pruneAction {
	if err := params.Validate(); err != nil {
		panic(err)
	}
	return &pruneAction{
		params: params,
	}
}

func (a *pruneAction) NumDeleted() int {
	return a.results.NumDeleted
}

func (a *pruneAction) Parameters() map[string]interface{} {
	return a.params.ToInterface()
}

func (a *pruneAction) HasResults() bool {
	return a.results != nil
}

func (a *pruneAction) Results() map[string]interface{} {
	return a.results.ToInterface()
}

func pruneMetrics(name string) (prometheus.Counter, *prometheus.HistogramVec) {
	return actionMetrics("prune_"+name, "items", "deleted")
}
