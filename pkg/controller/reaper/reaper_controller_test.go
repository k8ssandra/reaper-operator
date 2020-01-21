package reaper

import (
	"context"
	"github.com/jsanda/reaper-operator/pkg/apis"
	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

var (
	name            = "reaper-operator"
	namespace       = "reaper"
	namespaceName = types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
)

func setupReconcile(t *testing.T, state ...runtime.Object) (*ReconcileReaper, reconcile.Result) {
	s := scheme.Scheme
	if err := apis.AddToScheme(s); err != nil {
		t.FailNow()
	}
	cl := fake.NewFakeClientWithScheme(s, state...)
	r := &ReconcileReaper{client: cl, scheme: scheme.Scheme}
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

	return r, res
}

func setupReconcileWithRequeue(t *testing.T, state ...runtime.Object) *ReconcileReaper {
	r, res := setupReconcile(t, state...)

	// Check the result of reconciliation to make sure it has the desired state.
	if !res.Requeue {
		t.Error("reconcile did not requeue request as expected")
	}

	return r
}

func setupReconcileWithoutRequeue(t *testing.T, state ...runtime.Object) *ReconcileReaper {
	r, res := setupReconcile(t, state...)

	if res.Requeue {
		t.Error("did not expect reconcile to requeue the request")
	}

	return r
}

func TestReconcile(t *testing.T) {
	t.Run("SetDefaults", testSetDefaults)
	t.Run("ConfigMapCreated", testConfigMapCreated)
	t.Run("ServiceCreated", testServiceCreated)
	t.Run("DeploymentCreated", testDeploymentCreated)
}

func testSetDefaults(t *testing.T) {
	reaper := &v1alpha1.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name: name,
		},
		Spec: v1alpha1.ReaperSpec{
		},
	}

	objs := []runtime.Object{reaper}

	r := setupReconcileWithRequeue(t, objs...)

	// verify default values set
	instance := &v1alpha1.Reaper{}
	if err := r.client.Get(context.TODO(), namespaceName, instance); err != nil {
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

func testConfigMapCreated(t *testing.T) {
	reaper := createReaper()

	objs := []runtime.Object{reaper}

	r := setupReconcileWithRequeue(t, objs...)

	cm := &corev1.ConfigMap{}
	if err := r.client.Get(context.TODO(), namespaceName, cm); err != nil {
		t.Errorf("Failed to get ConfigMap: (%s)", err)
	}
}

func testServiceCreated(t *testing.T) {
	reaper := createReaper()
	cm := createConfigMap(reaper)

	objs := []runtime.Object{reaper, cm}

	r := setupReconcileWithRequeue(t, objs...)

	svc := &corev1.Service{}
	if err := r.client.Get(context.TODO(), namespaceName, svc); err != nil {
		t.Errorf("Failed to get Service: (%s)", err)
	}
}

func testDeploymentCreated(t *testing.T) {
	reaper := createReaper()
	cm := createConfigMap(reaper)
	svc := createService(reaper)

	objs := []runtime.Object{reaper, cm, svc}

	r := setupReconcileWithRequeue(t, objs...)

	deployment := &appsv1.Deployment{}
	if err := r.client.Get(context.TODO(), namespaceName, deployment); err != nil {
		t.Errorf("Failed to get Deployment: (%s)", err)
	}
}

func createReaper() *v1alpha1.Reaper {
	return &v1alpha1.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name: name,
		},
		Spec: v1alpha1.ReaperSpec{
			ServerConfig: v1alpha1.ServerConfig{
				HangingRepairTimeoutMins: int32Ptr(60),
				RepairIntensity: "0.75",
				RepairParallelism: "DATACENTER_AWARE",
				RepairRunThreadCount: int32Ptr(20),
				ScheduleDaysBetween: int32Ptr(10),
				StorageType: v1alpha1.DefaultStorageType,
			},
		},
	}
}

func createConfigMap(reaper *v1alpha1.Reaper) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reaper.Namespace,
			Name: reaper.Name,
		},
	}
}

func createService(reaper *v1alpha1.Reaper) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reaper.Namespace,
			Name: reaper.Name,
		},
	}
}