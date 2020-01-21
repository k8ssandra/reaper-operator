package reaper

import (
	"context"
	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

func TestSetDefaults(t *testing.T) {
	var (
		name            = "reaper-operator"
		namespace       = "reaper"
		namespaceName = types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}
	)

	reaper := &v1alpha1.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name: name,
		},
		Spec: v1alpha1.ReaperSpec{
		},
	}

	// Objects to track in the fake client
	objs := []runtime.Object{reaper}

	// Register operator types with the runtime scheme
	s := scheme.Scheme
	s.AddKnownTypes(v1alpha1.SchemeGroupVersion, reaper)

	// Create a fake client to mock API calls
	cl := fake.NewFakeClientWithScheme(s, objs...)

	// Create a ReconcileReaper object with the scheme and fake client
	r := &ReconcileReaper{scheme: s, client: cl}

	// Mock request to simulate Reconcile() being called on an event for a watched resource
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	// Check the result of reconciliation to make sure it has the desired state
	if !res.Requeue {
		t.Error("reconcile did not requeue request as expected")
	}

	// verify default values set
	instance := &v1alpha1.Reaper{}
	if err = r.client.Get(context.TODO(), namespaceName, instance); err != nil {
		t.Fatalf("Failed to get Reaper: (%v)", err)
	}

	if *instance.Spec.ServerConfig.HangingRepairTimeoutMins != v1alpha1.DefaultHangingRepairTimeoutMins {
		t.Errorf("HangingRepairTimeoutMins (%d) is not the expected value: %d", *instance.Spec.ServerConfig.HangingRepairTimeoutMins, v1alpha1.DefaultHangingRepairTimeoutMins)
	}

	if instance.Spec.ServerConfig.RepairIntensity != v1alpha1.DefaultRepairIntensity {
		t.Errorf("RepairIntensity (%s) is not the expected value: %s", instance.Spec.ServerConfig.RepairIntensity, v1alpha1.DefaultRepairIntensity)
	}

	if instance.Spec.ServerConfig.RepairParallelism != v1alpha1.DefaultRepairParallelism {
		t.Errorf("RepairParallelism (%s) is not the expected value: %s", instance.Spec.ServerConfig.RepairParallelism, v1alpha1.DefaultRepairParallelism)
	}

	if *instance.Spec.ServerConfig.RepairRunThreadCount != v1alpha1.DefaultRepairRunThreadCount {
		t.Errorf("RepairRunThreadCount (%d) is not the expected value: %d", *instance.Spec.ServerConfig.RepairRunThreadCount, v1alpha1.DefaultRepairRunThreadCount)
	}

	if *instance.Spec.ServerConfig.ScheduleDaysBetween != v1alpha1.DefaultScheduleDaysBetween {
		t.Errorf("ScheduleDaysBetween (%d) is not the expected value: %d", *instance.Spec.ServerConfig.ScheduleDaysBetween, v1alpha1.DefaultScheduleDaysBetween)
	}

	if instance.Spec.ServerConfig.StorageType != v1alpha1.DefaultStorageType {
		t.Errorf("StorageType (%s) is not the expected value: %s", instance.Spec.ServerConfig.StorageType, v1alpha1.DefaultStorageType)
	}
}