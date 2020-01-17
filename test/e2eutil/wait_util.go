package e2eutil

import (
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"testing"
	"time"
)

func WaitForOperatorDeployment(t *testing.T,
	f *framework.Framework,
	namespace string,
	name string,
	retryInterval time.Duration,
	timeout time.Duration,) error {

	return e2eutil.WaitForDeployment(t, f.KubeClient, namespace, name, 1, retryInterval, timeout)
}
