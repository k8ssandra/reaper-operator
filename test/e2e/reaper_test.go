package e2e

import (
	goctx "context"
	casskopapi "github.com/Orange-OpenSource/cassandra-k8s-operator/pkg/apis"
	casskop "github.com/Orange-OpenSource/cassandra-k8s-operator/pkg/apis/db/v1alpha1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/thelastpickle/reaper-operator/pkg/apis"
	"github.com/thelastpickle/reaper-operator/test/e2eutil"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"
	"testing"
	"time"

	"github.com/thelastpickle/reaper-operator/pkg/apis/reaper/v1alpha1"
)

const (
	cassandraClusterName  = "reaper-cluster"
	cassandraReadyTimeout = 5 * time.Minute
	reaperRetryInterval   = 3 * time.Second
	reaperReadyTimeout    = 5 * time.Minute
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 60
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 60
)

func cleanupWithPolling(ctx *framework.TestCtx) *framework.CleanupOptions {
	return &framework.CleanupOptions{
		TestContext:   ctx,
		Timeout:       cleanupTimeout,
		RetryInterval: cleanupRetryInterval,
	}
}

type E2ETest func(t *testing.T, f *framework.Framework, ctx *framework.TestCtx)

func e2eTest(test E2ETest) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, f := e2eutil.InitOperator(t)
		defer ctx.Cleanup()

		test(t, f, ctx)
	}
}

func TestReaper(t *testing.T) {
	reaperList := &v1alpha1.ReaperList{}
	if err := framework.AddToFrameworkScheme(apis.AddToScheme, reaperList); err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	ccList := &casskop.CassandraClusterList{}
	if err := framework.AddToFrameworkScheme(casskopapi.AddToScheme, ccList); err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	t.Run("DeployReaperMemoryBackend", e2eTest(testDeployReaperMemoryBackend))
	t.Run("DeployReaperCassandraBackend", e2eTest(testDeployReaperCassandraBackend))
	t.Run("AddDeleteManagedCluster", e2eTest(testAddDeleteManagedCluster))
	t.Run("UpdateReaperConfiguration", e2eTest(testUpdateReaperConfiguration))
}

func testDeployReaperMemoryBackend(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("Failed to get namespace: %s", err)
	}

	reaper := v1alpha1.Reaper{
		TypeMeta: metav1.TypeMeta{
			Kind: "Reaper",
			APIVersion: "reaper.cassandra-reaper.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "reaper-e2e",
			Namespace: namespace,
		},
		Spec: v1alpha1.ReaperSpec{
			ServerConfig: v1alpha1.ServerConfig{
				StorageType: "memory",
			},
		},
	}

	retryInterval := 5 * time.Second
	deleteTimeout := 1 * time.Minute

	cleanup := &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 5, RetryInterval: time.Second * 1}
	if err = f.Client.Create(goctx.TODO(), &reaper, cleanup); err != nil {
		t.Fatalf("Failed to create Reaper: %s\n", err)
	}

	if err = e2eutil.WaitForReaperToBeReady(t, f, reaper.Namespace, reaper.Name, reaperRetryInterval, reaperReadyTimeout); err != nil {
		t.Fatalf("Timed out waiting for Reaper (%s) to be ready: %s\n", reaper.Name, err)
	}

	// Now delete the Reaper object and verify all underlying objects get cleaned up

	instance := &v1alpha1.Reaper{}
	if err = f.Client.Get(goctx.TODO(), types.NamespacedName{Namespace: reaper.Namespace, Name: reaper.Name}, instance); err != nil {
		t.Fatalf("Failed to get Reaper (%s): %s", reaper.Name, err)
	}

	if err = f.Client.Delete(goctx.TODO(), instance); err != nil {
		t.Errorf("Failed to delete Reaper (%s): %s", reaper.Name, err)
	}

	if err = e2eutil.WaitForDeploymentToBeDeleted(t, f, reaper.Namespace, reaper.Name, retryInterval, deleteTimeout); err != nil {
		t.Errorf("Timed out waiting for Deployment (%s) to get deleted: %s", reaper.Name, err)
	}

	if err = e2eutil.WaitForConfigMapToBeDeleted(t, f, reaper.Namespace, reaper.Name, retryInterval, deleteTimeout); err != nil {
		t.Errorf("Timed out waiting for ConfigMap (%s) to get deleted: %s", reaper.Name, err)
	}

	if err = e2eutil.WaitForServiceToBeDeleted(t, f, reaper.Namespace, reaper.Name, retryInterval, deleteTimeout); err != nil {
		t.Errorf("Timed out waiting for Service (%s) to get deleted: %s", reaper.Name, err)
	}
}

