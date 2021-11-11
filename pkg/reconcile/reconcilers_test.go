package reconcile

import (
	"context"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/reaper-operator/api/v1alpha1"
	mlabels "github.com/k8ssandra/reaper-operator/pkg/labels"
	"github.com/k8ssandra/reaper-operator/pkg/status"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestNewService(t *testing.T) {
	reaper := newReaperWithCassandraBackend("service-test", "test-reaper")
	key := types.NamespacedName{Namespace: reaper.Namespace, Name: GetServiceName(reaper.Name)}

	service := newService(key, reaper)

	assert.Equal(t, key.Name, service.Name)
	assert.Equal(t, key.Namespace, service.Namespace)
	assert.Equal(t, createLabels(reaper), service.Labels)

	assert.Equal(t, createLabels(reaper), service.Spec.Selector)
	assert.Equal(t, 1, len(service.Spec.Ports))

	port := corev1.ServicePort{
		Name:     "app",
		Protocol: corev1.ProtocolTCP,
		Port:     8080,
		TargetPort: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: "app",
		},
	}
	assert.Equal(t, port, service.Spec.Ports[0])
}

func TestNewDeployment(t *testing.T) {
	assert := assert.New(t)
	image := "test/reaper:latest"
	reaper := newReaperWithCassandraBackend("service-test", "test-reaper")
	reaper.Spec.Image = image
	reaper.Spec.ImagePullPolicy = "Always"
	reaper.Spec.ServerConfig.AutoScheduling = &api.AutoScheduler{Enabled: true}
	reaper.Spec.ServiceAccountName = "reaper"

	labels := createLabels(reaper)
	deployment := newDeployment(reaper, "target-datacenter-service")

	assert.Equal(reaper.Namespace, deployment.Namespace)
	assert.Equal(reaper.Name, deployment.Name)
	assert.Equal(labels, deployment.Labels)
	assert.Equal(reaper.Spec.ServiceAccountName, deployment.Spec.Template.Spec.ServiceAccountName)

	selector := deployment.Spec.Selector
	assert.Equal(0, len(selector.MatchLabels))
	assert.ElementsMatch(selector.MatchExpressions, []metav1.LabelSelectorRequirement{
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
	})

	assert.Equal(labels, deployment.Spec.Template.Labels)

	podSpec := deployment.Spec.Template.Spec
	assert.Equal(1, len(podSpec.Containers))

	container := podSpec.Containers[0]

	assert.Equal(image, container.Image)
	assert.Equal(corev1.PullAlways, container.ImagePullPolicy)
	assert.ElementsMatch(container.Env, []corev1.EnvVar{
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
			Value: "[target-datacenter-service]",
		},
		{
			Name:  "REAPER_AUTH_ENABLED",
			Value: "false",
		},
		{
			Name:  "REAPER_AUTO_SCHEDULING_ENABLED",
			Value: "true",
		},
	})

	assert.Equal(1, len(podSpec.InitContainers))

	initContainer := podSpec.InitContainers[0]
	assert.Equal(image, initContainer.Image)
	assert.Equal(corev1.PullAlways, initContainer.ImagePullPolicy)
	assert.ElementsMatch(initContainer.Env, []corev1.EnvVar{
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
			Value: "[target-datacenter-service]",
		},
		{
			Name:  "REAPER_AUTH_ENABLED",
			Value: "false",
		},
		{
			Name:  "REAPER_AUTO_SCHEDULING_ENABLED",
			Value: "true",
		},
	})

	assert.ElementsMatch(initContainer.Args, []string{"schema-migration"})

	reaper.Spec.ServerConfig.AutoScheduling = &api.AutoScheduler{
		Enabled:              false,
		InitialDelay:         "PT10S",
		PeriodBetweenPolls:   "PT5M",
		BeforeFirstSchedule:  "PT10M",
		ScheduleSpreadPeriod: "PT6H",
		ExcludedClusters:     []string{"a", "b"},
		ExcludedKeyspace:     []string{"system.powers"},
	}

	deployment = newDeployment(reaper, "target-datacenter-service")
	podSpec = deployment.Spec.Template.Spec
	container = podSpec.Containers[0]
	assert.Equal(4, len(container.Env))

	reaper.Spec.ServerConfig.AutoScheduling.Enabled = true
	deployment = newDeployment(reaper, "target-datacenter-service")
	podSpec = deployment.Spec.Template.Spec
	container = podSpec.Containers[0]
	assert.Equal(11, len(container.Env))

	assert.Contains(container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_PERIOD_BETWEEN_POLLS",
		Value: "PT5M",
	})

	assert.Contains(container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_TIME_BEFORE_FIRST_SCHEDULE",
		Value: "PT10M",
	})

	assert.Contains(container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_INITIAL_DELAY_PERIOD",
		Value: "PT10S",
	})

	assert.Contains(container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_EXCLUDED_CLUSTERS",
		Value: "[a, b]",
	})

	assert.Contains(container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_EXCLUDED_KEYSPACES",
		Value: "[system.powers]",
	})

	probe := &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthcheck",
				Port: intstr.FromInt(8081),
			},
		},
		InitialDelaySeconds: 45,
		PeriodSeconds:       15,
	}
	assert.Equal(probe, container.LivenessProbe)
	assert.Equal(probe, container.ReadinessProbe)
}

