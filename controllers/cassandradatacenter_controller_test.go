package controllers

import (
	"context"
	"fmt"
	cassdcapi "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/reaper-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CassandraDatacenterName = "dc1"
	ControllerTestNamespace = "dc-test"
)

var _ = Describe("CassandraDatacenter controller testing", func() {
	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	It("checking Reaper cluster status", func() {
		By("expecting CassandraDatacenter creation")
		testNamespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ControllerTestNamespace,
			},
		}
		Expect(k8sClient.Create(context.Background(), testNamespace)).Should(Succeed())
		reaper := createReaper(ControllerTestNamespace)
		Expect(k8sClient.Create(context.Background(), reaper)).Should(Succeed())

		cassdcKey := types.NamespacedName{
			Name:      CassandraDatacenterName,
			Namespace: ControllerTestNamespace,
		}

		testDc := &cassdcapi.CassandraDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:        cassdcKey.Name,
				Namespace:   cassdcKey.Namespace,
				Annotations: map[string]string{},
			},
			Spec: cassdcapi.CassandraDatacenterSpec{
				ClusterName:   "test-dc",
				ServerType:    "cassandra",
				ServerVersion: "3.11.7",
				Size:          3,
			},
			Status: cassdcapi.CassandraDatacenterStatus{
				CassandraOperatorProgress: cassdcapi.ProgressReady,
				Conditions: []cassdcapi.DatacenterCondition{
					{
						Status: corev1.ConditionTrue,
						Type:   cassdcapi.DatacenterReady,
					},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), testDc)).Should(Succeed())
		Eventually(func() bool {
			result := &cassdcapi.CassandraDatacenter{}
			_ = k8sClient.Get(context.Background(), cassdcKey, result)
			return result.Spec.Size == testDc.Spec.Size
		}, timeout, interval).Should(BeTrue())

		By("making Reaper ready")
		patch := client.MergeFrom(reaper.DeepCopy())
		reaper.Status.Ready = true
		k8sClient.Status().Patch(context.Background(), reaper, patch)
		verifyReaperReady(types.NamespacedName{Namespace: ControllerTestNamespace, Name: ReaperName})

		By("verifying reaper is not targeting the CassandraDatacenter")
		Consistently(func() bool {
			if len(reaperManager.ClustersManaged) > 0 {
				return false
			}

			key := types.NamespacedName{Namespace: reaper.Namespace, Name: ReaperName}
			fetchedReaper := &api.Reaper{}
			if err := k8sClient.Get(context.Background(), key, fetchedReaper); err != nil {
				return false
			}
			return len(fetchedReaper.Status.Clusters) == 0
		}, timeout, interval).Should(BeTrue())

		By("updating reaper annotation to point to non-existing reaper instance")
		testDcPatch := client.MergeFrom(testDc.DeepCopy())
		testDc.Annotations = map[string]string{
			"reaper.cassandra-reaper.io/instance": "notHere",
		}
		Expect(k8sClient.Patch(context.Background(), testDc, testDcPatch)).Should(Succeed())
		Eventually(func() bool {
			result := &cassdcapi.CassandraDatacenter{}
			_ = k8sClient.Get(context.Background(), cassdcKey, result)
			v, _ := result.Annotations["reaper.cassandra-reaper.io/instance"]
			return v == "notHere"
		}).Should(BeTrue())

		By("verifying reaper is still not targeting the CassandraDatacenter")
		Consistently(func() bool {
			if len(reaperManager.ClustersManaged) > 0 || len(reaperManager.ClustersManaged) > 0 {
				return false
			}

			key := types.NamespacedName{Namespace: reaper.Namespace, Name: ReaperName}
			fetchedReaper := &api.Reaper{}
			if err := k8sClient.Get(context.Background(), key, fetchedReaper); err != nil {
				return false
			}
			return len(fetchedReaper.Status.Clusters) == 0
		}, timeout, interval).Should(BeTrue())

		By("updating annotation to target working reaper instance")
		testDcPatch = client.MergeFrom(testDc.DeepCopy())
		testDc.Annotations = map[string]string{
			"reaper.cassandra-reaper.io/instance": reaper.Name,
		}
		Expect(k8sClient.Patch(context.Background(), testDc, testDcPatch)).Should(Succeed())
		Eventually(func() bool {
			result := &cassdcapi.CassandraDatacenter{}
			_ = k8sClient.Get(context.Background(), cassdcKey, result)
			v, _ := result.Annotations["reaper.cassandra-reaper.io/instance"]
			return v == reaper.Name
		}, timeout, interval).Should(BeTrue())

		By("verifying reaper is now targeting the CassandraDatacenter, but fails")
		reaperManager.failMode = true
		Consistently(func() bool {
			key := types.NamespacedName{Namespace: reaper.Namespace, Name: ReaperName}
			fetchedReaper := &api.Reaper{}
			if err := k8sClient.Get(context.Background(), key, fetchedReaper); err != nil {
				return false
			}
			if len(fetchedReaper.Status.Clusters) == 0 && len(reaperManager.ClustersManaged) == 0 {
				return true
			}

			return false
		}, timeout, interval).Should(BeTrue())

		By("letting reaper succeed with the cluster adding")
		reaperManager.failMode = false
		Eventually(func() bool {
			key := types.NamespacedName{Namespace: reaper.Namespace, Name: ReaperName}
			fetchedReaper := &api.Reaper{}
			if err := k8sClient.Get(context.Background(), key, fetchedReaper); err != nil {
				return false
			}
			for _, v := range fetchedReaper.Status.Clusters {
				if v == testDc.Spec.ClusterName {
					// Verify Reaper knows it also
					for _, d := range reaperManager.ClustersManaged {
						if d == testDc.Spec.ClusterName {
							return true
						}
					}
					return false
				}
			}
			return false
		}, timeout, interval).Should(BeTrue())

		By("ensuring cluster status is re-added even if removed")
		// Remove cluster status from reaper
		patchClusters := client.MergeFrom(reaper.DeepCopy())
		reaper.Status.Clusters = make([]string, 0)
		k8sClient.Status().Patch(context.Background(), reaper, patchClusters)

		// Add unnecessary stuff to CassandraDatacenter to trigger reconcile
		testDcPatch = client.MergeFrom(testDc.DeepCopy())
		testDc.Annotations["useless/update"] = "true"
		Expect(k8sClient.Patch(context.Background(), testDc, testDcPatch)).Should(Succeed())

		// Wait for the cluster to be re-added
		Eventually(func() bool {
			key := types.NamespacedName{Namespace: reaper.Namespace, Name: ReaperName}
			fetchedReaper := &api.Reaper{}
			if err := k8sClient.Get(context.Background(), key, fetchedReaper); err != nil {
				return false
			}
			for _, v := range fetchedReaper.Status.Clusters {
				if v == testDc.Spec.ClusterName {
					return true
				}
			}
			return false
		}, timeout, interval).Should(BeTrue())
	})
})

type TestReaperManager struct {
	ClustersManaged []string
	failMode        bool
}

func (r *TestReaperManager) Connect(reaper *api.Reaper) error {
	r.ClustersManaged = make([]string, 0)
	return nil
}

func (r *TestReaperManager) AddClusterToReaper(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) error {
	if r.failMode {
		return fmt.Errorf("mocked failure of cluster adding")
	}
	r.ClustersManaged = append(r.ClustersManaged, cassdc.Spec.ClusterName)
	return nil
}

func (r *TestReaperManager) VerifyClusterIsConfigured(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) (bool, error) {
	for _, v := range r.ClustersManaged {
		if v == cassdc.Spec.ClusterName {
			return true, nil
		}
	}
	return false, nil
}
