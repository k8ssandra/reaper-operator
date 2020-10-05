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
	return s.updateReady(ctx, reaper, true)
}

func (s *StatusManager) SetNotReady(ctx context.Context, reaper *api.Reaper) error {
	return s.updateReady(ctx, reaper, false)
}

func (s *StatusManager) updateReady(ctx context.Context, reaper *api.Reaper, ready bool) error {
	patch := client.MergeFrom(reaper.DeepCopy())
	reaper.Status.Ready = ready
	return s.Status().Patch(ctx, reaper, patch)
}
