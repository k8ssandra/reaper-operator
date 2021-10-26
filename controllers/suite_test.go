/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/k8ssandra/reaper-operator/pkg/config"
	"github.com/k8ssandra/reaper-operator/pkg/reconcile"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/reaper-operator/api/v1alpha1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var reaperManager *TestReaperManager
var managementMockServer *httptest.Server

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))
	By("creating a fake mgtt-api-http")
	var err error
	// The client in cass-operator has hardcoded port of 8080, so we need to run our mgtt-api listener in that port
	mgttapiListener, err := net.Listen("tcp", "127.0.0.1:8080")
	Expect(err).To(BeNil())

	managementMockServer = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	managementMockServer.Listener.Close()
	managementMockServer.Listener = mgttapiListener
	managementMockServer.Start()

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("..", "test", "config", "cass-operator-crd", "bases"),
		},
	}

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = clientgoscheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = api.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = cassdcapi.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		// Disable metrics-server, since we need port 8080
		MetricsBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	reconcile.InitReconcilers(k8sManager.GetClient(), k8sManager.GetScheme())

	err = (&ReaperReconciler{
		Client:               k8sManager.GetClient(),
		Log:                  ctrl.Log.WithName("controllers").WithName("Reaper"),
		ServiceReconciler:    reconcile.GetServiceReconciler(),
		DeploymentReconciler: reconcile.GetDeploymentReconciler(),
		SchemaReconciler:     reconcile.GetSchemaReconciler(),
		Validator:            config.NewValidator(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	reaperManager = &TestReaperManager{}

	err = (&CassandraDatacenterReconciler{
		Client:        k8sManager.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("CassandraDatacenter"),
		ReaperManager: reaperManager,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	managementMockServer.Close()
	gexec.KillAndWait(5 * time.Second)
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
