package reaper

import (
	"context"
	"github.com/jsanda/reaper-operator/pkg/config"
	"time"

	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_reaper")

const (
	ReaperImage = "jsanda/reaper-k8s:2.0.2-b6bfb774ccbb"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Reaper Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileReaper{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		validator: config.NewValidator(),
		configMapReconciler: NewConfigMapReconciler(mgr.GetClient(), mgr.GetScheme()),
		serviceReconciler: NewServiceReconciler(mgr.GetClient(), mgr.GetScheme()),
		schemaReconciler: NewSchemaReconciler(mgr.GetClient(), mgr.GetScheme()),
		deploymentReconciler: NewDeploymentReconciler(mgr.GetClient(), mgr.GetScheme()),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("reaper-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Reaper
	err = c.Watch(&source.Kind{Type: &v1alpha1.Reaper{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileReaper implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileReaper{}

// ReconcileReaper reconciles a Reaper object
type ReconcileReaper struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme

	validator config.Validator

	configMapReconciler ReaperConfigMapReconciler

	serviceReconciler ReaperServiceReconciler

	schemaReconciler ReaperSchemaReconciler

	deploymentReconciler ReaperDeploymentReconciler
}

// Reconcile reads that state of the cluster for a Reaper object and makes changes based on the state read
// and what is in the Reaper.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileReaper) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Reaper")

	ctx := context.Background()

	// Fetch the Reaper instance
	instance := &v1alpha1.Reaper{}
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{Requeue: true, RequeueAfter: 30 * time.Second}, err
	}

	instance = instance.DeepCopy()

	if err := r.validator.Validate(instance.Spec.ServerConfig); err != nil {
		return reconcile.Result{}, err
	}

	if r.validator.SetDefaults(&instance.Spec.ServerConfig) {
		if err = r.client.Update(ctx, instance); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	if result, err := r.configMapReconciler.ReconcileConfigMap(ctx, instance); result != nil || err != nil {
		return *result, err
	}

	if result, err := r.serviceReconciler.ReconcileService(ctx, instance); result != nil || err != nil {
		return *result, err
	}

	if result, err := r.schemaReconciler.ReconcileSchema(ctx, instance); result != nil || err != nil {
		return *result, err
	}

	if result, err := r.deploymentReconciler.ReconcileDeployment(ctx, instance); result != nil || err != nil {
		return *result, err
	}

	return reconcile.Result{}, nil
}

func createLabels(instance *v1alpha1.Reaper) map[string]string {
	return map[string]string{
		"app": "reaper",
		"reaper": instance.Name,
	}
}

func int32Ptr(n int32) *int32 {
	return &n
}

func boolPtr(b bool) *bool {
	return &b
}
