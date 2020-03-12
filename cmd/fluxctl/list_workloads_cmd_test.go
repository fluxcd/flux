package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fluxcd/flux/pkg/cluster"

	"github.com/fluxcd/flux/pkg/resource"

	v6 "github.com/fluxcd/flux/pkg/api/v6"
)

const replicas int = 1

var expectedJson = "[" +
	"{\"ID\":\"applications:deployment/mah-app-0\",\"Containers\":null,\"ReadOnly\":\"\",\"Status\":\"ready\"," +
	"\"Rollout\":{\"Desired\":" + strconv.Itoa(replicas) + ",\"Updated\":0,\"Ready\":0,\"Available\":0,\"Outdated\":0,\"Messages\":null}," +
	"\"SyncError\":\"\",\"Antecedent\":\"\",\"Labels\":null,\"Automated\":false,\"Locked\":false,\"Ignore\":false," +
	"\"Policies\":null}," +
	"{\"ID\":\"applications:deployment/mah-app-1\",\"Containers\":null,\"ReadOnly\":\"\",\"Status\":\"ready\"," +
	"\"Rollout\":{\"Desired\":" + strconv.Itoa(replicas) + ",\"Updated\":0,\"Ready\":0,\"Available\":0,\"Outdated\":0,\"Messages\":null}," +
	"\"SyncError\":\"\",\"Antecedent\":\"\",\"Labels\":null,\"Automated\":false,\"Locked\":false,\"Ignore\":false,\"Policies\":null}," +
	"{\"ID\":\"applications:deployment/mah-app-2\",\"Containers\":null,\"ReadOnly\":\"\",\"Status\":\"ready\"," +
	"\"Rollout\":{\"Desired\":" + strconv.Itoa(replicas) + ",\"Updated\":0,\"Ready\":0,\"Available\":0,\"Outdated\":0,\"Messages\":null}," +
	"\"SyncError\":\"\",\"Antecedent\":\"\",\"Labels\":null,\"Automated\":false,\"Locked\":false,\"Ignore\":false,\"Policies\":null}," +
	"{\"ID\":\"applications:deployment/mah-app-3\",\"Containers\":null,\"ReadOnly\":\"\",\"Status\":\"ready\"," +
	"\"Rollout\":{\"Desired\":" + strconv.Itoa(replicas) + ",\"Updated\":0,\"Ready\":0,\"Available\":0,\"Outdated\":0,\"Messages\":null}," +
	"\"SyncError\":\"\",\"Antecedent\":\"\",\"Labels\":null,\"Automated\":false,\"Locked\":false,\"Ignore\":false,\"Policies\":null}," +
	"{\"ID\":\"applications:deployment/mah-app-4\",\"Containers\":null,\"ReadOnly\":\"\",\"Status\":\"ready\"," +
	"\"Rollout\":{\"Desired\":" + strconv.Itoa(replicas) + ",\"Updated\":0,\"Ready\":0,\"Available\":0,\"Outdated\":0,\"Messages\":null}," +
	"\"SyncError\":\"\",\"Antecedent\":\"\",\"Labels\":null,\"Automated\":false,\"Locked\":false,\"Ignore\":false,\"Policies\":null}]\n"

func Test_outputWorkloadsJson(t *testing.T) {
	buf := &bytes.Buffer{}

	t.Run("sends JSON to the io.Writer", func(t *testing.T) {
		workloads := testWorkloads(5)
		err := outputWorkloadsJson(workloads, buf)
		if fmt.Sprint(buf) != expectedJson {
			t.Errorf("It did not get expected result:\n\n%s\n\nInstead got:\n\n%s", expectedJson, fmt.Sprint(buf))
		}
		require.NoError(t, err)
		unmarshallTarget := &[]v6.ControllerStatus{}
		err = json.Unmarshal(buf.Bytes(), unmarshallTarget)
		require.NoError(t, err)
	})
}

func Test_outputWorkloadsTab(t *testing.T) {
	var opts *workloadListOpts = &workloadListOpts{
		noHeaders: false,
	}

	t.Run("sends Tab to the io.Writer", func(t *testing.T) {
		workloads := testWorkloads(5)
		outputWorkloadsTab(workloads, opts)
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
			Rollout: cluster.RolloutStatus{
				Desired: 1,
			},
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