func testDeployReaperCassandraBackend(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("Failed to get namespace: %s", err)
	}

	if err := createCassandraCluster(cassandraClusterName, namespace, f, ctx); err != nil {
		t.Fatalf("Failed to create CassandraCluster: %s", err)
	}

	if err := e2eutil.WaitForCassKopCluster(t, f, namespace, cassandraClusterName, 10 * time.Second, cassandraReadyTimeout); err != nil {
		t.Fatalf("Failed waiting for CassandraCluster to become ready: %s\n", err)
	}

	reaper := v1alpha1.Reaper{
		TypeMeta: metav1.TypeMeta{
			Kind: "Reaper",
			APIVersion: "reaper.cassandra-reaper.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "reaper-e2e",
			Namespace: namespace,
		},
		Spec: v1alpha1.ReaperSpec{
			ServerConfig: v1alpha1.ServerConfig{
				StorageType: "cassandra",
				CassandraBackend: &v1alpha1.CassandraBackend{
					ClusterName: cassandraClusterName,
					ContactPoints: []string{"reaper-cluster"},
					Keyspace: "reaper",
				},
			},
		},
	}

	if err = f.Client.Create(goctx.TODO(), &reaper, cleanupWithPolling(ctx)); err != nil {
		t.Fatalf("Failed to create Reaper: %s\n", err)
	}

	if err = e2eutil.WaitForReaperToBeReady(t, f, reaper.Namespace, reaper.Name, reaperRetryInterval, reaperReadyTimeout); err != nil {
		t.Fatalf("Timed out waiting for Reaper (%s) to be ready: %s\n", reaper.Name, err)
	}
}

