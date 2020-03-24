package reaper

import (
	"context"
	reapergo "github.com/jsanda/reaper-client-go/reaper"
)

type fakeRESTClient struct {
	getClusterNames func (ctx context.Context) ([]string, error)

	getCluster func (ctx context.Context, name string) (*reapergo.Cluster, error)

	getClusters func (ctx context.Context) <-chan reapergo.GetClusterResult

	getClustersSync func (ctx context.Context) ([]*reapergo.Cluster, error)

	addCluster func (ctx context.Context, cluster string, seed string) error

	deleteCluster func (ctx context.Context, cluster string) error
}

func newFakeRESTClient() reapergo.ReaperClient {
	return &fakeRESTClient{
		getClusterNames: func(ctx context.Context) (strings []string, err error) {
			return []string {}, nil
		},

		getCluster: func(ctx context.Context, name string) (*reapergo.Cluster, error) {
			return nil, nil
		},

		getClusters: func(ctx context.Context) <-chan reapergo.GetClusterResult {
			results := make(chan reapergo.GetClusterResult)
			defer close(results)
			return results
		},

		getClustersSync: func(ctx context.Context) (clusters []*reapergo.Cluster, err error) {
			return []*reapergo.Cluster{}, nil
		},

		addCluster: func(ctx context.Context, cluster string, seed string) error {
			return nil
		},

		deleteCluster: func(ctx context.Context, cluster string) error {
			return nil
		},
	}
}

func (c *fakeRESTClient) GetClusterNames(ctx context.Context) ([]string, error) {
	return c.getClusterNames(ctx)
}

func (c *fakeRESTClient) GetCluster(ctx context.Context, name string) (*reapergo.Cluster, error) {
	return c.getCluster(ctx, name)
}

func (c *fakeRESTClient) GetClusters(ctx context.Context) <-chan reapergo.GetClusterResult {
	return c.getClusters(ctx)
}

func (c *fakeRESTClient) GetClustersSync(ctx context.Context) ([]*reapergo.Cluster, error) {
	return c.getClustersSync(ctx)
}

func (c *fakeRESTClient) AddCluster(ctx context.Context, cluster string, seed string) error {
	return c.addCluster(ctx, cluster, seed)
}

func (c *fakeRESTClient) DeleteCluster(ctx context.Context, cluster string) error {
	return c.deleteCluster(ctx, cluster)
}
