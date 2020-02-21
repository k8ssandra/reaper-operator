package reaper

import (
	"context"
	"fmt"
	"github.com/jsanda/reaper-operator/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	"strconv"
	"strings"
	"time"

	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

	if len(instance.Status.Conditions) == 0  {
		if result, err := r.configMapReconciler.ReconcileConfigMap(ctx, instance); result != nil || err != nil {
			return *result, err
		}

		if result, err := r.serviceReconciler.ReconcileService(ctx, instance); result != nil || err != nil {
			return *result, err
		}

		if result, err := r.schemaReconciler.ReconcileSchema(ctx, instance); result != nil || err != nil {
			return *result, err
		}

		reqLogger.Info("Reconciling deployment")
		deployment := &appsv1.Deployment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, deployment)
		if err != nil && errors.IsNotFound(err) {
			// Create the Deployment
			deployment = r.newDeployment(instance)
			reqLogger.Info("Creating deployment", "Reaper.Namespace", instance.Namespace, "Reaper.Name",
				instance.Name, "Deployment.Name", deployment.Name)
			if err = controllerutil.SetControllerReference(instance, deployment, r.scheme); err != nil {
				reqLogger.Error(err, "Failed to set owner reference on Reaper Deployment")
				return reconcile.Result{}, err
			}
			if err = r.client.Create(context.TODO(), deployment); err != nil {
				reqLogger.Error(err, "Failed to create Deployment")
				return reconcile.Result{}, err
			} else {
				return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
			}
		} else if err != nil {
			reqLogger.Error(err, "Failed to get Deployment")
			return reconcile.Result{}, err
		}

		if updateStatus(instance, deployment) {
			if err = r.client.Status().Update(context.TODO(), instance); err != nil {
				reqLogger.Error(err, "Failed to update status")
				return reconcile.Result{}, err
			}
		}

		if instance.Status.ReadyReplicas != instance.Status.Replicas {
			return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
	}

	return reconcile.Result{}, nil
}

func updateStatus(instance *v1alpha1.Reaper, deployment *appsv1.Deployment) bool {
	updated := false

	if instance.Status.AvailableReplicas != deployment.Status.AvailableReplicas {
		instance.Status.AvailableReplicas = deployment.Status.AvailableReplicas
		updated = true
	}

	if instance.Status.ReadyReplicas != deployment.Status.ReadyReplicas {
		instance.Status.ReadyReplicas = deployment.Status.ReadyReplicas
		updated = true
	}

	if instance.Status.Replicas != deployment.Status.Replicas {
		instance.Status.Replicas = deployment.Status.Replicas
		updated = true
	}

	if instance.Status.UpdatedReplicas != deployment.Status.UpdatedReplicas {
		instance.Status.UpdatedReplicas = deployment.Status.UpdatedReplicas
		updated = true
	}

	return updated
}

func convert(r v1alpha1.ReplicationConfig) string {
	if r.SimpleStrategy != nil {
		replicationFactor := strconv.FormatInt(int64(*r.SimpleStrategy), 10)
		return fmt.Sprintf(`{'class': 'SimpleStrategy', 'replication_factor': %s}`, replicationFactor)
	} else {
		var sb strings.Builder
		dcs := make([]string, 0)
		for k, v := range *r.NetworkTopologyStrategy {
			sb.WriteString("'")
			sb.WriteString(k)
			sb.WriteString("': ")
			sb.WriteString(strconv.FormatInt(int64(v), 10))
			dcs = append(dcs, sb.String())
			sb.Reset()
		}
		return fmt.Sprintf("{'class': 'NetworkTopologyStrategy', %s}", strings.Join(dcs, ", "))
	}
}

func (r *ReconcileReaper) newDeployment(instance *v1alpha1.Reaper) *appsv1.Deployment {
	var initialDelay int32
	if instance.Spec.ServerConfig.StorageType == v1alpha1.Memory {
		initialDelay = 10
	} else {
		initialDelay = 60
	}

	selector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key: "app",
				Operator: metav1.LabelSelectorOpIn,
				Values: []string{"reaper"},
			},
			{
				Key: "reaper",
				Operator: metav1.LabelSelectorOpIn,
				Values: []string{instance.Name},
			},
		},
	}

	healthProbe := &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthcheck",
				Port: intstr.FromInt(8081),
			},
		},
		InitialDelaySeconds: initialDelay,
		PeriodSeconds: 3,
	}

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Name,
			Namespace: instance.Namespace,
			Labels: createLabels(instance),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: createLabels(instance),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "reaper",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Image: ReaperImage,
							Ports: []corev1.ContainerPort{
								{
									Name: "ui",
									ContainerPort: 8080,
									Protocol: "TCP",
								},
								{
									Name: "admin",
									ContainerPort: 8081,
									Protocol: "TCP",
								},
							},
							LivenessProbe: healthProbe,
							ReadinessProbe: healthProbe,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name: "reaper-config",
									MountPath: "/etc/cassandra-reaper",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "reaper-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: instance.Name,
									},
									Items: []corev1.KeyToPath{
										{
											Key: "reaper.yaml",
											Path: "cassandra-reaper.yaml",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
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
