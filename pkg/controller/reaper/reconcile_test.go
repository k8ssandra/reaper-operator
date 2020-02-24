package reaper

import (
	"context"
	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
	"github.com/jsanda/reaper-operator/pkg/apis"
	appsv1 "k8s.io/api/apps/v1"
	v1batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func createDeploymentReconciler(state ...runtime.Object) *deploymentReconciler {
	cl := fake.NewFakeClientWithScheme(s, state...)
	return &deploymentReconciler{
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
	t.Run("ReconcileDeploymentNotFound", testReconcileDeploymentNotFound)
	t.Run("ReconcileDeploymentNotReady", testReconcileDeploymentNotReady)
	t.Run("ReconcileDeploymentReady", testReconcileDeploymentReady)
}

func testReconcileConfigMapNotFound(t *testing.T) {
	reaper := createReaper()

	objs := []runtime.Object{reaper}

	r := createConfigMapReconciler(objs...)

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

	if reaper.Status.Configuration == "" {
		t.Error("expected Reaper.Status.Configuration to be updated")
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

func testReconcileDeploymentNotFound(t *testing.T) {
	reaper := createReaper()

	r := createDeploymentReconciler()

	result, err := r.ReconcileDeployment(context.TODO(), reaper)

	if result == nil {
		t.Errorf("expected non-nil result")
	} else if !result.Requeue {
		t.Errorf("expected requeue")
	}

	if err != nil {
		t.Errorf("did not expect an error but got: (%s)", err)
	}

	deployment := &appsv1.Deployment{}
	if err := r.client.Get(context.TODO(), namespaceName, deployment); err != nil {
		t.Errorf("failed to get deployment: (%s)", err)
	}
}

func testReconcileDeploymentNotReady(t *testing.T) {
	reaper := createReaper()
	deployment := createNotReadyDeployment(reaper)

	objs := []runtime.Object{reaper, deployment}

	r := createDeploymentReconciler(objs...)

	result, err := r.ReconcileDeployment(context.TODO(), reaper)

	if result == nil {
		t.Errorf("expected non-nil result")
	} else if !result.Requeue {
		t.Errorf("expected requeue")
	}

	if err != nil {
		t.Errorf("did not expect an error but got: (%s)", err)
	}
}

func testReconcileDeploymentReady(t *testing.T) {
	reaper := createReaper()
	deployment := createReadyDeployment(reaper)

	objs := []runtime.Object{reaper, deployment}

	r := createDeploymentReconciler(objs...)

	result, err := r.ReconcileDeployment(context.TODO(), reaper)

	if result != nil {
		t.Errorf("expected result (nil), got (%v)", result)
	}

	if err != nil {
		t.Errorf("expected err (nil), got (%s)", err)
	}
}

func createReadyDeployment(reaper *v1alpha1.Reaper) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reaper.Namespace,
			Name: reaper.Name,
		},
		Status: appsv1.DeploymentStatus{
			// The operator currently only supports deploying a singe replica, but this will change at some
			// point in the future.
			ReadyReplicas: 1,
			Replicas: 1,
		},
	}
}

func createNotReadyDeployment(reaper *v1alpha1.Reaper) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reaper.Namespace,
			Name: reaper.Name,
		},
		Status: appsv1.DeploymentStatus{
			// The operator currently only supports deploying a singe replica, but this will change at some
			// point in the future.
			ReadyReplicas: 0,
			Replicas: 1,
		},
	}
}