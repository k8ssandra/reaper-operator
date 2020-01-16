package reaper

import (
	"context"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"

	reaperv1alpha1 "github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
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
	return &ReconcileReaper{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("reaper-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Reaper
	err = c.Watch(&source.Kind{Type: &reaperv1alpha1.Reaper{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Reaper
	//err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
	//	IsController: true,
	//	OwnerType:    &reaperv1alpha1.Reaper{},
	//})
	//if err != nil {
	//	return err
	//}

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

	// Fetch the Reaper instance
	instance := &reaperv1alpha1.Reaper{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	instance = instance.DeepCopy()

	if checkDefaults(instance) {
		if err = r.client.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	serverConfig := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, serverConfig)
	if err != nil && errors.IsNotFound(err) {
		// create server config configmap
		cm, err := r.newServerConfigMap(instance)
		if err != nil {
			reqLogger.Error(err, "Failed to create new ConfigMap")
			return reconcile.Result{}, err
		}
		if err = controllerutil.SetControllerReference(instance, cm, r.scheme); err != nil {
			reqLogger.Error(err, "Failed to set owner reference on Reaper server config ConfigMap")
			return reconcile.Result{}, err
		}
		if err = r.client.Create(context.TODO(), cm); err != nil {
			reqLogger.Error(err, "Failed to save ConfigMap")
			return reconcile.Result{}, err
		} else {
			return reconcile.Result{Requeue: true}, nil
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get ConfigMap")
		return reconcile.Result{}, err
	}

	deployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, deployment)
	if err != nil && errors.IsNotFound(err) {
		// Create the Deployment
		deployment := r.newDeployment(instance)
		if err = controllerutil.SetControllerReference(instance, deployment, r.scheme); err != nil {
			reqLogger.Error(err, "Failed to set owner reference on Reaper Deployment")
			return reconcile.Result{}, err
		}
		if err = r.client.Create(context.TODO(), deployment); err != nil {
			reqLogger.Error(err, "Failed to create Deployment")
			return reconcile.Result{}, err
		} else {
			return reconcile.Result{Requeue: true}, nil
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment")
		return reconcile.Result{}, err
	}

	service := &corev1.Service{}
	err = r.client.Get(context.TODO(),types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		// Create the Service
		service := r.newService(instance)
		if err = r.client.Create(context.TODO(), service); err != nil {
			reqLogger.Error(err, "Failed to create Service")
			return reconcile.Result{}, err
		} else {
			return reconcile.Result{Requeue: true}, nil
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileReaper) newServerConfigMap(instance *reaperv1alpha1.Reaper) (*corev1.ConfigMap, error) {
	output, err := yaml.Marshal(&instance.Spec.ServerConfig)
	if err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Name,
			Namespace: instance.Namespace,
		},
		Data: map[string]string{
			"reaper.yaml": string(output),
		},
	}

	return cm, nil
}

func checkDefaults(instance *reaperv1alpha1.Reaper) bool {
	updated := false

	if instance.Spec.ServerConfig.HangingRepairTimeoutMins == nil {
		instance.Spec.ServerConfig.HangingRepairTimeoutMins = int32Ptr(30)
		updated = true
	}

	if instance.Spec.ServerConfig.RepairIntensity == "" {
		instance.Spec.ServerConfig.RepairIntensity = "0.9"
		updated = true
	}

	if instance.Spec.ServerConfig.RepairParallelism == "" {
		instance.Spec.ServerConfig.RepairParallelism = "DATACENTER_AWARE"
		updated = true
	}

	if instance.Spec.ServerConfig.RepairRunThreadCount == nil {
		instance.Spec.ServerConfig.RepairRunThreadCount = int32Ptr(15)
		updated = true
	}

	if instance.Spec.ServerConfig.ScheduleDaysBetween == nil {
		instance.Spec.ServerConfig.ScheduleDaysBetween = int32Ptr(7)
		updated = true
	}

	return updated
}

func (r *ReconcileReaper) newService(instance *reaperv1alpha1.Reaper) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind: "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 8080,
					Name: "ui",
					Protocol: corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type: intstr.String,
						StrVal: "ui",
					},
				},
			},
			Selector: createLabels(instance),
		},
	}
}

func (r *ReconcileReaper) newDeployment(instance *reaperv1alpha1.Reaper) *appsv1.Deployment {
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
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Name,
			Namespace: instance.Namespace,
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
							ImagePullPolicy: corev1.PullAlways,
							Image: "jsanda/cassandra-reaper-k8s:latest",
							Ports: []corev1.ContainerPort{
								{
									Name: "ui",
									ContainerPort: 8080,
									Protocol: "TCP",
								},
							},
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

func createLabels(instance *reaperv1alpha1.Reaper) map[string]string {
	return map[string]string{
		"app": "reaper",
		"reaper": instance.Name,
	}
}

func int32Ptr(n int32) *int32 {
	return &n
}
