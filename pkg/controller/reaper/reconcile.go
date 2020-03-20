package reaper

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mitchellh/hashstructure"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	v1batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"strings"
	"time"

	"github.com/thelastpickle/reaper-operator/pkg/apis/reaper/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	reapergo "github.com/jsanda/reaper-client-go/reaper"
)

var logger = logf.Log.WithName("reconcile")

const (
	reaperRestartedAt = "cassandra-reaper.io/restartedAt"
)

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

type ReaperDeploymentReconciler interface {
	// Called to reconcile the Reaper deployment. Note that reconciliation will continue after calling this function
	// only when both return values are nil.
	ReconcileDeployment(ctx context.Context, r *v1alpha1.Reaper) (*reconcile.Result, error)
}

type ReaperClustersReconciler interface {
	ReconcileClusters(ctx context.Context, r *v1alpha1.Reaper) (*reconcile.Result, error)
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

type deploymentReconciler struct {
	client client.Client

	scheme *runtime.Scheme
}

type clustersReconciler struct {
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

func NewDeploymentReconciler(c client.Client, s *runtime.Scheme) ReaperDeploymentReconciler {
	return &deploymentReconciler{client: c, scheme: s}
}

func NewClustersReconciler(c client.Client, s *runtime.Scheme) ReaperClustersReconciler {
	return &clustersReconciler{client: c, scheme: s}
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
			if hash, err := r.computeHash(reaper); err != nil {
				reqLogger.Error(err, "failed to compute configuration hash")
				return &reconcile.Result{}, err
			} else {
				reaper.Status.Configuration = hash
				if err = r.client.Status().Update(ctx, reaper); err != nil {
					reqLogger.Error(err, "failed to update status")
					return &reconcile.Result{}, err
				}
			}
			return &reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get ConfigMap")
		return &reconcile.Result{}, err
	}

	// else if server config has changed,
	//
	// 1) update the configmap
	// update status to indicate restart required
	// 2) restart reaper pods
  	hash, err := r.computeHash(reaper)
  	if err != nil {
  		reqLogger.Error(err,"failed to compute configuration hash")
  		return &reconcile.Result{}, err
	} else if reaper.Status.Configuration != hash {
		reqLogger.Info("configuration change detected", "CurrentConfiguration", reaper.Status.Configuration,
			"NewConfiguration", hash)

		if config, err := r.generateConfig(reaper); err != nil {
			reqLogger.Error(err, "failed to generate updated configuration")
			return &reconcile.Result{}, err
		} else {
			serverConfig.Data = map[string]string{
				"reaper.yaml": config,
			}
			if err  = r.client.Update(ctx, serverConfig); err != nil {
				reqLogger.Error(err, "failed to update ConfigMap")
				return &reconcile.Result{}, err
			} else {
				reaper.Status.Configuration = hash
				SetConfigurationUpdatedCondition(&reaper.Status)
				if err = r.client.Status().Update(ctx, reaper); err != nil {
					reqLogger.Error(err, "failed to update status")
					return &reconcile.Result{}, err
				} else {
					return &reconcile.Result{Requeue: true}, nil
				}
			}
		}
	}

	return nil, nil
}

func (r *configMapReconciler) newServerConfigMap(reaper *v1alpha1.Reaper) (*corev1.ConfigMap, error) {
	if config, err := r.generateConfig(reaper); err != nil {
		return nil, err
	} else {
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
				"reaper.yaml": config,
			},
		}

		return cm, nil
	}
}

func (r *configMapReconciler) generateConfig(reaper *v1alpha1.Reaper) (string, error) {
	if out, err := yaml.Marshal(&reaper.Spec.ServerConfig); err == nil {
		return string(out), nil
	} else {
		return "", err
	}
}

func (r *configMapReconciler) computeHash(reaper *v1alpha1.Reaper) (string, error) {
	if hash, err := hashstructure.Hash(reaper.Spec.ServerConfig, nil); err == nil {
		return strconv.FormatUint(hash, 10), nil
	} else {
		return "", err
	}
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
	switch reaper.Spec.ServerConfig.StorageType {
	case v1alpha1.Memory:
		return nil, nil
	case v1alpha1.Cassandra:
		return r.reconcileCassandraSchema(ctx, reaper, reqLogger)
	default:
		return nil, fmt.Errorf("unsupported storage type: (%s)", reaper.Spec.ServerConfig.StorageType)
	}
}

