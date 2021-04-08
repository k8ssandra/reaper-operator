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
	v1batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cassdcapi "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
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
		Expect(testClient.Create(context.Background(), testNamespace)).Should(Succeed())
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
		Expect(testClient.Create(context.Background(), testDc)).Should(Succeed())

		patchCassdc := client.MergeFrom(testDc.DeepCopy())
		testDc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		testDc.Status.Conditions = []cassdcapi.DatacenterCondition{
			{
				Status: corev1.ConditionTrue,
				Type:   cassdcapi.DatacenterReady,
			},
		}

		cassdc := &cassdcapi.CassandraDatacenter{}
		Expect(testClient.Status().Patch(context.Background(), testDc, patchCassdc)).Should(Succeed())
		Eventually(func() bool {
			err := testClient.Get(context.Background(), cassdcKey, cassdc)
			if err != nil {
				return false
			}
			return cassdc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
		}, timeout, interval).Should(BeTrue())
	})

	Specify("create a new Reaper instance", func() {
		By("create the Reaper object")
		reaper := createReaper(ReaperNamespace)
		Expect(testClient.Create(context.Background(), reaper)).Should(Succeed())

		By("check that the service is created")
		serviceKey := types.NamespacedName{Namespace: ReaperNamespace, Name: reconcile.GetServiceName(reaper.Name)}
		service := &corev1.Service{}

		Eventually(func() bool {
			err := testClient.Get(context.Background(), serviceKey, service)
			if err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue(), "service creation check failed")

		Expect(len(service.OwnerReferences)).Should(Equal(1))
		Expect(service.OwnerReferences[0].UID).Should(Equal(reaper.UID))

		By("check that the schema job is created")
		jobKey := types.NamespacedName{Namespace: reaper.Namespace, Name: reaper.Name + "-schema"}
		job := &v1batch.Job{}

		Eventually(func() bool {
			err := testClient.Get(context.Background(), jobKey, job)
			if err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue(), "schema job creation check failed")

		// We need to mock the job completion in order for the deployment to get created
		jobPatch := client.MergeFrom(job.DeepCopy())
		job.Status.Conditions = append(job.Status.Conditions, v1batch.JobCondition{
			Type:   v1batch.JobComplete,
			Status: corev1.ConditionTrue,
		})
		Expect(testClient.Status().Patch(context.Background(), job, jobPatch)).Should(Succeed())

		Expect(len(job.OwnerReferences)).Should(Equal(1))
		Expect(job.OwnerReferences[0].UID).Should(Equal(reaper.GetUID()))

		By("check that the deployment is created")
		deploymentKey := types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName}
		deployment := &appsv1.Deployment{}

		Eventually(func() bool {
			err := testClient.Get(context.Background(), deploymentKey, deployment)
			if err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue(), "deployment creation check failed")

		Expect(len(deployment.OwnerReferences)).Should(Equal(1), "deployment owner reference not set")
		Expect(deployment.OwnerReferences[0].UID).Should(Equal(reaper.GetUID()), "deployment owner reference has wrong uid")

		By("update deployment to be ready")
		patchDeploymentStatus(deployment, 1, 1)

		reaperKey := types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName}
		verifyReaperReady(reaperKey)

		// Now simulate the Reaper app entering a state in which its readiness probe fails. This
		// should cause the deployment to have its status updated. The Reaper object's .Status.Ready
		// field should subsequently be updated.
		By("update deployment to be not ready")
		patchDeploymentStatus(deployment, 1, 0)

		verifyReaperNotReady(reaperKey)
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
		Expect(testClient.Create(context.Background(), service)).Should(Succeed())

		By("create the schema job")
		jobKey := types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName + "-schema"}
		// We can use a fake job here with only the required properties set. Since the job already
		// exists, the reconciler will just check its status. There are unit tests to verify that
		// the job is created as expected.
		job := &v1batch.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: jobKey.Namespace,
				Name:      jobKey.Name,
			},
			Spec: v1batch.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyOnFailure,
						Containers: []corev1.Container{
							{
								Name:  "fake-schema-job",
								Image: "fake-schema-job:test",
							},
						},
					},
				},
			},
		}
		Expect(testClient.Create(context.Background(), job)).Should(Succeed())

		// We need to mock the job completion in order for the deployment to get created
		jobPatch := client.MergeFrom(job.DeepCopy())
		job.Status.Conditions = append(job.Status.Conditions, v1batch.JobCondition{
			Type:   v1batch.JobComplete,
			Status: corev1.ConditionTrue,
		})
		Expect(testClient.Status().Patch(context.Background(), job, jobPatch)).Should(Succeed())

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
		Expect(testClient.Create(context.Background(), deployment)).Should(Succeed())

		// We need to mock the deployment being ready in order for Reaper status to be updated
		By("update deployment to be ready")
		patchDeploymentStatus(deployment, 1, 1)

		By("create the Reaper object")
		reaper := createReaper(ReaperNamespace)
		Expect(testClient.Create(context.Background(), reaper)).Should(Succeed())

		verifyReaperReady(types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName})
	})

	Specify("autoscheduling is enabled", func() {
		By("create the Reaper object")
		reaper := createReaper(ReaperNamespace)
		reaper.Spec.ServerConfig.AutoScheduling = &api.AutoScheduler{
			Enabled: true,
		}
		Expect(testClient.Create(context.Background(), reaper)).Should(Succeed())

		// Remove unnecessary parts, verify that the deployment has envVar set for autoscheduling
		By("check that the schema job is created")
		jobKey := types.NamespacedName{Namespace: reaper.Namespace, Name: reaper.Name + "-schema"}
		job := &v1batch.Job{}

		Eventually(func() bool {
			err := testClient.Get(context.Background(), jobKey, job)
			if err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue(), "schema job creation check failed")

		// We need to mock the job completion in order for the deployment to get created
		jobPatch := client.MergeFrom(job.DeepCopy())
		job.Status.Conditions = append(job.Status.Conditions, v1batch.JobCondition{
			Type:   v1batch.JobComplete,
			Status: corev1.ConditionTrue,
		})
		Expect(testClient.Status().Patch(context.Background(), job, jobPatch)).Should(Succeed())

		By("check that the deployment is created")
		deploymentKey := types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName}
		deployment := &appsv1.Deployment{}

		Eventually(func() bool {
			err := testClient.Get(context.Background(), deploymentKey, deployment)
			if err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue(), "deployment creation check failed")

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
		Expect(testClient.Create(context.Background(), &secret)).Should(Succeed())

		By("create the Reaper object and modify it")
		reaper := createReaper(ReaperNamespace)
		reaper.Spec.ServerConfig.CassandraBackend.CassandraUserSecretName = "top-secret-cass"
		Expect(testClient.Create(context.Background(), reaper)).Should(Succeed())

		By("check that the schema job is created")
		jobKey := types.NamespacedName{Namespace: reaper.Namespace, Name: reaper.Name + "-schema"}
		job := &v1batch.Job{}

		Eventually(func() bool {
			err := testClient.Get(context.Background(), jobKey, job)
			if err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue(), "schema job creation check failed")

		By("verify schema job has EnvVars set")
		Expect(job.Spec.Template.Spec.Containers[0].Env).Should(HaveLen(5))
		Expect(job.Spec.Template.Spec.Containers[0].Env[3].Name).To(Equal("USERNAME"))
		Expect(job.Spec.Template.Spec.Containers[0].Env[3].ValueFrom.SecretKeyRef.LocalObjectReference.Name).To(Equal("top-secret-cass"))
		Expect(job.Spec.Template.Spec.Containers[0].Env[3].ValueFrom.SecretKeyRef.Key).To(Equal("username"))
		Expect(job.Spec.Template.Spec.Containers[0].Env[4].Name).To(Equal("PASSWORD"))
		Expect(job.Spec.Template.Spec.Containers[0].Env[4].ValueFrom.SecretKeyRef.LocalObjectReference.Name).To(Equal("top-secret-cass"))
		Expect(job.Spec.Template.Spec.Containers[0].Env[4].ValueFrom.SecretKeyRef.Key).To(Equal("password"))

		// We need to mock the job completion in order for the deployment to get created
		jobPatch := client.MergeFrom(job.DeepCopy())
		job.Status.Conditions = append(job.Status.Conditions, v1batch.JobCondition{
			Type:   v1batch.JobComplete,
			Status: corev1.ConditionTrue,
		})
		Expect(testClient.Status().Patch(context.Background(), job, jobPatch)).Should(Succeed())

		By("check that the deployment is created")
		deploymentKey := types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName}
		deployment := &appsv1.Deployment{}

		Eventually(func() bool {
			err := testClient.Get(context.Background(), deploymentKey, deployment)
			if err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue(), "deployment creation check failed")

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

type ReaperConditionFunc func(reaper *api.Reaper) bool

func verifyReaperReady(key types.NamespacedName) {
	By("check that the reaper is ready")
	waitForReaperCondition(key, func(reaper *api.Reaper) bool {
		return reaper.Status.Ready
	})
	//Eventually(func() bool {
	//	updatedReaper := &api.Reaper{}
	//	if err := testClient.Get(context.Background(), key, updatedReaper); err != nil {
	//		return false
	//	}
	//	ctrl.Log.WithName("test").Info("after update", "updatedReaper", updatedReaper)
	//	return updatedReaper.Status.Ready
	//}, timeout, interval).Should(BeTrue())
}

func verifyReaperNotReady(key types.NamespacedName) {
	By("check that the reaper is not ready")
	waitForReaperCondition(key, func(reaper *api.Reaper) bool {
		return !reaper.Status.Ready
	})
	//Eventually(func() bool {
	//	updatedReaper := &api.Reaper{}
	//	err := testClient.Get(context.Background(), key, updatedReaper)
	//	if err != nil {
	//		return false
	//	}
	//	ctrl.Log.WithName("test").Info("after update", "updatedReaper", updatedReaper)
	//	return updatedReaper.Status.Ready == false
	//}, timeout, interval).Should(BeTrue(), "reaper status should have been updated")
}

func waitForReaperCondition(key types.NamespacedName, condition ReaperConditionFunc) {
	Eventually(func() bool {
		updatedReaper := &api.Reaper{}
		err := testClient.Get(context.Background(), key, updatedReaper)
		if err != nil {
			return false
		}
		ctrl.Log.WithName("test").Info("after update", "updatedReaper", updatedReaper)
		return condition(updatedReaper)
	}, timeout, interval).Should(BeTrue(), "reaper status should have been updated")
}

func patchDeploymentStatus(deployment *appsv1.Deployment, replicas, readyReplicas int32) {
	deploymentPatch := client.MergeFrom(deployment.DeepCopy())
	deployment.Status.Replicas = replicas
	deployment.Status.ReadyReplicas = readyReplicas
	Expect(testClient.Status().Patch(context.Background(), deployment, deploymentPatch)).Should(Succeed())
}
