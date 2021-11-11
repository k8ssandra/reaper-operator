package reconcile

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/httphelper"
	api "github.com/k8ssandra/reaper-operator/api/v1alpha1"
	"github.com/k8ssandra/reaper-operator/pkg/config"
	mlabels "github.com/k8ssandra/reaper-operator/pkg/labels"
	"github.com/k8ssandra/reaper-operator/pkg/status"
	"github.com/k8ssandra/reaper-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	envVarEnableCassAuth = "REAPER_CASS_AUTH_ENABLED"
)

// ReaperRequest containers the information necessary to perform reconciliation actions on a Reaper object.
type ReaperRequest struct {
	Reaper *api.Reaper

	Logger logr.Logger

	StatusManager *status.StatusManager
}

type ServiceReconciler interface {
	ReconcileService(ctx context.Context, req ReaperRequest) (*ctrl.Result, error)
}

type SchemaReconciler interface {
	ReconcileSchema(ctx context.Context, req ReaperRequest) (*ctrl.Result, error)
}

type DeploymentReconciler interface {
	ReconcileDeployment(ctx context.Context, req ReaperRequest) (*ctrl.Result, error)
}

type defaultReconciler struct {
	client.Client

	scheme *runtime.Scheme

	secretsManager SecretsManager
}

var reconciler defaultReconciler

func InitReconcilers(client client.Client, scheme *runtime.Scheme) {
	reconciler = defaultReconciler{
		Client:         client,
		scheme:         scheme,
		secretsManager: NewSecretsManager(),
	}
}

func GetServiceReconciler() ServiceReconciler {
	return &reconciler
}

func GetSchemaReconciler() SchemaReconciler {
	return &reconciler
}

func GetDeploymentReconciler() DeploymentReconciler {
	return &reconciler
}

func (r *defaultReconciler) ReconcileService(ctx context.Context, req ReaperRequest) (*ctrl.Result, error) {
	reaper := req.Reaper
	key := types.NamespacedName{Namespace: reaper.Namespace, Name: GetServiceName(reaper.Name)}

	req.Logger.Info("reconciling service", "service", key)

	service := &corev1.Service{}
	err := r.Client.Get(ctx, key, service)
	if err != nil && errors.IsNotFound(err) {
		// create the service
		service = newService(key, reaper)
		if err = controllerutil.SetControllerReference(reaper, service, r.scheme); err != nil {
			req.Logger.Error(err, "failed to set owner reference on service", "service", key)
			return &ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
		}

		req.Logger.Info("creating service", "service", key)
		if err = r.Client.Create(ctx, service); err != nil {
			req.Logger.Error(err, "failed to create service", "service", key)
			return &ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
		}

		return nil, nil
	} else if err != nil {
		req.Logger.Error(err, "failed to get service", "service", key)
		return &ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}

	return nil, nil
}

func GetServiceName(reaperName string) string {
	return reaperName + "-reaper-service"
}

func newService(key types.NamespacedName, reaper *api.Reaper) *corev1.Service {
	labels := createLabels(reaper)

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:     8080,
					Name:     "app",
					Protocol: corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "app",
					},
				},
			},
			Selector: labels,
		},
	}
}

func (r *defaultReconciler) ReconcileSchema(ctx context.Context, req ReaperRequest) (*ctrl.Result, error) {
	reaper := req.Reaper
	if reaper.Spec.ServerConfig.StorageType == api.StorageTypeMemory {
		// No need to run schema job when using in-memory backend
		return nil, nil
	}

	// Check that Cassandra is ready before creating the schema job if running under k8ssandra
	ready, err := r.checkForCassandraDatacenterReadiness(ctx, req)
	if err != nil {
		req.Logger.Error(err, "failed to check for CassandraDatacenter readiness")
		return &ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
	}

	if !ready {
		return &ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
	}

	return r.createSchema(ctx, req)
}