func TestTolerations(t *testing.T) {
	image := "test/reaper:latest"
	tolerations := []corev1.Toleration{
		{
			Key:      "key1",
			Operator: corev1.TolerationOpEqual,
			Value:    "value1",
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:      "key2",
			Operator: corev1.TolerationOpEqual,
			Value:    "value2",
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}

	reaper := newReaperWithCassandraBackend("service-test", "test-reaper")
	reaper.Spec.Image = image
	reaper.Spec.Tolerations = tolerations

	deployment := newDeployment(reaper, "target-datacenter-service")

	assert.ElementsMatch(t, tolerations, deployment.Spec.Template.Spec.Tolerations)
}

func TestAffinity(t *testing.T) {
	image := "test/reaper:latest"
	affinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/e2e-az-name",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"e2e-az1", "e2e-az2"},
							},
						},
					},
				},
			},
		},
	}
	reaper := newReaperWithCassandraBackend("service-test", "test-reaper")
	reaper.Spec.Image = image
	reaper.Spec.Affinity = affinity

	deployment := newDeployment(reaper, "target-datacenter-service")

	same := assert.ObjectsAreEqualValues(affinity, deployment.Spec.Template.Spec.Affinity)
	assert.True(t, same, "affinity does not match")
}

func TestContainerSecurityContext(t *testing.T) {
	image := "test/reaper:latest"
	readOnlyRootFilesystemOverride := true
	securityContext := &corev1.SecurityContext{
		ReadOnlyRootFilesystem: &readOnlyRootFilesystemOverride,
	}
	reaper := newReaperWithCassandraBackend("service-test", "test-reaper")
	reaper.Spec.Image = image
	reaper.Spec.SecurityContext = securityContext

	deployment := newDeployment(reaper, "target-datacenter-service")
	podSpec := deployment.Spec.Template.Spec

	assert.True(t, len(podSpec.Containers) == 1, "Expected a single container to exist")
	assert.Equal(t, podSpec.Containers[0].Name, "reaper")

	same := assert.ObjectsAreEqualValues(securityContext, podSpec.Containers[0].SecurityContext)
	assert.True(t, same, "securityContext does not match for container")
}

func TestSchemaInitContainerSecurityContext(t *testing.T) {
	image := "test/reaper:latest"
	readOnlyRootFilesystemOverride := true
	initContainerSecurityContext := &corev1.SecurityContext{
		ReadOnlyRootFilesystem: &readOnlyRootFilesystemOverride,
	}
	nonInitContainerSecurityContext := &corev1.SecurityContext{
		ReadOnlyRootFilesystem: &readOnlyRootFilesystemOverride,
	}

	reaper := newReaperWithCassandraBackend("service-test", "test-reaper")
	reaper.Spec.Image = image
	reaper.Spec.SecurityContext = nonInitContainerSecurityContext
	reaper.Spec.SchemaInitContainerConfig.SecurityContext = initContainerSecurityContext

	deployment := newDeployment(reaper, "target-datacenter-service")
	podSpec := deployment.Spec.Template.Spec

	assert.Equal(t, podSpec.InitContainers[0].Name, "reaper-schema-init")
	assert.True(t, len(podSpec.InitContainers) == 1, "Expected a single schema init container to exist")
	same := assert.ObjectsAreEqualValues(initContainerSecurityContext, podSpec.InitContainers[0].SecurityContext)
	assert.True(t, same, "securityContext does not match for schema init container")
}

func TestPodSecurityContext(t *testing.T) {
	image := "test/reaper:latest"
	runAsUser := int64(8675309)
	podSecurityContext := &corev1.PodSecurityContext{
		RunAsUser: &runAsUser,
	}
	reaper := newReaperWithCassandraBackend("service-test", "test-reaper")
	reaper.Spec.Image = image
	reaper.Spec.PodSecurityContext = podSecurityContext

	deployment := newDeployment(reaper, "target-datacenter-service")
	podSpec := deployment.Spec.Template.Spec

	same := assert.ObjectsAreEqualValues(podSecurityContext, podSpec.SecurityContext)
	assert.True(t, same, "podSecurityContext expected at pod level")
}

