package e2eutil

import (
	goctx "context"
	v1alpha1 "github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"testing"
	"time"
)

type ReaperConditionFunc func(reaper *v1alpha1.Reaper) (bool, error)

func WaitForOperatorDeployment(t *testing.T,
	f *framework.Framework,
	namespace string,
	name string,
	retryInterval time.Duration,
	timeout time.Duration,) error {

	return e2eutil.WaitForDeployment(t, f.KubeClient, namespace, name, 1, retryInterval, timeout)
}

func WaitForReaper(t *testing.T,
	f *framework.Framework,
	namespace string,
	name string,
	retryInterval time.Duration,
	timeout time.Duration) error {

	return wait.Poll(retryInterval, timeout, func() (bool, error) {
		reaper := &v1alpha1.Reaper{}

		if err := f.Client.Get(goctx.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, reaper); err!= nil {
			if apierrors.IsNotFound(err) {
				t.Logf("Reaper %s not found\n", name)
				return false, nil
			}
			return false, nil
		}
		if reaper.Status.ReadyReplicas == 0 {
			t.Logf("Waiting for Reaper %s\n", name)
			return false, nil
		}

		return true, nil
	})
}
