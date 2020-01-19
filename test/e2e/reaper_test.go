package e2e

import (
	goctx "context"
	"github.com/jsanda/reaper-operator/pkg/apis"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/jsanda/reaper-operator/test/e2eutil"
	"testing"
	"time"

	reaperv1 "github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
)

func TestDeployReaperWithMemoryBackend(t *testing.T) {
	reaperList := &reaperv1.ReaperList{}
	if err := framework.AddToFrameworkScheme(apis.AddToScheme, reaperList); err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	ctx, f := e2eutil.InitOperator(t)
	defer ctx.Cleanup()

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("Failed to get namespace: %s", err)
	}

	reaper := reaperv1.Reaper{
		TypeMeta: metav1.TypeMeta{
			Kind: "Reaper",
			APIVersion: "reaper.cassandra-reaper.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "reaper-e2e",
			Namespace: namespace,
		},
		Spec: reaperv1.ReaperSpec{
			ServerConfig: reaperv1.ServerConfig{
				StorageType: "memory",
			},
		},
	}

	cleanup := &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 5, RetryInterval: time.Second * 1}
	if err = f.Client.Create(goctx.TODO(), &reaper, cleanup); err != nil {
		t.Fatalf("Failed to create Reaper: %s\n", err)
	}

	if err = e2eutil.WaitForReaper(t, f, reaper.Namespace, reaper.Name, 3 * time.Second, 1 * time.Minute); err != nil {
		t.Fatalf("Timed out waiting for Reaper (%s) to be ready: %s\n", reaper.Name, err)
	}
}
