package framework

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"

	api "github.com/thelastpickle/reaper-operator/api/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	OperatorRetryInterval = 5 * time.Second
	OperatorTimeout = 30 * time.Second
)

var (
	Client client.Client
	initialized bool
)

func Init() {
	if initialized {
		return
	}
	err := api.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	apiConfig, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	Expect(err).ToNot(HaveOccurred())

	clientConfig := clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{})
	cfg, err := clientConfig.ClientConfig()

	Client, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(Client).ToNot(BeNil())

	initialized = true
}

func KustomizeAndApply(namespace, dir string) {
	path, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	kustomizeDir := filepath.Clean(path + "/../config/" + dir)

	kustomize := exec.Command("kustomize", "build", kustomizeDir)
	var stdout, stderr bytes.Buffer
	kustomize.Stdout = &stdout
	kustomize.Stderr = &stderr
	err = kustomize.Run()
	Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("kustomize build failed: %s", err))


	kubectl := exec.Command("kubectl", "-n", namespace, "apply", "-f", "-")
	kubectl.Stdin = &stdout
	out, err := kubectl.CombinedOutput()
	GinkgoWriter.Write(out)
	Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("kubectl apply failed: %s", err))
}

func ApplyFile(namespace, file string) {
	var stdout bytes.Buffer

	kubectl := exec.Command("kubectl", "-n", namespace, "apply", "-f", file)
	kubectl.Stdin = &stdout
	out, err := kubectl.CombinedOutput()
	GinkgoWriter.Write(out)
	Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("kubectl apply failed: %s", err))
}

func CreateNamespace(name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := Client.Create(context.Background(), namespace); err != nil {
		return fmt.Errorf("failed to create namespace %s: %s", name, err)
	}
	return nil
}

func WaitForDeploymentReady(key types.NamespacedName, readyReplicas int32, retryInterval, timeout time.Duration) error {
	return wait.Poll(retryInterval, timeout, func() (bool, error) {
		deployment := &appsv1.Deployment{}
		err := Client.Get(context.Background(), key, deployment)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return true, nil
		}
		return deployment.Status.ReadyReplicas == readyReplicas, nil
	})
}

func WaitForCassOperatorReady(namespace string) error {
	key := types.NamespacedName{Namespace: namespace, Name: "cass-operator"}
	return WaitForDeploymentReady(key, 1, OperatorRetryInterval, OperatorTimeout)
}

func WaitForReaperOperatorReady(namespace string) error {
	key := types.NamespacedName{Namespace: namespace, Name: "controller-manager"}
	return WaitForDeploymentReady(key, 1, OperatorRetryInterval, OperatorTimeout)
}

// Returns s with a date suffix of -yyMMddHHmmss
func WithDateSuffix(s string) string {
	return s + "-" + time.Now().Format("060102150405")
}

