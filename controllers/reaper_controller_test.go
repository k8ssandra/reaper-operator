package controllers

import (
	"context"
	"strconv"
	"time"

	api "github.com/k8ssandra/reaper-operator/api/v1alpha1"
	mlabels "github.com/k8ssandra/reaper-operator/pkg/labels"
	"github.com/k8ssandra/reaper-operator/pkg/reconcile"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
)

const (
	ReaperName              = "test-reaper"
	CassandraDatacenterName = "test-dc"

	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

var _ = Describe("Reaper controller", func() {
	i := 0
	ReaperNamespace := ""

	BeforeEach(func() {
		ReaperNamespace = "reaper-test-" + strconv.Itoa(i)
		testNamespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ReaperNamespace,
			},
		}
		Expect(k8sClient.Create(context.Background(), testNamespace)).Should(Succeed())
		i = i + 1

		cassdcKey := types.NamespacedName{Name: CassandraDatacenterName, Namespace: ReaperNamespace}
		testDc := &cassdcapi.CassandraDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      CassandraDatacenterName,
				Namespace: ReaperNamespace,
			},
			Spec: cassdcapi.CassandraDatacenterSpec{
				ClusterName:   "test-dc",
				ServerType:    "cassandra",
				ServerVersion: "3.11.7",
				Size:          3,
			},
			Status: cassdcapi.CassandraDatacenterStatus{},
		}
		Expect(k8sClient.Create(context.Background(), testDc)).Should(Succeed())

		patchCassdc := client.MergeFrom(testDc.DeepCopy())
		testDc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		testDc.Status.Conditions = []cassdcapi.DatacenterCondition{
			{
				Status: corev1.ConditionTrue,
				Type:   cassdcapi.DatacenterReady,
			},
		}

		cassdc := &cassdcapi.CassandraDatacenter{}
		Expect(k8sClient.Status().Patch(context.Background(), testDc, patchCassdc)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), cassdcKey, cassdc)
			if err != nil {
				return false
			}
			return cassdc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
		}, timeout, interval).Should(BeTrue())

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cassdc-pod1",
				Namespace: ReaperNamespace,
				Labels: map[string]string{
					cassdcapi.ClusterLabel:    "test-dc1",
					cassdcapi.DatacenterLabel: "dc1",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "cassandra",
						Image: "k8ssandra/cassandra-nothere:latest",
					},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), pod)).Should(Succeed())

		podIP := "127.0.0.1"

		patchPod := client.MergeFrom(pod.DeepCopy())
		pod.Status = corev1.PodStatus{
			PodIP: podIP,
			PodIPs: []corev1.PodIP{
				{
					IP: podIP,
				},
			},
		}
		Expect(k8sClient.Status().Patch(context.Background(), pod, patchPod)).Should(Succeed())

		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-dc-test-dc-all-pods-service",
				Namespace: ReaperNamespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name: "mgmt-api-http",
						Port: int32(8080),
					},
				},
				Selector: map[string]string{
					cassdcapi.ClusterLabel:    "test-dc1",
					cassdcapi.DatacenterLabel: "dc1",
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), service)).Should(Succeed())
	})

	Specify("create a new Reaper instance", func() {
		By("create the Reaper object")
		reaper := createReaper(ReaperNamespace)
		Expect(k8sClient.Create(context.Background(), reaper)).Should(Succeed())

		By("check that the service is created")
		serviceKey := types.NamespacedName{Namespace: ReaperNamespace, Name: reconcile.GetServiceName(reaper.Name)}
		service := &corev1.Service{}

		Eventually(func() error {
			return k8sClient.Get(context.Background(), serviceKey, service)
		}, timeout, interval).Should(Succeed(), "service creation check failed")

		Expect(len(service.OwnerReferences)).Should(Equal(1))
		Expect(service.OwnerReferences[0].UID).Should(Equal(reaper.UID))

		By("check that the deployment is created")
		deploymentKey := types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName}
		deployment := &appsv1.Deployment{}

		Eventually(func() error {
			return k8sClient.Get(context.Background(), deploymentKey, deployment)
		}, timeout, interval).Should(Succeed(), "deployment creation check failed")

		Expect(len(deployment.OwnerReferences)).Should(Equal(1), "deployment owner reference not set")
		Expect(deployment.OwnerReferences[0].UID).Should(Equal(reaper.GetUID()), "deployment owner reference has wrong uid")

		By("update deployment to be ready")
		patchDeploymentStatus(deployment, 1, 1)

		verifyReaperReady(types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName})

		// Now simulate the Reaper app entering a state in which its readiness probe fails. This
		// should cause the deployment to have its status updated. The Reaper object's .Status.Ready
		// field should subsequently be updated.
		By("update deployment to be not ready")
		patchDeploymentStatus(deployment, 1, 0)

		reaperKey := types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName}
		updatedReaper := &api.Reaper{}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), reaperKey, updatedReaper)
			if err != nil {
				return false
			}
			ctrl.Log.WithName("test").Info("after update", "updatedReaper", updatedReaper)
			return updatedReaper.Status.Ready == false
		}, timeout, interval).Should(BeTrue(), "reaper status should have been updated")
	})

	Specify("create a new Reaper instance when objects exist", func() {
		// The purpose of this test is to cover code paths where an object, e.g., the
		// deployment already exists. This could happen after a failed reconciliation and
		// the request gets requeued.

		By("create the service")
		serviceKey := types.NamespacedName{Namespace: ReaperNamespace, Name: reconcile.GetServiceName(ReaperName)}
		// We can use a fake service here with only the required properties set. Since the service already
		// exists, the reconciler should continue its work. There are unit tests to verify that the service
		// is created as expected.
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: serviceKey.Namespace,
				Name:      serviceKey.Name,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "fake-port",
						Protocol: corev1.ProtocolTCP,
						Port:     8888,
					},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), service)).Should(Succeed())

		By("create the deployment")
		// We can use a fake deployment here with only the required properties set. Since the deployment
		// already exists, the reconciler will just check that it is ready. There are unit tests to
		// verify that the deployment is created as expected.
		labels := map[string]string{
			"reaper.cassandra-reaper.io/reaper": ReaperName,
			"app.kubernetes.io/managed-by":      "reaper-operator",
		}
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ReaperNamespace,
				Name:      ReaperName,
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      mlabels.ManagedByLabel,
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{mlabels.ManagedByLabelValue},
						},
						{
							Key:      mlabels.ReaperLabel,
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{ReaperName},
						},
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "fake-deployment",
								Image: "fake-deployment:test",
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), deployment)).Should(Succeed())

		// We need to mock the deployment being ready in order for Reaper status to be updated
		By("update deployment to be ready")
		patchDeploymentStatus(deployment, 1, 1)

		By("create the Reaper object")
		reaper := createReaper(ReaperNamespace)
		Expect(k8sClient.Create(context.Background(), reaper)).Should(Succeed())

		verifyReaperReady(types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName})
	})

	Specify("autoscheduling is enabled", func() {
		By("create the Reaper object")
		reaper := createReaper(ReaperNamespace)
		reaper.Spec.ServerConfig.AutoScheduling = &api.AutoScheduler{
			Enabled: true,
		}
		Expect(k8sClient.Create(context.Background(), reaper)).Should(Succeed())

		By("check that the deployment is created")
		deploymentKey := types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName}
		deployment := &appsv1.Deployment{}

		Eventually(func() error {
			return k8sClient.Get(context.Background(), deploymentKey, deployment)
		}, timeout, interval).Should(Succeed(), "deployment creation check failed")

		Expect(len(deployment.Spec.Template.Spec.Containers)).Should(Equal(1))

		autoSchedulingEnabled := false

		for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
			if env.Name == "REAPER_AUTO_SCHEDULING_ENABLED" && env.Value == "true" {
				autoSchedulingEnabled = true
			}
		}
		Expect(autoSchedulingEnabled).Should(BeTrue())
	})

	Specify("cassandra uses authentication", func() {
		By("creating a secret")
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ReaperNamespace,
				Name:      "top-secret-cass",
			},
			Data: map[string][]byte{
				"username": []byte("bond"),
				"password": []byte("james"),
			},
		}
		Expect(k8sClient.Create(context.Background(), &secret)).Should(Succeed())

		By("create the Reaper object and modify it")
		reaper := createReaper(ReaperNamespace)
		reaper.Spec.ServerConfig.CassandraBackend.CassandraUserSecretName = "top-secret-cass"
		Expect(k8sClient.Create(context.Background(), reaper)).Should(Succeed())

		By("check that the deployment is created")
		deploymentKey := types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName}
		deployment := &appsv1.Deployment{}

		Eventually(func() error {
			return k8sClient.Get(context.Background(), deploymentKey, deployment)
		}, timeout, interval).Should(Succeed(), "deployment creation check failed")

		By("verify the deployment has CassAuth EnvVars")
		envVars := deployment.Spec.Template.Spec.Containers[0].Env
		Expect(envVars[len(envVars)-3].Name).To(Equal("REAPER_CASS_AUTH_USERNAME"))
		Expect(envVars[len(envVars)-3].ValueFrom.SecretKeyRef.LocalObjectReference.Name).To(Equal("top-secret-cass"))
		Expect(envVars[len(envVars)-3].ValueFrom.SecretKeyRef.Key).To(Equal("username"))
		Expect(envVars[len(envVars)-2].Name).To(Equal("REAPER_CASS_AUTH_PASSWORD"))
		Expect(envVars[len(envVars)-2].ValueFrom.SecretKeyRef.LocalObjectReference.Name).To(Equal("top-secret-cass"))
		Expect(envVars[len(envVars)-2].ValueFrom.SecretKeyRef.Key).To(Equal("password"))
		Expect(envVars[len(envVars)-1].Name).To(Equal("REAPER_CASS_AUTH_ENABLED"))
		Expect(envVars[len(envVars)-1].Value).To(Equal("true"))

		// Schema init env var check, note: config init doesn't currently utilize these vars
		schemaInitEnvVars := deployment.Spec.Template.Spec.InitContainers[1].Env
		Expect(schemaInitEnvVars[len(schemaInitEnvVars)-3].Name).To(Equal("REAPER_CASS_AUTH_USERNAME"))
		Expect(schemaInitEnvVars[len(schemaInitEnvVars)-3].ValueFrom.SecretKeyRef.LocalObjectReference.Name).To(Equal("top-secret-cass"))
		Expect(schemaInitEnvVars[len(schemaInitEnvVars)-3].ValueFrom.SecretKeyRef.Key).To(Equal("username"))
		Expect(schemaInitEnvVars[len(schemaInitEnvVars)-2].Name).To(Equal("REAPER_CASS_AUTH_PASSWORD"))
		Expect(schemaInitEnvVars[len(schemaInitEnvVars)-2].ValueFrom.SecretKeyRef.LocalObjectReference.Name).To(Equal("top-secret-cass"))
		Expect(schemaInitEnvVars[len(schemaInitEnvVars)-2].ValueFrom.SecretKeyRef.Key).To(Equal("password"))
		Expect(schemaInitEnvVars[len(schemaInitEnvVars)-1].Name).To(Equal("REAPER_CASS_AUTH_ENABLED"))
		Expect(schemaInitEnvVars[len(schemaInitEnvVars)-1].Value).To(Equal("true"))
	})
})

