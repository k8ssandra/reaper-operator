package reaper

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
	v1batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"

	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var logger = logf.Log.WithName("reconcile")

type ReaperConfigMapReconciler interface {
	// Called to reconcile the Reaper ConfigMap. Note that reconciliation will continue after calling this function
	// only when both return values are nil.
	ReconcileConfigMap(ctx context.Context, r *v1alpha1.Reaper) (*reconcile.Result, error)
}

type ReaperServiceReconciler interface {
	// Called to reconcile the Reaper service. Note that reconciliation will continue after calling this function only
	//when both return values are nil.
	ReconcileService(ctx context.Context, r *v1alpha1.Reaper) (*reconcile.Result, error)
}

type ReaperSchemaReconciler interface {
	// Called to reconcile schema initialization for Reaper backend. Note that reconciliation will continue after
	// calling this function only when both return values are nil.
	ReconcileSchema(ctx context.Context, r *v1alpha1.Reaper) (*reconcile.Result, error)
}

type configMapReconciler struct {
	client client.Client

	scheme *runtime.Scheme
}

type serviceReconciler struct {
	client client.Client

	scheme *runtime.Scheme
}

type schemaReconciler struct {
	client client.Client

	scheme *runtime.Scheme
}

func NewConfigMapReconciler(c client.Client, s *runtime.Scheme) ReaperConfigMapReconciler {
	return &configMapReconciler{client: c, scheme: s}
}

func NewServiceReconciler(c client.Client, s *runtime.Scheme) ReaperServiceReconciler {
	return &serviceReconciler{client: c, scheme: s}
}

func NewSchemaReconciler(c client.Client, s *runtime.Scheme) ReaperSchemaReconciler {
	return &schemaReconciler{client: c, scheme: s}
}

func (r *configMapReconciler) ReconcileConfigMap(ctx context.Context, reaper *v1alpha1.Reaper) (*reconcile.Result, error) {
	reqLogger := getRequestLogger(reaper)
	reqLogger.Info("Reconciling configmap")
	serverConfig := &corev1.ConfigMap{}
	err := r.client.Get(ctx, types.NamespacedName{Name: reaper.Name, Namespace: reaper.Namespace}, serverConfig)
	if err != nil && errors.IsNotFound(err) {
		// create server config configmap
		cm, err := r.newServerConfigMap(reaper)
		if err != nil {
			reqLogger.Error(err, "Failed to create new ConfigMap")
			return &reconcile.Result{}, err
		}
		reqLogger.Info("Creating configmap", "ConfigMap.Name", cm.Name)
		if err = controllerutil.SetControllerReference(reaper, cm, r.scheme); err != nil {
			reqLogger.Error(err, "Failed to set owner reference on Reaper server config ConfigMap")
			return &reconcile.Result{}, err
		}
		if err = r.client.Create(ctx, cm); err != nil {
			reqLogger.Error(err, "Failed to save ConfigMap")
			return &reconcile.Result{}, err
		} else {
			return &reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get ConfigMap")
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *configMapReconciler) newServerConfigMap(reaper *v1alpha1.Reaper) (*corev1.ConfigMap, error) {
	output, err := yaml.Marshal(&reaper.Spec.ServerConfig)
	if err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      reaper.Name,
			Namespace: reaper.Namespace,
			Labels:    createLabels(reaper),
		},
		Data: map[string]string{
			"reaper.yaml": string(output),
		},
	}

	return cm, nil
}

