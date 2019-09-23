package daemon

import (
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/update"
)

type note struct {
	JobID  job.ID        `json:"jobID"`
	Spec   update.Spec   `json:"spec"`
	Result update.Result `json:"result"`
}