func (r *defaultReconciler) checkForCassandraDatacenterReadiness(ctx context.Context, req ReaperRequest) (bool, error) {
	reaper := req.Reaper
	cassdc, err := r.cassandraDatacenter(ctx, reaper)
	if err != nil {
		if errors.IsNotFound(err) {
			// We're unable to track the progress - let the process try
			return true, nil
		}
		req.Logger.Error(err, "failed to fetch CassandraDatacenter")
		return false, err
	}

	if isCassdcReady(cassdc) {
		return true, nil
	}

	return false, nil
}

func (r *defaultReconciler) cassandraDatacenter(ctx context.Context, reaper *api.Reaper) (*cassdcapi.CassandraDatacenter, error) {
	cassandra := *reaper.Spec.ServerConfig.CassandraBackend
	cassdc := &cassdcapi.CassandraDatacenter{}

	targetDc := cassandra.CassandraDatacenter
	if targetDc.Namespace == "" {
		targetDc.Namespace = reaper.Namespace
	}

	cassdcKey := types.NamespacedName{Namespace: targetDc.Namespace, Name: targetDc.Name}
	err := r.Client.Get(ctx, cassdcKey, cassdc)
	return cassdc, err
}

func isCassdcReady(cassdc *cassdcapi.CassandraDatacenter) bool {
	if cassdc.Status.CassandraOperatorProgress != cassdcapi.ProgressReady {
		return false
	}
	cassStatus := cassdc.GetConditionStatus(cassdcapi.DatacenterReady)
	return cassStatus == corev1.ConditionTrue
}

func (r *defaultReconciler) createSchema(ctx context.Context, req ReaperRequest) (*ctrl.Result, error) {
	reaper := req.Reaper
	cassdc, err := r.cassandraDatacenter(ctx, reaper)
	if err != nil {
		req.Logger.Error(err, "failed to fetch target CassandraDatacenter")
		return &ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}

	httpClient, err := httphelper.BuildManagementApiHttpClient(cassdc, r.Client, ctx)
	if err != nil {
		req.Logger.Error(err, "error in BuildManagementApiHttpClient")
		return &ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}

	protocol, err := httphelper.GetManagementApiProtocol(cassdc)
	if err != nil {
		req.Logger.Error(err, "error in GetManagementApiProtocol")
		return &ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}

	nodeMgmtClient := httphelper.NodeMgmtClient{
		Client:   httpClient,
		Log:      req.Logger,
		Protocol: protocol,
	}

	cassandra := *reaper.Spec.ServerConfig.CassandraBackend
	replicationConfig := config.ReplicationToConfig(cassdc.Name, cassandra.Replication)

	pods, err := r.getCassandraDatacenterPods(ctx, cassdc)
	if err != nil {
		req.Logger.Error(err, "failed to get CassandraDatacenter pods")
		return &ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}

	if len(pods) < 1 {
		req.Logger.Error(err, "not enough CassandraDatacenter pods")
		return &ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}

	err = nodeMgmtClient.CreateKeyspace(&pods[0], cassandra.Keyspace, replicationConfig)
	if err != nil {
		req.Logger.Error(err, "failed to create keyspace")
		return &ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}
	return nil, nil
}

