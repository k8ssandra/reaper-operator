package clusters

import (
	"context"
	"github.com/thelastpickle/reaper-operator/pkg/apis"
	"github.com/thelastpickle/reaper-operator/pkg/apis/reaper/v1alpha1"
	"github.com/thelastpickle/reaper-operator/pkg/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	reapergo "github.com/jsanda/reaper-client-go/reaper"
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	reaperNamespace = "monitor-test"
	reaperName      = "monitor-test"
)

func TestCheckReapers(t *testing.T) {
	if err := apis.AddToScheme(scheme.Scheme); err != nil {
		t.FailNow()
	}

	t.Run("CheckReapersClustersOutOfSync", testCheckReapersClusterOutOfSync)
}

func testCheckReapersClusterOutOfSync(t *testing.T) {
	cluster := v1alpha1.CassandraCluster{
		Name: "cluster-1",
		Service: v1alpha1.CassandraService{
			Name: "cluster-1",
			Namespace: reaperNamespace,
		},
	}

	reaper := &v1alpha1.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reaperNamespace,
			Name: reaperName,
		},
		Spec: v1alpha1.ReaperSpec{
			Clusters: []v1alpha1.CassandraCluster{cluster},
		},
		Status: v1alpha1.ReaperStatus{
			Clusters: []v1alpha1.CassandraCluster{cluster},
		},
	}

	reaperCh := make(chan v1alpha1.Reaper)

	go func() {
		reaperCh <- *reaper
		close(reaperCh)
	}()

	k8sClient := fake.NewFakeClientWithScheme(scheme.Scheme, reaper)

	restClient := testutil.NewFakeRESTClient()
	restClient.GetClustersStub = func(ctx context.Context) <-chan reapergo.GetClusterResult {
		resultCh := make(chan reapergo.GetClusterResult)
		defer close(resultCh)
		return resultCh
	}

	checkReapers(context.TODO(), k8sClient, reaperCh)

	namespaceName := types.NamespacedName{
		Namespace: reaperNamespace,
		Name: reaperName,
	}
	reaper = &v1alpha1.Reaper{}
	err := k8sClient.Get(context.TODO(), namespaceName, reaper)
	if err != nil {
		t.Fatalf("failed to get Reaper: %s", err)
	}

	assert.Empty(t, reaper.Status.Clusters, "expected .Status.Clusters to be cleared but found %+v", reaper.Status.Clusters)
}
