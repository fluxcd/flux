package api

import (
	"context"
	"time"

	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/service"
	"github.com/weaveworks/flux/update"
)

type Service interface {
	Client
	Upstream

	Status(context.Context) (service.Status, error)
	History(context.Context, update.ResourceSpec, time.Time, int64, time.Time) ([]history.Entry, error)
	GetConfig(ctx context.Context, fingerprint string) (service.InstanceConfig, error)
	SetConfig(context.Context, service.InstanceConfig) error
	PatchConfig(context.Context, service.ConfigPatch) error
}
