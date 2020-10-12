package e2e

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/thelastpickle/reaper-operator/test/framework"
	"k8s.io/apimachinery/pkg/types"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	defaultRetryInterval = 30 * time.Second
	defaultTimeout = 3 * time.Minute

	// kustomize currently does not support setting fields dynamically at runtime from the
	// command line. For now the namespace has to match the namespace declared in the
	// test/config/test_dir/kustomization.yaml file. See
	//https://github.com/kubernetes-sigs/kustomize/issues/1113 for more info.
	namespace = "deploy-reaper-test"
)

var (
	namespaceBase = "reaper-cass-backend"
)

var _ = Describe("Deploy Reaper with Cassandra backend", func() {
	Context("When a Cassandra cluster is deployed", func() {
		Specify("Reaper is deployed", func() {
			By("create namespace " + namespace)
			err := framework.CreateNamespace(namespace)
			Expect(err).ToNot(HaveOccurred())

			By("deploy cass-operator and reaper-operator")
			framework.KustomizeAndApply(namespace, "deploy_reaper_test")

			By("wait for cass-operator to be ready")
			cassOperatorKey := types.NamespacedName{Namespace: namespace, Name: "cass-operator"}
			err = framework.WaitForDeploymentReady(cassOperatorKey, 1, defaultRetryInterval, defaultTimeout)
			Expect(err).ToNot(HaveOccurred(), "failed waiting for cass-operator to become ready")

			By("wait for reaper-operator to be ready")
			reaperOperatorKey := types.NamespacedName{Namespace: namespace, Name: "controller-manager"}
			err = framework.WaitForDeploymentReady(reaperOperatorKey, 1, defaultRetryInterval, defaultTimeout)
			Expect(err).ToNot(HaveOccurred(), "failed waiting for reaper-operator to become ready")
		})
	})
})
