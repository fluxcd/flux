package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fluxcd/flux/pkg/cluster"

	"github.com/fluxcd/flux/pkg/resource"

	v6 "github.com/fluxcd/flux/pkg/api/v6"
)

func Test_outputWorkloadsJson(t *testing.T) {
	buf := &bytes.Buffer{}

	t.Run("sends JSON to the io.Writer", func(t *testing.T) {
		workloads := testWorkloads(5)
		err := outputWorkloadsJson(workloads, buf)
		require.NoError(t, err)
		unmarshallTarget := &[]v6.ControllerStatus{}
		err = json.Unmarshal(buf.Bytes(), unmarshallTarget)
		require.NoError(t, err)
	})
}

func testWorkloads(workloadCount int) []v6.ControllerStatus {
	workloads := []v6.ControllerStatus{}
	for i := 0; i < workloadCount; i++ {
		name := fmt.Sprintf("mah-app-%d", i)
		id := resource.MakeID("applications", "deployment", name)

		cs := v6.ControllerStatus{
			ID:         id,
			Containers: nil,
			ReadOnly:   "",
			Status:     "ready",
			Rollout:    cluster.RolloutStatus{},
			SyncError:  "",
			Antecedent: resource.ID{},
			Labels:     nil,
			Automated:  false,
			Locked:     false,
			Ignore:     false,
			Policies:   nil,
		}

		workloads = append(workloads, cs)

	}
	return workloads
}
