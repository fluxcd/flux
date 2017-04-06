package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
)

func validateTask(s swarm.Service, t swarm.Task) bool {
	// Similarly, checks to see if a task is worth considering for inclusion.
	// Only include running tasks.
	return t.Status.State == swarm.TaskStateRunning
}

func (c *Swarm) AllServices(namespace string) ([]cluster.Service, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := c.client.ServiceList(ctx, types.ServiceListOptions{})
	pss := make([]cluster.Service, len(s))
	if err != nil {
		return pss, err
	}
	for k, v := range s {
		cn := strings.Split(v.Spec.Annotations.Name, "_")[1]
		ps := cluster.Service{
			ID:       flux.MakeServiceID(v.Spec.TaskTemplate.ContainerSpec.Labels["com.docker.stack.namespace"], cn),
			IP:       "?",
			Metadata: v.Spec.Annotations.Labels,
			//			Status:     string(v.UpdateStatus.State),
			Containers: cluster.ContainersOrExcuse{},
		}
		args := filters.NewArgs()
		args.Add("service", v.ID)
		ts, err := c.client.TaskList(ctx, types.TaskListOptions{Filters: args})
		if err != nil {
			return pss, err
		}
		pcs := []cluster.Container{}
		for _, t := range ts {
			if validateTask(v, t) {
				pcs = append(pcs, cluster.Container{
					Name:  fmt.Sprintf("%s.%d.%s", v.Spec.Name, t.Slot, t.ID),
					Image: t.Spec.ContainerSpec.Image,
				})
			}
		}
		ps.Containers.Containers = pcs
		pss[k] = ps
	}
	return pss, nil
}

func (c *Swarm) SomeServices(ids []flux.ServiceID) (res []cluster.Service, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	args := filters.NewArgs()
	for _, v := range ids {
		namespace, svc := v.Components()
		args.Add("name", fmt.Sprintf("%s_%s", namespace, svc))
	}
	s, err := c.client.ServiceList(ctx, types.ServiceListOptions{args})

	pss := make([]cluster.Service, 0)
	if err != nil {
		return pss, err
	}

	for _, v := range s {
		cn := strings.Split(v.Spec.Annotations.Name, "_")[1]
		ps := cluster.Service{
			ID:       flux.MakeServiceID(v.Spec.TaskTemplate.ContainerSpec.Labels["com.docker.stack.namespace"], cn),
			IP:       "?",
			Metadata: v.Spec.Annotations.Labels,
			//Status:     string(v.UpdateStatus.State),
			Containers: cluster.ContainersOrExcuse{},
		}
		args := filters.NewArgs()
		args.Add("service", v.ID)
		ts, err := c.client.TaskList(ctx, types.TaskListOptions{Filters: args})
		if err != nil {
			return pss, err
		}
		pcs := []cluster.Container{}
		for _, t := range ts {
			if validateTask(v, t) {
				pcs = append(pcs, cluster.Container{
					Name:  fmt.Sprintf("%s.%d.%s", v.Spec.Name, t.Slot, t.ID),
					Image: t.Spec.ContainerSpec.Image,
				})
			}
		}
		ps.Containers.Containers = pcs
		pss = append(pss, ps)
	}
	return pss, nil
}
