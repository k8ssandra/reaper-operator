package status

import (
	"context"

	cassdcv1beta1 "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/thelastpickle/reaper-operator/api/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StatusManager struct {
	client.Client
}

// Sets .status.ready to true and patch the status.
func (s *StatusManager) SetReady(ctx context.Context, reaper *api.Reaper) error {
	return s.updateReady(ctx, reaper, true)
}

// Sets .status.ready to false and patch the status.
func (s *StatusManager) SetNotReady(ctx context.Context, reaper *api.Reaper) error {
	return s.updateReady(ctx, reaper, false)
}

func (s *StatusManager) updateReady(ctx context.Context, reaper *api.Reaper, ready bool) error {
	patch := client.MergeFrom(reaper.DeepCopy())
	reaper.Status.Ready = ready
	return s.Status().Patch(ctx, reaper, patch)
}

// Adds the cluster to .status.clusters if it not already in the list. The status is patch
// updated if the list is modified.
func (s *StatusManager) AddClusterToStatus(ctx context.Context, reaper *api.Reaper, cassdc *cassdcv1beta1.CassandraDatacenter) error {
	if contains(reaper.Status.Clusters, cassdc.Spec.ClusterName) {
		return nil
	}

	patch := client.MergeFrom(reaper.DeepCopy())
	clusters := append(reaper.Status.Clusters, cassdc.Spec.ClusterName)
	reaper.Status.Clusters = clusters

	return s.Status().Patch(ctx, reaper, patch)
}

// Removes the cluster from .status.clusters if it is in the list. The status is patch
// updated if the list is modified.
func (s *StatusManager) RemoveClusterFromStatus(ctx context.Context, reaper *api.Reaper, cassdc *cassdcv1beta1.CassandraDatacenter) error {
	if !contains(reaper.Status.Clusters, cassdc.Spec.ClusterName) {
		return nil
	}

	patch := client.MergeFrom(reaper.DeepCopy())
	reaper.Status.Clusters = remove(reaper.Status.Clusters, cassdc.Spec.ClusterName)

	return s.Status().Patch(ctx, reaper, patch)
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func remove(slice []string, s string) []string {
	newSlice := make([]string, 0)
	for _, v := range slice {
		if v != s {
			newSlice = append(newSlice, s)
		}
	}

	return newSlice
}
