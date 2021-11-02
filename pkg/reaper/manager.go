package reaper

import (
	"context"
	"fmt"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/reaper-client-go/reaper"
	reapergo "github.com/k8ssandra/reaper-client-go/reaper"
	api "github.com/k8ssandra/reaper-operator/api/v1alpha1"
)

type ReaperManager interface {
	Connect(reaper *api.Reaper) error
	AddClusterToReaper(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) error
	VerifyClusterIsConfigured(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) (bool, error)
}

// RestReaperManager abstracts the ugly details of how to connect to the Reaper instance
type RestReaperManager struct {
	reaperClient reaper.ReaperClient
}

func (r *RestReaperManager) Connect(reaper *api.Reaper) error {
	// Include the namespace in case Reaper is deployed in a different namespace than
	// the CassandraDatacenter.
	reaperSvc := reaper.Name + "-reaper-service" + "." + reaper.Namespace

	reaperClient, err := reapergo.NewReaperClient(fmt.Sprintf("http://%s:8080", reaperSvc))
	if err != nil {
		return err
	}
	r.reaperClient = reaperClient
	return nil
}

func (r *RestReaperManager) AddClusterToReaper(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) error {
	return r.reaperClient.AddCluster(ctx, cassdc.Spec.ClusterName, cassdc.GetDatacenterServiceName())
}

func (r *RestReaperManager) VerifyClusterIsConfigured(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) (bool, error) {
	_, err := r.reaperClient.GetCluster(ctx, cassdc.Spec.ClusterName)
	if err != nil {
		if err == reaper.CassandraClusterNotFound {
			// We didn't have issues verifying the existence, but the cluster isn't there
			return false, nil
		}
		return false, err
	}
	return true, nil
}