func (r *serviceReconciler) ReconcileService(ctx context.Context, reaper *v1alpha1.Reaper) (*reconcile.Result, error) {
	reqLogger := getRequestLogger(reaper)
	reqLogger.Info("Reconciling service")
	service := &corev1.Service{}
	err := r.client.Get(ctx,types.NamespacedName{Name: reaper.Name, Namespace: reaper.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		// Create the Service
		service = r.newService(reaper)
		reqLogger.Info("Creating service", "Service.Name", service.Name)
		if err = controllerutil.SetControllerReference(reaper, service, r.scheme); err != nil {
			reqLogger.Error(err, "Failed to set owner reference on Reaper Service")
			return &reconcile.Result{}, err
		}
		if err = r.client.Create(ctx, service); err != nil {
			reqLogger.Error(err, "Failed to create Service")
			return &reconcile.Result{}, err
		} else {
			return &reconcile.Result{Requeue: true}, nil
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service")
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *serviceReconciler) newService(reaper *v1alpha1.Reaper) *corev1.Service {
	labels := createLabels(reaper)

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind: "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      reaper.Name,
			Namespace: reaper.Namespace,
			Labels:    labels,
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
			Selector: labels,
		},
	}
}

func (r *schemaReconciler) ReconcileSchema(ctx context.Context, reaper *v1alpha1.Reaper) (*reconcile.Result, error) {
	reqLogger := getRequestLogger(reaper)
	if reaper.Spec.ServerConfig.StorageType == v1alpha1.Cassandra {
		reqLogger.Info("Reconciling schema job")
		schemaJob := &v1batch.Job{}
		jobName := getSchemaJobName(reaper)
		err := r.client.Get(ctx, types.NamespacedName{Namespace: reaper.Namespace, Name: jobName}, schemaJob)
		if err != nil && errors.IsNotFound(err) {
			// Create the job
			schemaJob = r.newSchemaJob(reaper)
			reqLogger.Info("Creating schema job", "Reaper.Namespace", reaper.Namespace, "Reaper.Name",
				reaper.Name, "Job.Name", schemaJob.Name)
			if err = controllerutil.SetControllerReference(reaper, schemaJob, r.scheme); err != nil {
				reqLogger.Error(err, "Failed to set owner reference", "SchemaJob", jobName)
				return &reconcile.Result{}, err
			}
			if err = r.client.Create(context.TODO(), schemaJob); err != nil {
				reqLogger.Error(err, "Failed to create schema Job")
				return &reconcile.Result{}, err
			} else {
				return &reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
			}
		} else if err != nil {
			reqLogger.Error(err, "Failed to get schema Job")
			return &reconcile.Result{}, err
		} else if !jobFinished(schemaJob) {
			return &reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		} else if failed, err := jobFailed(schemaJob); failed {
			return &reconcile.Result{}, err
		} else {
			// the job completed successfully
			return nil, nil
		}
	}
	return nil, fmt.Errorf("unsupported storage type: (%s)", reaper.Spec.ServerConfig.StorageType)
}

func getSchemaJobName(r *v1alpha1.Reaper) string {
	return fmt.Sprintf("%s-schema", r.Name)
}

func (r *schemaReconciler) newSchemaJob(reaper *v1alpha1.Reaper) *v1batch.Job {
	cassandra := *reaper.Spec.ServerConfig.CassandraBackend
	return &v1batch.Job{
		TypeMeta: metav1.TypeMeta{
			Kind: "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reaper.Namespace,
			Name: getSchemaJobName(reaper),
			Labels: createLabels(reaper),
		},
		Spec: v1batch.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name: getSchemaJobName(reaper),
							Image: "jsanda/create_keyspace:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name: "KEYSPACE",
									Value: cassandra.Keyspace,
								},
								{
									Name: "CONTACT_POINTS",
									Value: strings.Join(cassandra.ContactPoints, ","),
								},
								// TODO Add replication_factor. There is already a function in tlp-stress-operator
								//      that does the serialization. I need to move that function to a shared lib.
								{
									Name: "REPLICATION",
									Value: convert(cassandra.Replication),
								},
							},
						},
					},
				},
			},
		},
	}
}

func jobFinished(job *v1batch.Job) bool {
	for _, c := range job.Status.Conditions {
		if (c.Type == v1batch.JobComplete || c.Type == v1batch.JobFailed) && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func jobFailed(job *v1batch.Job) (bool, error) {
	for _, cond := range job.Status.Conditions {
		if cond.Type == v1batch.JobFailed && cond.Status == corev1.ConditionTrue {
			return true, fmt.Errorf("schema job failed: %s", cond.Message)
		}
	}
	return false, nil
}

func getRequestLogger(reaper *v1alpha1.Reaper) logr.Logger {
	return log.WithValues("Reaper.Namespace", reaper.Namespace, "Reaper.Name", reaper.Name)
}