package reaper

import (
	"context"
	"errors"
	"github.com/thelastpickle/reaper-operator/pkg/apis"
	"github.com/thelastpickle/reaper-operator/pkg/apis/reaper/v1alpha1"
	"github.com/thelastpickle/reaper-operator/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	v1batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	namespaceName   = types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	s = scheme.Scheme
)

const (
	CassandraClusterName = "reaper-test"
	CassandraServiceName = "reaper-test"
	Keyspace             = "reaper-test"
)

type NoOpValidator struct{
	validationError error
	defaultsUpdated bool
}

func (v *NoOpValidator) Validate(reaper *v1alpha1.Reaper) error {
	return v.validationError
}

func (v *NoOpValidator) SetDefaults(reaper *v1alpha1.Reaper) bool {
	return v.defaultsUpdated
}

type NoOpReconciler struct {
	result *reconcile.Result
	err error
}

func (r *NoOpReconciler) ReconcileConfigMap(ctx context.Context, reaper *v1alpha1.Reaper) (*reconcile.Result, error) {
	return r.result, r.err
}

func (r *NoOpReconciler) ReconcileService(ctx context.Context, reaper *v1alpha1.Reaper) (*reconcile.Result, error) {
	return r.result, r.err
}

func (r *NoOpReconciler) ReconcileSchema(ctx context.Context, reaper *v1alpha1.Reaper) (*reconcile.Result, error) {
	return r.result, r.err
}

func (r *NoOpReconciler) ReconcileDeployment(ctx context.Context, reaper *v1alpha1.Reaper) (*reconcile.Result, error) {
	return r.result, r.err
}

func newRequest() reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func createReconciler(state ...runtime.Object) *ReconcileReaper {
	cl := fake.NewFakeClientWithScheme(s, state...)
	noOpReconciler := &NoOpReconciler{result: nil, err: nil}
	return &ReconcileReaper{
		client: cl,
		scheme: scheme.Scheme,
		validator: &NoOpValidator{},
		configMapReconciler: noOpReconciler,
		serviceReconciler: noOpReconciler,
		schemaReconciler: noOpReconciler,
	}
}

func createReconcilerWithValidator(v config.Validator, state ...runtime.Object) *ReconcileReaper {
	cl := fake.NewFakeClientWithScheme(s, state...)
	noOpReconciler := &NoOpReconciler{result: nil, err: nil}
	return &ReconcileReaper{
		client: cl,
		scheme: scheme.Scheme,
		validator: v,
		configMapReconciler: noOpReconciler,
		serviceReconciler: noOpReconciler,
		schemaReconciler: noOpReconciler,
		deploymentReconciler: noOpReconciler,
	}
}

func TestReconcile(t *testing.T) {
	if err := apis.AddToScheme(s); err != nil {
		t.FailNow()
	}

	t.Run("ValidationFails", testValidationFails)
	t.Run("SetDefaults", testSetDefaults)

	// These tests are commented out since they have been moved to reconcile_test.go. I will
	// replace the tests with mocks to verify the correct reconciler calls happen.

	//t.Run("ConfigMapCreated", testConfigMapCreated)
	//t.Run("ServiceCreated", testServiceCreated)
	//t.Run("SchemaJobRun", testSchemaJobCreated)
	//t.Run("DeploymentCreateWhenSchemaJobCompleted", testDeploymentCreatedWhenSchemaJobCompleted)
	//t.Run("DeploymentNotCreatedWhenSchemaJobNotComplete", testDeploymentNotCreatedWhenSchemaJobNotCompleted)
	//t.Run("DeploymentNotCreatedWhenSchemaJobFailed", testDeploymentNotCreatedWhenSchemaJobFailed)
}

