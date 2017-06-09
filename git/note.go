package git

import (
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

type Note struct {
	JobID  job.ID        `json:"jobID"`
	Spec   update.Spec   `json:"spec"`
	Result update.Result `json:"result"`
}
