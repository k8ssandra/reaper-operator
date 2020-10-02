package reconcile

import (
	"context"

	api "github.com/thelastpickle/reaper-operator/api/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StatusManager struct {
	client.Client
}

func (s *StatusManager) SetReady(ctx context.Context, reaper *api.Reaper) error {
	patch := client.MergeFrom(reaper.DeepCopy())
	reaper.Status.Ready = true
	return s.Patch(ctx, reaper, patch)
}