func testAddDeleteManagedCluster(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("Failed to get namespace: %s", err)
	}

	if err := createCassandraCluster(cassandraClusterName, namespace, f, ctx); err != nil {
		t.Fatalf("Failed to create CassandraCluster: %s", err)
	}

	if err := e2eutil.WaitForCassKopCluster(t, f, namespace, cassandraClusterName, 10 * time.Second, cassandraReadyTimeout); err != nil {
		t.Fatalf("Failed waiting for CassandraCluster to become ready: %s\n", err)
	}

	reaper := v1alpha1.Reaper{
		TypeMeta: metav1.TypeMeta{
			Kind: "Reaper",
			APIVersion: "reaper.cassandra-reaper.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "reaper-e2e",
			Namespace: namespace,
		},
		Spec: v1alpha1.ReaperSpec{
			ServerConfig: v1alpha1.ServerConfig{
				StorageType: "memory",
			},
			Clusters: []v1alpha1.CassandraCluster{
				{
					Name: cassandraClusterName,
					Service: v1alpha1.CassandraService{
						Name: cassandraClusterName,
						Namespace: namespace,
					},
				},
			},
		},
	}

	cleanup := &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 5, RetryInterval: time.Second * 1}
	if err = f.Client.Create(goctx.TODO(), &reaper, cleanup); err != nil {
		t.Fatalf("Failed to create Reaper: %s\n", err)
	}

	if err = e2eutil.WaitForReaperToBeReady(t, f, reaper.Namespace, reaper.Name, reaperRetryInterval, reaperReadyTimeout); err != nil {
		t.Fatalf("Timed out waiting for Reaper (%s) to be ready: %s\n", reaper.Name, err)
	}

	clusterAdded := func(reaper *v1alpha1.Reaper) (bool, error) {
		for _, cc := range reaper.Status.Clusters {
			if cc.Name == cassandraClusterName {
				return true, nil
			}
		}
		return false, nil
	}

	clusterStatusTimeout := 1 * time.Minute
	retryInterval := 1 * time.Second

	if err = e2eutil.WaitForReaperCondition(t, f, reaper.Namespace, reaper.Name, retryInterval, clusterStatusTimeout, clusterAdded); err != nil {
		t.Errorf("Timed out waiting for status to be updated after adding cluster: %s", err)
	}

	// Now we need to remove the cluster from the spec. First, we need to reload the Reaper
	// object so that we have the latest version. Then we simply reassign .Spec.Clusters to an
	// empty slice which effectively removes the cluster.

	name := reaper.Name
	reaper = v1alpha1.Reaper{}
	if err := f.Client.Get(goctx.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, &reaper); err!= nil {
		t.Fatalf("failed to reload Reaper before deleting cluster: %s", err)
	}

	reaper.Spec.Clusters = []v1alpha1.CassandraCluster{}
	if err = f.Client.Update(goctx.TODO(), &reaper); err != nil {
		t.Fatalf("failed to update Reaper after removing cluster: %s", err)
	}

	clusterDeleted := func(reaper *v1alpha1.Reaper) (bool, error) {
		for _, cc := range reaper.Status.Clusters {
			if cc.Name == cassandraClusterName {
				return false, nil
			}
		}
		return true, nil
	}

	if err = e2eutil.WaitForReaperCondition(t, f, reaper.Namespace, reaper.Name, retryInterval, clusterStatusTimeout, clusterDeleted); err != nil {
		t.Errorf("Timed out waiting for status to be updated after deleting cluster: %s", err)
	}
}

