package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/thelastpickle/reaper-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Reaper controller", func() {
	const (
		ReaperName           = "test-reaper"
		ReaperNamespace      = "default"
		CassandraClusterName = "test-cluster"
		CassandraServiceName = "test-cluster-svc"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Specify("create a new Reaper instance", func() {
		By("create the Reaper object")
		reaper := &api.Reaper{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ReaperNamespace,
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
		Expect(k8sClient.Create(context.Background(), reaper)).Should(Succeed())

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
		Expect(k8sClient.Status().Patch(context.Background(), job, jobPatch))

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

		Expect(len(deployment.OwnerReferences)).Should(Equal(1))
		Expect(deployment.OwnerReferences[0].UID).Should(Equal(reaper.GetUID()))

		By("check that the reaper is ready")
		Eventually(func() bool {
			key := types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName}
			updatedReaper := &api.Reaper{}
			if err := k8sClient.Get(context.Background(), key, updatedReaper); err != nil {
				return false
			}
			return updatedReaper.Status.Ready
		}, timeout, interval)
	})
})
