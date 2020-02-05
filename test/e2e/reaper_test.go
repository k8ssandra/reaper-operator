package e2e

import (
	goctx "context"
	"github.com/jsanda/reaper-operator/pkg/apis"
	"github.com/jsanda/reaper-operator/test/e2eutil"
	casskop "github.com/Orange-OpenSource/cassandra-k8s-operator/pkg/apis/db/v1alpha1"
	casskopapi "github.com/Orange-OpenSource/cassandra-k8s-operator/pkg/apis"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
	"time"

	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
)

const (
	cassandraClusterName = "reaper-cluster"
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

func TestDeployReaperWithMemoryBackend(t *testing.T) {
	reaperList := &v1alpha1.ReaperList{}
	if err := framework.AddToFrameworkScheme(apis.AddToScheme, reaperList); err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	ctx, f := e2eutil.InitOperator(t)
	defer ctx.Cleanup()

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

	if err = e2eutil.WaitForReaperToBeReady(t, f, reaper.Namespace, reaper.Name, 3 * time.Second, 3 * time.Minute); err != nil {
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

	if err = e2eutil.WaitForDeploymentToBeDeleted(t, f, reaper.Namespace, reaper.Name, 5 * time.Second, 1 * time.Minute); err != nil {
		t.Errorf("Timed out waiting for Deployment (%s) to get deleted: %s", reaper.Name, err)
	}

	if err = e2eutil.WaitForConfigMapToBeDeleted(t, f, reaper.Namespace, reaper.Name, 5 * time.Second, 1 * time.Minute); err != nil {
		t.Errorf("Timed out waiting for ConfigMap (%s) to get deleted: %s", reaper.Name, err)
	}

	if err = e2eutil.WaitForServiceToBeDeleted(t, f, reaper.Namespace, reaper.Name, 5 * time.Second, 1 * time.Minute); err != nil {
		t.Errorf("Timed out waiting for Service (%s) to get deleted: %s", reaper.Name, err)
	}
}

func TestDeployReaperWithCassandraBackend(t *testing.T) {
	reaperList := &v1alpha1.ReaperList{}
	if err := framework.AddToFrameworkScheme(apis.AddToScheme, reaperList); err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	ccList := &casskop.CassandraClusterList{}
	if err := framework.AddToFrameworkScheme(casskopapi.AddToScheme, ccList); err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	ctx, f := e2eutil.InitOperator(t)
	defer ctx.Cleanup()

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("Failed to get namespace: %s", err)
	}

	if err := createCassandraCluster(cassandraClusterName, namespace, f, ctx); err != nil {
		t.Fatalf("Failed to create CassandraCluster: %s", err)
	}

	if err := e2eutil.WaitForCassKopCluster(t, f, namespace, cassandraClusterName, 10 * time.Second, 3 * time.Minute); err != nil {
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

	if err = e2eutil.WaitForReaperToBeReady(t, f, reaper.Namespace, reaper.Name, 3 * time.Second, 3 * time.Minute); err != nil {
		t.Fatalf("Timed out waiting for Reaper (%s) to be ready: %s\n", reaper.Name, err)
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