func (r *defaultReconciler) ReconcileDeployment(ctx context.Context, req ReaperRequest) (*ctrl.Result, error) {
	reaper := req.Reaper
	key := types.NamespacedName{Namespace: reaper.Namespace, Name: reaper.Name}

	req.Logger.Info("reconciling deployment", "deployment", key)

	deployment := &appsv1.Deployment{}
	desiredDeployment, err := r.buildNewDeployment(req)
	if err != nil {
		req.Logger.Error(err, "failed to build deployment", "deployment", key)
		return &ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	err = r.Get(ctx, key, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			if err = controllerutil.SetControllerReference(reaper, desiredDeployment, r.scheme); err != nil {
				req.Logger.Error(err, "failed to set owner on deployment", "deployment", key)
				return &ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
			}

			if err = r.Create(ctx, desiredDeployment); err != nil {
				req.Logger.Error(err, "failed to create deployment", "deployment", key)
			}
			return &ctrl.Result{RequeueAfter: 10 * time.Second}, err
		} else {
			req.Logger.Error(err, "failed to get deployment", "deployment", key)
			return &ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
	} else {
		if deployment.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType {
			// When strategy is not set to RecreateDeploymentStrategyType this can cause [issues](https://github.com/k8ssandra/reaper-operator/issues/63) where multiple Reaper pods exist concurrently.
			// This is not supported due to the way that migrations in Reaper work. We need to ensure that RecreateDeploymentStrategyType is in place before any upgrades can occur.
			req.Logger.Info("Reaper deployment not currently using appsv1.RecreateDeploymentStrategyType, patching and re-queueing the request")
			patch := client.StrategicMergeFrom(deployment.DeepCopy())
			deployment.Spec.Strategy = appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			}
			if err := r.Patch(ctx, deployment, patch); err != nil {
				req.Logger.Error(err, "Failed to patch deployment with appsv1.RecreateDeploymentStrategyType")
				return &ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}
			return &ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
		if !util.ResourcesHaveSameHash(desiredDeployment, deployment) {
			req.Logger.Info("updating deployment", "deployment", key)

			// TODO Figure out how we want to handle any deployment template spec updates and intelligently copy them.
			// Note that simply calling Deployment.DeepCopy() will fail on update because the
			// label selector is immutable.

			// TODO add unit/integration tests

			desiredDeployment.Labels = util.MergeMap(map[string]string{}, deployment.Labels, desiredDeployment.Labels)
			desiredDeployment.Annotations = util.MergeMap(map[string]string{}, deployment.Annotations, desiredDeployment.Annotations)

			deployment.Labels = desiredDeployment.Labels
			deployment.Annotations = desiredDeployment.Annotations

			deployment.Spec.Template.Labels = desiredDeployment.Spec.Template.Labels
			deployment.Spec.Template.Annotations = desiredDeployment.Spec.Template.Annotations
			deployment.Spec.Template.Spec.Containers = desiredDeployment.Spec.Template.Spec.Containers

			deployment.Spec.MinReadySeconds = desiredDeployment.Spec.MinReadySeconds
			deployment.Spec.Paused = desiredDeployment.Spec.Paused
			deployment.Spec.ProgressDeadlineSeconds = desiredDeployment.Spec.ProgressDeadlineSeconds
			deployment.Spec.RevisionHistoryLimit = desiredDeployment.Spec.RevisionHistoryLimit
			deployment.Spec.Strategy = desiredDeployment.Spec.Strategy

			if err = r.Update(ctx, deployment); err != nil {
				req.Logger.Error(err, "failed to update deployment", "deployment", deployment)
			}
			return &ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}

		if isDeploymentReady(deployment) {
			if err := req.StatusManager.SetReady(ctx, reaper); err == nil {
				return nil, nil
			} else {
				req.Logger.Error(err, "failed to update status")
				return &ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}
		} else {
			req.Logger.Info("deployment not ready", "deployment", key)
			if err := req.StatusManager.SetNotReady(ctx, reaper); err != nil {
				req.Logger.Error(err, "deployment is not ready, failed to update reaper status", "deployment", key)
				return &ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}
			return &ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}
}

func (r *defaultReconciler) buildNewDeployment(req ReaperRequest) (*appsv1.Deployment, error) {
	reaper := req.Reaper
	cassdc, err := r.cassandraDatacenter(context.Background(), reaper)
	if err != nil {
		req.Logger.Error(err, "failed to fetch target CassandraDatacenter")
		return nil, err
	}

	deployment := newDeployment(reaper, cassdc.GetDatacenterServiceName())

	if len(reaper.Spec.ServerConfig.JmxUserSecretName) > 0 {
		secretKey := types.NamespacedName{Namespace: reaper.Namespace, Name: reaper.Spec.ServerConfig.JmxUserSecretName}
		secret, err := r.getSecret(secretKey)
		if err != nil {
			req.Logger.Error(err, "failed to get jmxUserSecret", "deployment", secretKey)
			return nil, err
		}

		if usernameEnvVar, passwordEnvVar, err := r.secretsManager.GetJmxAuthCredentials(secret); err == nil {
			addAuthEnvVars(deployment, usernameEnvVar, passwordEnvVar)
		} else {
			req.Logger.Error(err, "failed to get JMX credentials", "deployment", secretKey)
			return nil, err
		}
	}

	if len(reaper.Spec.ServerConfig.CassandraBackend.CassandraUserSecretName) > 0 {
		secretkey := types.NamespacedName{Namespace: reaper.Namespace, Name: reaper.Spec.ServerConfig.CassandraBackend.CassandraUserSecretName}
		secret, err := r.getSecret(secretkey)
		if err != nil {
			req.Logger.Error(err, "failed to get Cassandra authentication secret", "deployment", secretkey)
			return nil, err
		}

		if usernameEnvVar, passwordEnvVar, err := r.secretsManager.GetCassandraAuthCredentials(secret); err == nil {
			enableAuthVar := &corev1.EnvVar{
				Name:  envVarEnableCassAuth,
				Value: "true",
			}
			addAuthEnvVars(deployment, usernameEnvVar, passwordEnvVar, enableAuthVar)
		} else {
			req.Logger.Error(err, "failed to get Cassandra authentication credentials", "deployment", secretkey)
			return nil, err
		}
	}

	util.AddHashAnnotation(deployment)

	return deployment, nil
}

func addAuthEnvVars(deployment *appsv1.Deployment, vars ...*corev1.EnvVar) {
	initEnvVars := deployment.Spec.Template.Spec.InitContainers[0].Env
	envVars := deployment.Spec.Template.Spec.Containers[0].Env
	for _, v := range vars {
		initEnvVars = append(initEnvVars, *v)
		envVars = append(envVars, *v)
	}
	deployment.Spec.Template.Spec.InitContainers[0].Env = initEnvVars
	deployment.Spec.Template.Spec.Containers[0].Env = envVars
}

func newDeployment(reaper *api.Reaper, cassDcService string) *appsv1.Deployment {
	labels := createLabels(reaper)

	selector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      mlabels.ManagedByLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{mlabels.ManagedByLabelValue},
			},
			{
				Key:      mlabels.ReaperLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{reaper.Name},
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
		InitialDelaySeconds: 45,
		PeriodSeconds:       15,
	}

	envVars := make([]corev1.EnvVar, 0)
	if reaper.Spec.ServerConfig.CassandraBackend != nil {
		envVars = []corev1.EnvVar{
			{
				Name:  "REAPER_STORAGE_TYPE",
				Value: "cassandra",
			},
			{
				Name:  "REAPER_ENABLE_DYNAMIC_SEED_LIST",
				Value: "false",
			},
			{
				Name:  "REAPER_CASS_CONTACT_POINTS",
				Value: fmt.Sprintf("[%s]", cassDcService),
			},
			{
				Name:  "REAPER_AUTH_ENABLED",
				Value: "false",
			},
		}
	}

	if reaper.Spec.ServerConfig.AutoScheduling != nil {
		if reaper.Spec.ServerConfig.AutoScheduling.Enabled {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REAPER_AUTO_SCHEDULING_ENABLED",
				Value: "true",
			})
			if reaper.Spec.ServerConfig.AutoScheduling.InitialDelay != "" {
				envVars = append(envVars, corev1.EnvVar{
					Name:  "REAPER_AUTO_SCHEDULING_INITIAL_DELAY_PERIOD",
					Value: reaper.Spec.ServerConfig.AutoScheduling.InitialDelay,
				})
			}
			if reaper.Spec.ServerConfig.AutoScheduling.PeriodBetweenPolls != "" {
				envVars = append(envVars, corev1.EnvVar{
					Name:  "REAPER_AUTO_SCHEDULING_PERIOD_BETWEEN_POLLS",
					Value: reaper.Spec.ServerConfig.AutoScheduling.PeriodBetweenPolls,
				})
			}
			if reaper.Spec.ServerConfig.AutoScheduling.BeforeFirstSchedule != "" {
				envVars = append(envVars, corev1.EnvVar{
					Name:  "REAPER_AUTO_SCHEDULING_TIME_BEFORE_FIRST_SCHEDULE",
					Value: reaper.Spec.ServerConfig.AutoScheduling.BeforeFirstSchedule,
				})
			}
			if reaper.Spec.ServerConfig.AutoScheduling.ScheduleSpreadPeriod != "" {
				envVars = append(envVars, corev1.EnvVar{
					Name:  "REAPER_AUTO_SCHEDULING_SCHEDULE_SPREAD_PERIOD",
					Value: reaper.Spec.ServerConfig.AutoScheduling.ScheduleSpreadPeriod,
				})
			}
			if reaper.Spec.ServerConfig.AutoScheduling.ExcludedClusters != nil {
				envVars = append(envVars, corev1.EnvVar{
					Name:  "REAPER_AUTO_SCHEDULING_EXCLUDED_CLUSTERS",
					Value: fmt.Sprintf("[%s]", strings.Join(reaper.Spec.ServerConfig.AutoScheduling.ExcludedClusters, ", ")),
				})
			}
			if reaper.Spec.ServerConfig.AutoScheduling.ExcludedKeyspace != nil {
				envVars = append(envVars, corev1.EnvVar{
					Name:  "REAPER_AUTO_SCHEDULING_EXCLUDED_KEYSPACES",
					Value: fmt.Sprintf("[%s]", strings.Join(reaper.Spec.ServerConfig.AutoScheduling.ExcludedKeyspace, ", ")),
				})
			}
		}
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reaper.Namespace,
			Name:      reaper.Name,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &selector,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Affinity: reaper.Spec.Affinity,
					InitContainers: []corev1.Container{
						{
							Name:            "reaper-schema-init",
							ImagePullPolicy: corev1.PullPolicy(reaper.Spec.ImagePullPolicy),
							Image:           reaper.Spec.Image,
							SecurityContext: reaper.Spec.SchemaInitContainerConfig.SecurityContext,
							Env:             envVars,
							Args:            []string{"schema-migration"},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "reaper",
							ImagePullPolicy: corev1.PullPolicy(reaper.Spec.ImagePullPolicy),
							Image:           reaper.Spec.Image,
							SecurityContext: reaper.Spec.SecurityContext,
							Ports: []corev1.ContainerPort{
								{
									Name:          "app",
									ContainerPort: 8080,
									Protocol:      "TCP",
								},
								{
									Name:          "admin",
									ContainerPort: 8081,
									Protocol:      "TCP",
								},
							},
							LivenessProbe:  healthProbe,
							ReadinessProbe: healthProbe,
							Env:            envVars,
						},
					},
					ServiceAccountName: reaper.Spec.ServiceAccountName,
					Tolerations:        reaper.Spec.Tolerations,
					SecurityContext:    reaper.Spec.PodSecurityContext,
				},
			},
		},
	}
}

func isDeploymentReady(deployment *appsv1.Deployment) bool {
	return deployment.Status.ReadyReplicas == 1
}

func createLabels(r *api.Reaper) map[string]string {
	labels := make(map[string]string)
	labels[mlabels.ReaperLabel] = r.Name
	mlabels.SetOperatorLabels(labels)

	return labels
}

func (r *defaultReconciler) getSecret(key types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.Get(context.Background(), key, secret)

	return secret, err
}

func (r *defaultReconciler) getCassandraDatacenterPods(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) ([]corev1.Pod, error) {
	cassdcSvc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Namespace: cassdc.Namespace, Name: cassdc.GetAllPodsServiceName()}, cassdcSvc)
	if err != nil {
		return nil, err
	}

	podList := &corev1.PodList{}
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: cassdcSvc.Spec.Selector})
	if err != nil {
		return nil, err
	}

	listOpts := []client.ListOption{
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	}

	if err := r.List(context.Background(), podList, listOpts...); err != nil {
		// r.Log.Error(err, "failed to get pods for cassandradatacenter", "CassandraDatacenter", cassdc.Name)
		return nil, err
	}

	pods := make([]corev1.Pod, 0)
	pods = append(pods, podList.Items...)

	return pods, nil
}
