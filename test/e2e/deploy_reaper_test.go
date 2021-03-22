package e2e

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/k8ssandra/reaper-operator/api/v1alpha1"
	"github.com/k8ssandra/reaper-operator/test/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	defaultRetryInterval = 30 * time.Second
	defaultTimeout       = 3 * time.Minute

	// kustomize currently does not support setting fields dynamically at runtime from the
	// command line. For now the namespace has to match the namespace declared in the
	// test/config/test_dir/kustomization.yaml file. See
	//https://github.com/kubernetes-sigs/kustomize/issues/1113 for more info.
	namespace = "deploy-reaper-test"

	// If you change this, you will also need to update the selector in test/config/deploy_reaper_test/nodeport-service.yaml
	reaperName = "cass-backend"
)

var _ = Describe("Deploy Reaper with Cassandra backend", func() {
	Context("When a Cassandra cluster is deployed", func() {
		Specify("Reaper is deployed", func() {
			By("create namespace " + namespace)
			err := framework.CreateNamespace(namespace)
			Expect(err).ToNot(HaveOccurred())

			By("deploy cass-operator, reaper-operator and cassdc")
			framework.KustomizeAndApply(namespace, "deploy_reaper_test", "k8ssandra")
			// The first apply fails often for some reason, redo just to avoid flakiness of the test
			framework.KustomizeAndApply(namespace, "deploy_reaper_test", "k8ssandra")
			cassdcKey := types.NamespacedName{Namespace: namespace, Name: "reaper-test"}

			By("wait for reaper-operator to be ready")
			err = framework.WaitForReaperOperatorReady(namespace)
			Expect(err).ToNot(HaveOccurred(), "failed waiting for reaper-operator to become ready")

			By("deploy reaper")
			reaper := &api.Reaper{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      reaperName,
				},
				Spec: api.ReaperSpec{
					ServerConfig: api.ServerConfig{
						JmxUserSecretName: "reaper-jmx",
						StorageType:       api.StorageTypeCassandra,
						CassandraBackend: &api.CassandraBackend{
							CassandraDatacenter: api.CassandraDatacenterRef{
								Name: cassdcKey.Name,
							},
						},
					},
				},
			}

			err = framework.Client.Create(context.Background(), reaper)
			Expect(err).ToNot(HaveOccurred(), "failed to create reaper object")

			By("wait for cass-operator to be ready")
			err = framework.WaitForCassOperatorReady(namespace)
			Expect(err).ToNot(HaveOccurred(), "failed waiting for cass-operator to become ready")

			By("wait for cassdc to be ready")
			cassdcRetryInterval := 15 * time.Second
			cassdcTimeout := 7 * time.Minute
			err = framework.WaitForCassDcReady(cassdcKey, cassdcRetryInterval, cassdcTimeout)
			Expect(err).ToNot(HaveOccurred(), "failed waiting for cassdc to become ready")

			By("wait for reaper to become ready")
			reaperKey := types.NamespacedName{Namespace: reaper.Namespace, Name: reaper.Name}
			err = framework.WaitForReaperReady(reaperKey, 10*time.Second, 3*time.Minute)
			Expect(err).ToNot(HaveOccurred(), "failed waiting for reaper to become ready")

			// This should be ready at the same time as cassdc

			By("wait for the cluster to get registered with reaper")
			err = framework.WaitForReaper(reaperKey, 10*time.Second, 3*time.Minute, func(reaper *api.Reaper) bool {
				return len(reaper.Status.Clusters) == 1
			})
			Expect(err).ToNot(HaveOccurred(), "failing waiting for cluster to get registered")

			// Remove Cluster and see that it's being readded
			By("removing cluster status and expecting it to reappear")
			err = framework.Client.Get(context.Background(), reaperKey, reaper)
			Expect(err).ToNot(HaveOccurred(), "failed to refresh reaper object")

			reaperPatch := client.MergeFrom(reaper.DeepCopy())
			reaper.Status.Clusters = make([]string, 0)
			err = framework.Client.Status().Patch(context.Background(), reaper, reaperPatch)

			err = framework.WaitForReaper(reaperKey, 10*time.Second, 3*time.Minute, func(reaper *api.Reaper) bool {
				return len(reaper.Status.Clusters) == 0
			})
			Expect(err).ToNot(HaveOccurred(), "failing waiting for clusters to get removed")

			err = framework.WaitForReaper(reaperKey, 10*time.Second, 3*time.Minute, func(reaper *api.Reaper) bool {
				return len(reaper.Status.Clusters) == 1
			})
			Expect(err).ToNot(HaveOccurred(), "failing waiting for clusters to get re-registered")
		})

		Specify("More clusters are created", func() {
			// Since we didn't clean the previous resources, we don't need to redeploy everything
			By("starting second-cluster")
			framework.KustomizeAndApply(namespace, "deploy_reaper_test", "second-cluster")

			cassdcKey := types.NamespacedName{Namespace: namespace, Name: "secondary-reaper-test"}

			By("waiting for secondary-reaper-test cluster to be ready")
			cassdcRetryInterval := 15 * time.Second
			cassdcTimeout := 7 * time.Minute
			err := framework.WaitForCassDcReady(cassdcKey, cassdcRetryInterval, cassdcTimeout)
			Expect(err).ToNot(HaveOccurred(), "failed waiting for cassdc to become ready")

			By("waiting for the second cluster to get registered with reaper")
			reaperKey := types.NamespacedName{Namespace: namespace, Name: reaperName}
			err = framework.WaitForReaper(reaperKey, 10*time.Second, 3*time.Minute, func(reaper *api.Reaper) bool {
				return len(reaper.Status.Clusters) == 2
			})
			Expect(err).ToNot(HaveOccurred(), "failing waiting for cluster to get registered")
		})
	})
})

var _ = AfterSuite(func() {
	// Clean up the deploy-reaper-test namespace after the tests
	// CassandraDatacenters are deleted first so that cass-operator can remove the finalizers
	Expect(framework.RemoveCassandraDatacenters(namespace)).Should(Succeed())
	// Removing the namespace should be enough to remove rest of the resources (CRDs will be left behind..)
	Expect(framework.RemoveNamespace(namespace)).Should(Succeed())
}, 60)