func (r *schemaReconciler) reconcileCassandraSchema(ctx context.Context, reaper *v1alpha1.Reaper, reqLogger logr.Logger) (*reconcile.Result, error) {
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

func (r *deploymentReconciler) ReconcileDeployment(ctx context.Context, reaper *v1alpha1.Reaper) (*reconcile.Result, error) {
	reqLogger := getRequestLogger(reaper)
	reqLogger.Info("Reconciling deployment")
	deployment := &appsv1.Deployment{}
	err := r.client.Get(ctx, types.NamespacedName{Name: reaper.Name, Namespace: reaper.Namespace}, deployment)
	if err != nil && errors.IsNotFound(err) {
		// Create the Deployment
		deployment = r.newDeployment(reaper)
		reqLogger.Info("Creating deployment", "Reaper.Namespace", reaper.Namespace, "Reaper.Name",
			reaper.Name, "Deployment.Name", deployment.Name)
		if err = controllerutil.SetControllerReference(reaper, deployment, r.scheme); err != nil {
			reqLogger.Error(err, "Failed to set owner reference on Reaper Deployment")
			return &reconcile.Result{}, err
		}
		if err = r.client.Create(ctx, deployment); err != nil {
			reqLogger.Error(err, "Failed to create Deployment")
			return &reconcile.Result{}, err
		} else {
			return &reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment")
		return &reconcile.Result{}, err
	}

	status := reaper.Status.DeepCopy()
	CalculateStatus(status, deployment)

	result := &reconcile.Result{}

	if IsReady(status) {
		if IsRestartNeeded(status) {
			reqLogger.Info("restarting deployment", "Deployment.Name", deployment.Name)
			if err = r.restart(ctx, deployment); err != nil {
				reqLogger.Error(err, "There was an error while initiating a restart")
			}
			result = &reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}
		} else {
			result = nil
			err = nil
		}
	} else {
		result = &reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}
		err = nil
	}

	newReaper := reaper
	newReaper.Status = *status
	if err = r.client.Status().Update(ctx, newReaper); err != nil {
		reqLogger.Error(err, "failed to update status")
		result = &reconcile.Result{Requeue: true}
	}

	return result, err
}

func (r *deploymentReconciler) newDeployment(reaper *v1alpha1.Reaper) *appsv1.Deployment {
	var initialDelay int32
	if reaper.Spec.ServerConfig.StorageType == v1alpha1.Memory {
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
				Values: []string{reaper.Name},
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
			Name:      reaper.Name,
			Namespace: reaper.Namespace,
			Labels:    createLabels(reaper),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: createLabels(reaper),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "reaper",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Image: ReaperImage,
							Resources: reaper.Spec.DeploymentConfiguration.Resources,
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
									ReadOnly: true,
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
										Name: reaper.Name,
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
					Affinity: reaper.Spec.DeploymentConfiguration.Affinity,
				},
			},
		},
	}
}

func (r *deploymentReconciler) restart(ctx context.Context, deployment *appsv1.Deployment) error {
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations[reaperRestartedAt] = time.Now().Format(time.RFC3339)
	return r.client.Update(ctx, deployment)
}

func (r *clustersReconciler) ReconcileClusters(ctx context.Context, reaper *v1alpha1.Reaper) (*reconcile.Result, error) {
	reqLogger := getRequestLogger(reaper)
	reqLogger.Info("Reconciling clusters")

	status := reaper.Status.DeepCopy()
	var err error
	var result reconcile.Result

	if cluster := getNextClusterToAdd(reaper); cluster == nil {
		// TODO check for deletions
	} else {
		reqLogger.Info("adding cluster to Reaper!!!!", "CassandraCluster", cluster)

		restClient, err := reapergo.NewClient(fmt.Sprintf("http://%s.%s:8080", reaper.Name, reaper.Namespace))
		if err != nil {
			// There was a problem creating the REST client, so we are done
			reqLogger.Error(err,"failed to create REST client")
			result = reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}
		} else {
			// add the cluster using the REST client
			seed := fmt.Sprintf("%s.%s", cluster.Service.Name, cluster.Service.Namespace)
			if err := restClient.AddCluster(ctx, cluster.Name, seed); err == nil {
				// The cluster was successfully added. Add it to the status. We will requeue until there are no more
				// clusters to add/delete.
				status.Clusters = append(status.Clusters, *cluster)
				result = reconcile.Result{Requeue: true}
			} else {
				// Adding the cluster failed, so we need to requeue and try again.
				reqLogger.Error(err, "failed to add cluster to Reaper", "CassandraCluster", cluster)
				result = reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}
			}
		}
	}

	newReaper := reaper
	newReaper.Status = *status
	if err = r.client.Status().Update(ctx, newReaper); err != nil {
		reqLogger.Error(err, "failed to update status")
		result = reconcile.Result{Requeue: true}
	}

	return &result, err
}

func getNextClusterToAdd(reaper *v1alpha1.Reaper) *v1alpha1.CassandraCluster {
	for _, cc := range reaper.Spec.Clusters {
		if !statusContainsCluster(&reaper.Status, &cc) {
			return &cc
		}
	}
	return nil
}

func statusContainsCluster(status *v1alpha1.ReaperStatus, cluster *v1alpha1.CassandraCluster) bool {
	for _, cc := range status.Clusters {
		if cc == *cluster {
			return true
		}
	}
	return false
}

func getRequestLogger(reaper *v1alpha1.Reaper) logr.Logger {
	return log.WithValues("Reaper.Namespace", reaper.Namespace, "Reaper.Name", reaper.Name)
}