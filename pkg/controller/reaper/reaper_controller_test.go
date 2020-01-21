package reaper

import (
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
}