func testValidationFails(t *testing.T) {
	v := &NoOpValidator{validationError: errors.New("validation failed")}

	reaper := &v1alpha1.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name: name,
		},
		Spec: v1alpha1.ReaperSpec{
		},
	}

	objs := []runtime.Object{reaper}
	r := createReconcilerWithValidator(v, objs...)
	req := newRequest()

	res, err := r.Reconcile(req)

	if err == nil {
		t.Errorf("expected error to be returned")
	}

	if res.Requeue {
		t.Errorf("do not requeue when there is a validation error")
	}
}

func testSetDefaults(t *testing.T) {
	v := &NoOpValidator{defaultsUpdated: true}

	reaper := &v1alpha1.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name: name,
		},
		Spec: v1alpha1.ReaperSpec{
		},
	}

	objs := []runtime.Object{reaper}
	r := createReconcilerWithValidator(v, objs...)
	req := newRequest()

	res, err := r.Reconcile(req)

	if err != nil {
		t.Errorf("did not expect error to be returned")
	}

	if !res.Requeue {
		t.Errorf("expected requeue when defaults are not set")
	}
}

func testReconcileConfigMapRequeue(t *testing.T) {

}

func testReconcileConfigMapContinue(t *testing.T) {

}

func testConfigMapCreated(t *testing.T) {
	reaper := createReaper()

	objs := []runtime.Object{reaper}

	r := createReconciler(objs...)
	req := newRequest()
	res, err := r.Reconcile(req)

	if err != nil {
		t.Errorf("did not expect Reconcile to return error (%s)", err)
	}

	if !res.Requeue {
		t.Errorf("expected requeue after creating ConfigMap")
	}

	cm := &corev1.ConfigMap{}
	if err := r.client.Get(context.TODO(), namespaceName, cm); err != nil {
		t.Errorf("Failed to get ConfigMap: (%s)", err)
	}

	checkLabelsSet(t, reaper, cm)
}

func testServiceCreated(t *testing.T) {
	reaper := createReaper()
	cm := createConfigMap(reaper)

	objs := []runtime.Object{reaper, cm}

	r := createReconciler(objs...)
	req := newRequest()
	res, err := r.Reconcile(req)

	if err != nil {
		t.Errorf("did not expect Reconcile to return error (%s)", err)
	}

	if !res.Requeue {
		t.Errorf("expected requeue after creating Service")
	}

	svc := &corev1.Service{}
	if err := r.client.Get(context.TODO(), namespaceName, svc); err != nil {
		t.Errorf("Failed to get Service: (%s)", err)
	}

	checkLabelsSet(t, reaper, svc)
}

func testSchemaJobCreated(t *testing.T) {
	reaper := createReaper()
	cm := createConfigMap(reaper)
	svc := createService(reaper)

	objs := []runtime.Object{reaper, cm, svc}

	r := createReconciler(objs...)
	req := newRequest()
	res, err := r.Reconcile(req)

	if err != nil {
		t.Errorf("did not Reconcile to return error (%s)", err)
	}

	if !res.Requeue {
		t.Errorf("expected requeue after creating schema Job")
	}

	job := &v1batch.Job{}
	jobName := getSchemaJobName(reaper)
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: jobName}, job); err != nil {
		t.Errorf("Failed to get job (%s): (%s)", jobName, err)
	}

	checkLabelsSet(t, reaper, job)
}

func testDeploymentCreatedWhenSchemaJobCompleted(t *testing.T) {
	reaper := createReaper()
	cm := createConfigMap(reaper)
	svc := createService(reaper)
	job := createSchemaJobComplete(reaper)

	objs := []runtime.Object{reaper, cm, svc, job}

	r := createReconciler(objs...)
	req := newRequest()
	res, err := r.Reconcile(req)

	if err != nil {
		t.Errorf("did not expect Reconcile to return error (%s)", err)
	}

	if !res.Requeue {
		t.Errorf("Expected requeue after creating Deployment")
	}

	deployment := &appsv1.Deployment{}
	if err := r.client.Get(context.TODO(), namespaceName, deployment); err != nil {
		t.Errorf("Failed to get Deployment: (%s)", err)
	}

	checkLabelsSet(t, reaper, deployment)
}

