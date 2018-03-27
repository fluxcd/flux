package daemon

import (
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

type note struct {
	JobID  job.ID        `json:"jobID"`
	Spec   update.Spec   `json:"spec"`
	Result update.Result `json:"result"`
}
