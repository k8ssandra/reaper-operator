package testutil

import (
	"context"
	reapergo "github.com/jsanda/reaper-client-go/reaper"
)

type FakeRESTClient struct {
	GetClusterNamesStub func (ctx context.Context) ([]string, error)

	GetClusterStub func (ctx context.Context, name string) (*reapergo.Cluster, error)

	GetClustersStub func (ctx context.Context) <-chan reapergo.GetClusterResult

	GetClustersSyncStub func (ctx context.Context) ([]*reapergo.Cluster, error)

	AddClusterStub func (ctx context.Context, cluster string, seed string) error

	DeleteClusterStub func (ctx context.Context, cluster string) error
}

func NewFakeRESTClient() *FakeRESTClient {
	return &FakeRESTClient{
		GetClusterNamesStub: func(ctx context.Context) (strings []string, err error) {
			return []string {}, nil
		},

		GetClusterStub: func(ctx context.Context, name string) (*reapergo.Cluster, error) {
			return nil, nil
		},

		GetClustersStub: func(ctx context.Context) <-chan reapergo.GetClusterResult {
			results := make(chan reapergo.GetClusterResult)
			defer close(results)
			return results
		},

		GetClustersSyncStub: func(ctx context.Context) (clusters []*reapergo.Cluster, err error) {
			return []*reapergo.Cluster{}, nil
		},

		AddClusterStub: func(ctx context.Context, cluster string, seed string) error {
			return nil
		},

		DeleteClusterStub: func(ctx context.Context, cluster string) error {
			return nil
		},
	}
}

func (c *FakeRESTClient) GetClusterNames(ctx context.Context) ([]string, error) {
	return c.GetClusterNamesStub(ctx)
}

func (c *FakeRESTClient) GetCluster(ctx context.Context, name string) (*reapergo.Cluster, error) {
	return c.GetClusterStub(ctx, name)
}

func (c *FakeRESTClient) GetClusters(ctx context.Context) <-chan reapergo.GetClusterResult {
	return c.GetClustersStub(ctx)
}

func (c *FakeRESTClient) GetClustersSync(ctx context.Context) ([]*reapergo.Cluster, error) {
	return c.GetClustersSyncStub(ctx)
}

func (c *FakeRESTClient) AddCluster(ctx context.Context, cluster string, seed string) error {
	return c.AddClusterStub(ctx, cluster, seed)
}

func (c *FakeRESTClient) DeleteCluster(ctx context.Context, cluster string) error {
	return c.DeleteClusterStub(ctx, cluster)
}