func testDeploymentNotCreatedWhenSchemaJobNotCompleted(t *testing.T) {
	reaper := createReaper()
	cm := createConfigMap(reaper)
	svc := createService(reaper)
	job := createSchemaJob(reaper)

	objs := []runtime.Object{reaper, cm, svc, job}

	r := createReconciler(objs...)
	req := newRequest()
	res, err := r.Reconcile(req)

	if err != nil {
		t.Errorf("did not expect Reconcile to return error (%s)", err)
	}

	if !res.Requeue {
		t.Errorf("Expected requeue schema job not complete")
	}

	deployment := &appsv1.Deployment{}
	if err := r.client.Get(context.TODO(), namespaceName, deployment); err == nil {
		t.Errorf("Did not expect Deployment to be created")
	}
}

func testDeploymentNotCreatedWhenSchemaJobFailed(t *testing.T) {
	reaper := createReaper()
	cm := createConfigMap(reaper)
	svc := createService(reaper)
	job := createSchemaJobFailed(reaper)

	objs := []runtime.Object{reaper, cm, svc, job}

	r := createReconciler(objs...)
	req := newRequest()
	res, err := r.Reconcile(req)

	if err == nil {
		t.Errorf("expected Reconcile to return an error")
	}

	if res.Requeue {
		t.Errorf("did not expect requeue when schema job fails")
	}

	deployment := &appsv1.Deployment{}
	if err := r.client.Get(context.TODO(), namespaceName, deployment); err == nil {
		t.Errorf("Did not expect Deployment to be created")
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
				EnableCrossOrigin: boolPtr(false),
				StorageType: v1alpha1.Cassandra,
				EnableDynamicSeedList: boolPtr(false),
				JmxConnectionTimeoutInSeconds: int32Ptr(10),
				SegmentCountPerNode: int32Ptr(32),
				CassandraBackend: &v1alpha1.CassandraBackend{
					ClusterName:   CassandraClusterName,
					ContactPoints: []string {CassandraServiceName},
					Keyspace:      Keyspace,
					Replication: v1alpha1.ReplicationConfig{
						SimpleStrategy: int32Ptr(2),
					},
				},
			},
		},
	}
}

func createReaperWithMemoryStorage() *v1alpha1.Reaper {
	r := createReaper()
	r.Spec.ServerConfig.StorageType = v1alpha1.Memory
	r.Spec.ServerConfig.CassandraBackend = nil

	return r
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

func createSchemaJobComplete(reaper *v1alpha1.Reaper) *v1batch.Job {
	completionTime := metav1.Now()
	return &v1batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reaper.Namespace,
			Name: getSchemaJobName(reaper),
		},
		Status: v1batch.JobStatus{
			CompletionTime: &completionTime,
			Conditions: []v1batch.JobCondition{
				{
					Type: v1batch.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func createSchemaJobFailed(reaper *v1alpha1.Reaper) *v1batch.Job {
	return &v1batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reaper.Namespace,
			Name: getSchemaJobName(reaper),
		},
		Status: v1batch.JobStatus{
			Conditions: []v1batch.JobCondition{
				{
					Type: v1batch.JobFailed,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}



func createSchemaJob(reaper *v1alpha1.Reaper) *v1batch.Job {
	return &v1batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reaper.Namespace,
			Name: getSchemaJobName(reaper),
		},
	}
}

func checkLabelsSet(t *testing.T, r *v1alpha1.Reaper, o metav1.Object) {
	labels := o.GetLabels()

	if val, found := labels["app"]; !found {
		t.Errorf("expected to find label app")
	} else if val != "reaper" {
		t.Errorf("expected label app = reaper")
	}

	if val, found := labels["reaper"]; !found {
		t.Errorf("expected to find label reaper")
	} else if val != r.Name {
		t.Errorf("expected label reaper = %s", r.Name)
	}
}