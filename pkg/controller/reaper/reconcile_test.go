package reaper

import (
	"context"
	"github.com/jsanda/reaper-operator/pkg/apis"
	v1batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

func createServiceReconciler(state ...runtime.Object) *serviceReconciler {
	cl := fake.NewFakeClientWithScheme(s, state...)
	return &serviceReconciler{
		client: cl,
		scheme: scheme.Scheme,
	}
}

func createSchemaReconciler(state ...runtime.Object) *schemaReconciler {
	cl := fake.NewFakeClientWithScheme(s, state...)
	return &schemaReconciler{
		client: cl,
		scheme: scheme.Scheme,
	}
}

func TestReconcilers(t *testing.T) {
	if err := apis.AddToScheme(scheme.Scheme); err != nil {
		t.FailNow()
	}

	t.Run("ReconcileConfigMapNotFound", testReconcileConfigMapNotFound)
	t.Run("ReconcileConfigMapFound", testReconcileConfigMapFound)
	t.Run("ReconcileServiceNotFound", testReconcileServiceNotFound)
	t.Run("ReconcileServiceFound", testReconcileServiceFound)
	t.Run("ReconcileMemorySchema", testReconcileMemorySchema)
	t.Run("ReconcileCassandraSchemaJobCreated", testReconcileCassandraSchemaJobCreated)
	t.Run("ReconcileCassandraSchemaJobNotFinished", testReconcileCassandraSchemaJobNotFinished)
	t.Run("ReconcileCassandraSchemaJobCompleted", testReconcileCassandraSchemaJobCompleted)
	t.Run("ReconcileCassandraSchemaJobFailed", testReconcileCassandraSchemaJobFailed)
	t.Run("ReconcileSchemaInvalidStorage", testReconcileSchemaInvalidStorage)
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

func testReconcileServiceNotFound(t *testing.T) {
	reaper := createReaper()

	r := createServiceReconciler()

	result, err := r.ReconcileService(context.TODO(), reaper)

	if result == nil {
		t.Errorf("expected non-nil result")
	} else if !result.Requeue {
		t.Errorf("expected requeue")
	}

	if err != nil {
		t.Errorf("did not expect an error but got: (%s)", err)
	}

	svc := &corev1.Service{}
	if err := r.client.Get(context.TODO(), namespaceName, svc); err != nil {
		t.Errorf("failed to get Service: (%s)", err)
	}
}

func testReconcileServiceFound(t *testing.T) {
	reaper := createReaper()
	svc := createService(reaper)

	objs := []runtime.Object{reaper, svc}

	r := createServiceReconciler(objs...)

	result, err := r.ReconcileService(context.TODO(), reaper)

	if result != nil {
		t.Errorf("expected result (nil), got (%v)", result)
	}

	if err != nil {
		t.Errorf("expected error (nil), got (%s)", err)
	}
}

func testReconcileMemorySchema(t *testing.T) {
	reaper := createReaperWithMemoryStorage()

	r := createSchemaReconciler()

	result, err := r.ReconcileSchema(context.TODO(), reaper)

	if result != nil {
		t.Errorf("expected result (nil), got (%v)", result)
	}

	if err != nil {
		t.Errorf("expected error (nil), got (%s)", err)
	}
}

func testReconcileSchemaInvalidStorage(t *testing.T) {
	reaper := createReaperWithMemoryStorage()
	reaper.Spec.ServerConfig.StorageType = "invalid"

	r := createSchemaReconciler()

	result, err := r.ReconcileSchema(context.TODO(), reaper)

	if result != nil {
		t.Errorf("expected result (nil), got (%v)", result)
	}

	if err == nil {
		t.Errorf("expceted non-nil error")
	}
}

func testReconcileCassandraSchemaJobCreated(t *testing.T) {
	reaper := createReaper()

	r := createSchemaReconciler()

	result, err := r.ReconcileSchema(context.TODO(), reaper)

	if result == nil {
		t.Errorf("expected non-nil result")
	} else if !result.Requeue {
		t.Errorf("expected requeue")
	}

	if err != nil {
		t.Errorf("did not expect an error but got: (%s)", err)
	}

	job := &v1batch.Job{}
	jobName := getSchemaJobName(reaper)
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: jobName}, job); err != nil {
		t.Errorf("Failed to get job (%s): (%s)", jobName, err)
	}
}

func testReconcileCassandraSchemaJobNotFinished(t *testing.T) {
	reaper := createReaper()
	job := createSchemaJob(reaper)

	objs := []runtime.Object{reaper, job}

	r := createSchemaReconciler(objs...)

	result, err := r.ReconcileSchema(context.TODO(), reaper)

	if result == nil {
		t.Errorf("expected result non-nil result")
	} else if !result.Requeue {
		t.Errorf("expected requeue")
	}

	if err != nil {
		t.Errorf("did not expect an error but got: (%s)", err)
	}
}

func testReconcileCassandraSchemaJobCompleted(t *testing.T) {
	reaper := createReaper()
	job := createSchemaJobComplete(reaper)

	objs := []runtime.Object{reaper, job}

	r := createSchemaReconciler(objs...)

	result, err := r.ReconcileSchema(context.TODO(), reaper)

	if result != nil {
		t.Errorf("expected result (nil), got (%v)", result)
	}

	if err != nil {
		t.Errorf("expected error (nil), got (%s)", err)
	}
}

func testReconcileCassandraSchemaJobFailed(t *testing.T) {
	reaper := createReaper()
	job := createSchemaJobFailed(reaper)

	objs := []runtime.Object{reaper, job}

	r := createSchemaReconciler(objs...)

	result, err := r.ReconcileSchema(context.TODO(), reaper)

	if result == nil {
		t.Errorf("expected result non-nil result")
	} else if result.Requeue {
		t.Errorf("did not expect requeue")
	}

	if err == nil {
		t.Errorf("expceted non-nil error")
	}
}