package e2e

import (
	goctx "context"
	"github.com/jsanda/reaper-operator/pkg/apis"
	"github.com/jsanda/reaper-operator/test/e2eutil"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
	"time"

	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
)

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

	if err = e2eutil.WaitForReaperToBeReady(t, f, reaper.Namespace, reaper.Name, 3 * time.Second, 1 * time.Minute); err != nil {
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
}
