package framework

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	cassdcv1beta1 "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/thelastpickle/reaper-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OperatorRetryInterval = 5 * time.Second
	OperatorTimeout       = 30 * time.Second
)

var (
	Client      client.Client
	initialized bool
)

func Init() {
	if initialized {
		return
	}

	err := api.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = cassdcv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())

	apiConfig, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	Expect(err).ToNot(HaveOccurred())

	clientConfig := clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{})
	cfg, err := clientConfig.ClientConfig()

	Client, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(Client).ToNot(BeNil())

	initialized = true
}

func KustomizeAndApply(namespace, dir string) error {
	path, err := os.Getwd()
	if err != nil {
		return err
	}
	kustomizeDir := filepath.Clean(path + "/../config/" + dir)

	By("running kustomize build")
	kustomize := exec.Command("kustomize", "build", kustomizeDir)
	var stdout, stderr bytes.Buffer
	kustomize.Stdout = &stdout
	kustomize.Stderr = &stderr
	GinkgoWriter.Write([]byte(fmt.Sprintf("kustomize exit code: %d", kustomize.ProcessState.ExitCode())))
	if err = kustomize.Run(); err != nil {
		return err
	}

	//Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("kustomize build failed: %s", err))

	By("running kubectl apply")
	kubectl := exec.Command("kubectl", "-n", namespace, "apply", "-f", "-")
	kubectl.Stdin = &stdout
	out, err := kubectl.CombinedOutput()
	GinkgoWriter.Write([]byte(fmt.Sprintf("kubectl exit code: %d", kubectl.ProcessState.ExitCode())))
	GinkgoWriter.Write(out)
	//Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("kubectl apply failed: %s", err))

	return err
}

func ApplyFile(namespace, file string) error {
	var stdout bytes.Buffer

	kubectl := exec.Command("kubectl", "-n", namespace, "apply", "-f", file)
	kubectl.Stdin = &stdout
	out, err := kubectl.CombinedOutput()
	GinkgoWriter.Write(out)
	
	return err
	//Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("kubectl apply failed: %s", err))
}

func CreateNamespace(name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	err := Client.Create(context.Background(), namespace)
	// TODO I think it safer to fail if the namespace exists. Add a flag to continue if it already exists.
	if err == nil || apierrors.IsAlreadyExists(err) {
		return nil
	}
	return fmt.Errorf("failed to create namespace %s: %s", name, err)
}

// Blocks until .Status.ReadyReplicas == readyReplicas or until timeout is reached. An error is returned
// if fetching the Deployment fails.
func WaitForDeploymentReady(key types.NamespacedName, readyReplicas int32, retryInterval, timeout time.Duration) error {
	return wait.Poll(retryInterval, timeout, func() (bool, error) {
		deployment := &appsv1.Deployment{}
		err := Client.Get(context.Background(), key, deployment)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return true, err
		}
		return deployment.Status.ReadyReplicas == readyReplicas, nil
	})
}

// Blocks until the cass-operator Deployment is ready. This function assumes that there will be a
// single replica in the Deployment.
func WaitForCassOperatorReady(namespace string) error {
	key := types.NamespacedName{Namespace: namespace, Name: "cass-operator"}
	return WaitForDeploymentReady(key, 1, OperatorRetryInterval, OperatorTimeout)
}

// Blocks until the reaper-operator deployment is ready. This function assumes that there will be
// a single replica in the Deployment.
func WaitForReaperOperatorReady(namespace string) error {
	key := types.NamespacedName{Namespace: namespace, Name: "controller-manager"}
	return WaitForDeploymentReady(key, 1, OperatorRetryInterval, OperatorTimeout)
}

// Blocks until the CassandraDatacenter is ready as determined by
// .Status.CassandraOperatorProgress == ProgressReady or until timeout is reached. An error is returned
// is fetching the CassandraDatacenter fails.
func WaitForCassDcReady(key types.NamespacedName, retryInterval, timeout time.Duration) error {
	start := time.Now()
	return wait.Poll(retryInterval, timeout, func() (bool, error) {
		cassdc, err := GetCassDc(key)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return true, err
		}
		logCassDcStatus(cassdc, start)
		return cassdc.Status.CassandraOperatorProgress == cassdcv1beta1.ProgressReady, nil
	})
}

func GetCassDc(key types.NamespacedName) (*cassdcv1beta1.CassandraDatacenter, error) {
	cassdc := &cassdcv1beta1.CassandraDatacenter{}
	err := Client.Get(context.Background(), key, cassdc)

	return cassdc, err
}

func logCassDcStatus(cassdc *cassdcv1beta1.CassandraDatacenter, start time.Time) {
	if d, err := yaml.Marshal(cassdc.Status); err == nil {
		duration := time.Now().Sub(start)
		sec := int(duration.Seconds())
		s := fmt.Sprintf("cassdc status after %d sec...\n%s\n\n", sec, string(d))
		GinkgoWriter.Write([]byte(s))
	} else {
		log.Printf("failed to log cassdc status: %s", err)
	}
}

func WaitForReaperReady(key types.NamespacedName, retryInterval, timeout time.Duration) error {
	return wait.Poll(retryInterval, timeout, func() (bool, error) {
		reaper := &api.Reaper{}
		err := Client.Get(context.Background(), key, reaper)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return true, err
		}
		return reaper.Status.Ready, nil
	})
}

// Returns s with a date suffix of -yyMMddHHmmss
func WithDateSuffix(s string) string {
	return s + "-" + time.Now().Format("060102150405")
}