// Creates a new Reaper object with a Cassandra backend
func createReaper(namespace string) *api.Reaper {
	return &api.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      ReaperName,
		},
		Spec: api.ReaperSpec{
			ServerConfig: api.ServerConfig{
				StorageType: api.StorageTypeCassandra,
				CassandraBackend: &api.CassandraBackend{
					CassandraDatacenter: api.CassandraDatacenterRef{
						Name: CassandraDatacenterName,
					},
				},
			},
		},
	}
}

func verifyReaperReady(key types.NamespacedName) {
	By("check that the reaper is ready")
	Eventually(func() bool {
		updatedReaper := &api.Reaper{}
		if err := k8sClient.Get(context.Background(), key, updatedReaper); err != nil {
			return false
		}
		ctrl.Log.WithName("test").Info("after update", "updatedReaper", updatedReaper)
		return updatedReaper.Status.Ready
	}, timeout, interval).Should(BeTrue())
}

func patchDeploymentStatus(deployment *appsv1.Deployment, replicas, readyReplicas int32) {
	deploymentPatch := client.MergeFrom(deployment.DeepCopy())
	deployment.Status.Replicas = replicas
	deployment.Status.ReadyReplicas = readyReplicas
	Expect(k8sClient.Status().Patch(context.Background(), deployment, deploymentPatch)).Should(Succeed())
}
