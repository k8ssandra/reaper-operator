package reaper

import (
	"context"
	"github.com/jsanda/reaper-operator/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func createConfigMapReconciler(state ...runtime.Object) *configMapReconciler {
	cl := fake.NewFakeClientWithScheme(s, state...)
	return &configMapReconciler{
		client: cl,
		scheme: scheme.Scheme,
	}
}

func TestReconcileConfigMap(t *testing.T) {
	if err := apis.AddToScheme(scheme.Scheme); err != nil {
		t.FailNow()
	}

	t.Run("ReconcileConfigMapNotFound", testReconcileConfigMapNotFound)
	t.Run("ReconcileConfigMapFound", testReconcileConfigMapFound)
}

func testReconcileConfigMapNotFound(t *testing.T) {
	reaper := createReaper()

	r := createConfigMapReconciler()

	result, err := r.ReconcileConfigMap(context.TODO(), reaper)

	if result == nil {
		t.Errorf("expected non-nil result")
	} else if !result.Requeue {
		t.Errorf("expected requeue")
	}

	if err != nil {
		t.Errorf("did not expect an error but got: (%s)", err)
	}

	cm := &corev1.ConfigMap{}
	if err := r.client.Get(context.TODO(), namespaceName, cm); err != nil {
		t.Errorf("Failed to get ConfigMap: (%s)", err)
	}
}

func testReconcileConfigMapFound(t *testing.T) {
	reaper := createReaper()
	cm := createConfigMap(reaper)

	objs := []runtime.Object{reaper, cm}

	r := createConfigMapReconciler(objs...)

	result, err := r.ReconcileConfigMap(context.TODO(), reaper)

	if result != nil {
		t.Errorf("expected result (nil), got (%v)", result)
	}

	if err != nil {
		t.Errorf("expect error (nil), got (%s)", err)
	}
}
