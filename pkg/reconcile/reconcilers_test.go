package reconcile

import (
	"testing"

	api "github.com/k8ssandra/reaper-operator/api/v1alpha1"
	"github.com/k8ssandra/reaper-operator/pkg/config"
	mlabels "github.com/k8ssandra/reaper-operator/pkg/labels"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestNewService(t *testing.T) {
	reaper := newReaperWithCassandraBackend()
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

func TestNewSchemaJob(t *testing.T) {
	reaper := newReaperWithCassandraBackend()

	job := newSchemaJob(reaper)

	assert.Equal(t, getSchemaJobName(reaper), job.Name)
	assert.Equal(t, reaper.Namespace, job.Namespace)
	assert.Equal(t, createLabels(reaper), job.Labels)

	podSpec := job.Spec.Template.Spec
	assert.Equal(t, corev1.RestartPolicyOnFailure, podSpec.RestartPolicy)
	assert.Equal(t, 1, len(podSpec.Containers))

	container := podSpec.Containers[0]
	assert.Equal(t, schemaJobImage, container.Image)
	assert.Equal(t, schemaJobImagePullPolicy, container.ImagePullPolicy)
	assert.ElementsMatch(t, container.Env, []corev1.EnvVar{
		{
			Name:  "KEYSPACE",
			Value: api.DefaultKeyspace,
		},
		{
			Name:  "CONTACT_POINTS",
			Value: reaper.Spec.ServerConfig.CassandraBackend.CassandraService,
		},
		{
			Name:  "REPLICATION",
			Value: config.ReplicationToString(reaper.Spec.ServerConfig.CassandraBackend.Replication),
		},
	})
}

func TestNewDeployment(t *testing.T) {
	image := "test/reaper:latest"
	reaper := newReaperWithCassandraBackend()
	reaper.Spec.Image = image

	labels := createLabels(reaper)
	deployment := newDeployment(reaper)

	assert.Equal(t, reaper.Namespace, deployment.Namespace)
	assert.Equal(t, reaper.Name, deployment.Name)
	assert.Equal(t, labels, deployment.Labels)

	selector := deployment.Spec.Selector
	assert.Equal(t, 0, len(selector.MatchLabels))
	assert.ElementsMatch(t, selector.MatchExpressions, []metav1.LabelSelectorRequirement{
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

	assert.Equal(t, labels, deployment.Spec.Template.Labels)

	podSpec := deployment.Spec.Template.Spec
	assert.Equal(t, 1, len(podSpec.Containers))

	container := podSpec.Containers[0]
	assert.Equal(t, image, container.Image)
	assert.ElementsMatch(t, container.Env, []corev1.EnvVar{
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
			Value: "[" + reaper.Spec.ServerConfig.CassandraBackend.CassandraService + "]",
		},
		{
			Name:  "REAPER_AUTH_ENABLED",
			Value: "false",
		},
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
	assert.Equal(t, probe, container.LivenessProbe)
	assert.Equal(t, probe, container.ReadinessProbe)
}

func newReaperWithCassandraBackend() *api.Reaper {
	namespace := "service-test"
	reaperName := "test-reaper"
	clusterName := "cassandra"
	cassandraService := "cassandra-svc"

	return &api.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      reaperName,
		},
		Spec: api.ReaperSpec{
			ServerConfig: api.ServerConfig{
				StorageType: api.StorageTypeCassandra,
				CassandraBackend: &api.CassandraBackend{
					ClusterName:      clusterName,
					CassandraService: cassandraService,
					Keyspace:         api.DefaultKeyspace,
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
