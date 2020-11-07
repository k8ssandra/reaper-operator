package controllers

import (
	"context"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/thelastpickle/reaper-operator/api/v1alpha1"
	mlabels "github.com/thelastpickle/reaper-operator/pkg/labels"
	"github.com/thelastpickle/reaper-operator/pkg/reconcile"
	appsv1 "k8s.io/api/apps/v1"
	v1batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ReaperName           = "test-reaper"
	CassandraClusterName = "test-cluster"
	CassandraServiceName = "test-cluster-svc"

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

	})

	Specify("create a new Reaper instance", func() {
		By("create the Reaper object")
		reaper := createReaper(ReaperNamespace)
		Expect(k8sClient.Create(context.Background(), reaper)).Should(Succeed())

		By("check that the service is created")
		serviceKey := types.NamespacedName{Namespace: ReaperNamespace, Name: reconcile.GetServiceName(reaper.Name)}
		service := &corev1.Service{}

		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), serviceKey, service)
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
			err := k8sClient.Get(context.Background(), jobKey, job)
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
		Expect(k8sClient.Status().Patch(context.Background(), job, jobPatch)).Should(Succeed())

		Expect(len(job.OwnerReferences)).Should(Equal(1))
		Expect(job.OwnerReferences[0].UID).Should(Equal(reaper.GetUID()))

		By("check that the deployment is created")
		deploymentKey := types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName}
		deployment := &appsv1.Deployment{}

		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), deploymentKey, deployment)
			if err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue(), "deployment creation check failed")

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
		Expect(k8sClient.Create(context.Background(), job)).Should(Succeed())

		// We need to mock the job completion in order for the deployment to get created
		jobPatch := client.MergeFrom(job.DeepCopy())
		job.Status.Conditions = append(job.Status.Conditions, v1batch.JobCondition{
			Type:   v1batch.JobComplete,
			Status: corev1.ConditionTrue,
		})
		Expect(k8sClient.Status().Patch(context.Background(), job, jobPatch)).Should(Succeed())

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
					ClusterName:      CassandraClusterName,
					CassandraService: CassandraServiceName,
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