func testUpdateReaperConfiguration(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("Failed to get namespace: %s", err)
	}

	reaper := v1alpha1.Reaper{
		TypeMeta: metav1.TypeMeta{
			Kind: "Reaper",
			APIVersion: "reaper.cassandra-reaper.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "reaper-e2e",
			Namespace: namespace,
		},
		Spec: v1alpha1.ReaperSpec{
			ServerConfig: v1alpha1.ServerConfig{
				StorageType: "memory",
			},
		},
	}

	cleanup := &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 5, RetryInterval: time.Second * 1}
	if err = f.Client.Create(goctx.TODO(), &reaper, cleanup); err != nil {
		t.Fatalf("Failed to create Reaper: %s\n", err)
	}

	if err = e2eutil.WaitForReaperToBeReady(t, f, reaper.Namespace, reaper.Name, reaperRetryInterval, reaperReadyTimeout); err != nil {
		t.Fatalf("Timed out waiting for Reaper (%s) to be ready: %s\n", reaper.Name, err)
	}

	instance := &v1alpha1.Reaper{}
	if err = f.Client.Get(goctx.TODO(), types.NamespacedName{Namespace: reaper.Namespace, Name: reaper.Name}, instance); err != nil {
		t.Fatalf("Failed to get Reaper (%s): %s", reaper.Name, err)
	}

	deployment := &appsv1.Deployment{}
	if err = f.Client.Get(goctx.TODO(), types.NamespacedName{Namespace: reaper.Namespace, Name: reaper.Name}, deployment); err != nil {
		t.Fatalf("failed to get Deployment: %s", err)
	}

	cond := deploymentutil.GetDeploymentCondition(deployment.Status, appsv1.DeploymentProgressing)
	if cond == nil {
		t.Fatalf("failed to get Deployment condition (%s)", appsv1.DeploymentProgressing)
	}

	// Make a copy of the LastUpdateTime so that we can compare after the config update has completed. The condition
	// should get updated when a new ReplicaSet is rolled out. The LastUpdateTime timestamp should be updated as part
	// of that change.
	lastUpdated := cond.LastUpdateTime

	// First we need to update the Reaper object
	segmentCount := int32(32)
	instance.Spec.ServerConfig.SegmentCountPerNode = &segmentCount

	if err = f.Client.Update(goctx.TODO(), instance); err != nil {
		t.Fatalf("failed to update Reaper: %s", err)
	}

	// Wait for the Reaper object to be ready while the config update should be happening
	if err = e2eutil.WaitForReaperToBeReady(t, f, reaper.Namespace, reaper.Name, reaperRetryInterval, reaperReadyTimeout); err != nil {
		t.Fatalf("Timed out waiting for Reaper (%s) to be ready after updating Reaper.Spec.ServerConfig: %s\n", reaper.Name, err)
	}

	// Verify that the ConfigMap has been updated. This requires multiple steps:
	//
	// 1) Verify that the ConfigMap has reaper.yaml in it
	// 2) Unmarshal reaper.yaml into a ServerConfig
	// 3) Verify that the ServerConfig.SegmentCountPerNode has the expected value
	if cm, err := f.KubeClient.CoreV1().ConfigMaps(reaper.Namespace).Get(reaper.Name, metav1.GetOptions{}); err == nil {
		if config, found := cm.Data["reaper.yaml"]; found {
			serverConfig := v1alpha1.ServerConfig{}
			if err = yaml.Unmarshal([]byte(config), &serverConfig); err != nil {
				t.Errorf("failed to unmarshal reaper.yaml: %s", err)
			} else {
				if serverConfig.SegmentCountPerNode == nil || *serverConfig.SegmentCountPerNode != 32 {
					t.Errorf("failed to update SegmentCountPerNode. expected (%d), got (%d)", segmentCount, serverConfig.SegmentCountPerNode)
				}
			}
		} else {
			t.Errorf("failed to find reaper.yaml in ConfigMap")
		}
	} else {
		t.Errorf("failed to get ConfigMap after updating Reaper.Spec.ServerConfig: %s", err)
	}

	// Now verify that Reaper has been restarted which is done by the Deployment rolling out a new ReplicaSet
	deployment = &appsv1.Deployment{}
	if err = f.Client.Get(goctx.TODO(), types.NamespacedName{Namespace: reaper.Namespace, Name: reaper.Name}, deployment); err == nil {
		cond := deploymentutil.GetDeploymentCondition(deployment.Status, appsv1.DeploymentProgressing)
		if cond == nil {
			t.Errorf("failed to get Deployment condition (%s) after updating Reaper.Spec.ServerConfig", appsv1.DeploymentProgressing)
		} else if !cond.LastUpdateTime.After(lastUpdated.Time) {
			t.Errorf("cannot verify that a new ReplicaSet was rolled out with LastUpdateTime. expected (%v) to be after (%v)", cond.LastUpdateTime, lastUpdated)
		}
	} else {
		t.Errorf("failed to get Deployment after updating Reaper.Spec.ServerConfig: %s", err)
	}
}

func createCassandraCluster(name string, namespace string, f *framework.Framework, ctx *framework.TestCtx) error {
	cc := casskop.CassandraCluster{
		TypeMeta:   metav1.TypeMeta{
			Kind: "CassandraCluster",
			APIVersion: "db.orange.com/v1alpha",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Namespace: namespace,
		},
		Spec: casskop.CassandraClusterSpec{
			DeletePVC: true,
			DataCapacity: "5Gi",
			Resources: casskop.CassandraResources{
				Requests: casskop.CPUAndMem{
					CPU: "500m",
					Memory: "1Gi",
				},
				Limits: casskop.CPUAndMem{
					CPU: "500m",
					Memory: "1Gi",
				},
			},
		},
	}
	return f.Client.Create(goctx.TODO(), &cc, cleanupWithPolling(ctx))
}