func TestReaperDeploymentStrategy(t *testing.T) {
	// Test to ensure that if we set DeploymentStrategy to Rolling then it gets set back to Recreate by reconcile()
	// Reaper test resource
	testReaper := newReaperWithCassandraBackend("default", "test-reaper")
	runAsUser := int64(8675309)
	testReaper.Spec.Image = "test/reaper:latest"
	testReaper.Spec.PodSecurityContext = &corev1.PodSecurityContext{
		RunAsUser: &runAsUser,
	}
	// Create CassDC
	testCassDC := newCassDC("default", "dc1")

	// Test Scheme
	localSchemeBuilder := runtime.SchemeBuilder{
		api.AddToScheme,
		appsv1.AddToScheme,
		cassdcapi.AddToScheme,
	}
	testScheme := runtime.NewScheme()
	localSchemeBuilder.AddToScheme(testScheme)

	// Mock client
	k8sClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(testCassDC.DeepCopy()).Build()

	// Other fixtures
	testSecretsmngr := NewSecretsManager()
	testctx := context.Background()
	testStatusManager := &status.StatusManager{Client: k8sClient}
	testLogger := zap.New()

	// Test reconcile deployment
	r := defaultReconciler{
		k8sClient,
		testScheme,
		testSecretsmngr,
	}
	req := ReaperRequest{
		testReaper,
		testLogger,
		testStatusManager,
	}
	r.ReconcileDeployment(testctx, req)

	// Ensure Reaper is deploying with RecreateStrategy
	deploymentList := &appsv1.DeploymentList{}
	if err := k8sClient.List(testctx, deploymentList, client.MatchingLabels{"app.kubernetes.io/managed-by": "reaper-operator"}); err != nil {
		t.Log(err)
		assert.FailNow(t, "Failed to get Reaper instance")
	}
	reaperDeployment := deploymentList.Items[0].DeepCopy()
	assert.True(t, len(deploymentList.Items) == 1)                                               // Only one deployment should be observed
	assert.True(t, reaperDeployment.Spec.Strategy.Type == appsv1.RecreateDeploymentStrategyType) // Deployment type is right.

	// Change deploymentStrategy to RollingUpdate
	reaperDeploymentPatch := client.MergeFrom(reaperDeployment.DeepCopy())
	reaperDeployment.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
	}
	if err := r.Patch(testctx, reaperDeployment, reaperDeploymentPatch); err != nil {
		t.Log(err)
		assert.FailNow(t, "Failed to patch deployment with appsv1.RecreateDeploymentStrategyType")
	}

	if err := k8sClient.List(testctx, deploymentList, client.MatchingLabels{"app.kubernetes.io/managed-by": "reaper-operator"}); err != nil {
		t.Log(err)
		assert.FailNow(t, "Failed to get Reaper Deployment")
	}
	assert.True(t, deploymentList.Items[0].Spec.Strategy.Type == appsv1.RollingUpdateDeploymentStrategyType)

	// Reconcile - expect to see DeploymenStrategyType change back to Recreate
	r.ReconcileDeployment(testctx, req)
	if err := k8sClient.List(testctx, deploymentList, client.MatchingLabels{"app.kubernetes.io/managed-by": "reaper-operator"}); err != nil {
		t.Log(err)
		assert.FailNow(t, "Failed to get Reaper Deployment")
	}
	assert.True(t, len(deploymentList.Items) == 1) // Only one deployment should be observed
	assert.True(t, deploymentList.Items[0].Spec.Strategy.Type == appsv1.RecreateDeploymentStrategyType)

}

func newReaperWithCassandraBackend(namespace string, reaperName string) *api.Reaper {
	dcName := "dc1"

	return &api.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      reaperName,
		},
		Spec: api.ReaperSpec{
			ServerConfig: api.ServerConfig{
				StorageType: api.StorageTypeCassandra,
				CassandraBackend: &api.CassandraBackend{
					CassandraDatacenter: api.CassandraDatacenterRef{
						Name: dcName,
					},
					Keyspace: api.DefaultKeyspace,
					Replication: api.ReplicationConfig{
						NetworkTopologyStrategy: &map[string]int32{
							"DC1": 3,
						},
					},
				},
			},
		},
	}
}

func newCassDC(namespace string, cassDCName string) cassdcapi.CassandraDatacenter {
	return cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cassDCName,
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName:   "name",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
			Size:          3,
		},
	}
}
