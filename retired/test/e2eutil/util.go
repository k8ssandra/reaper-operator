package e2eutil

import (
framework "github.com/operator-framework/operator-sdk/pkg/test"
"testing"
"time"
)

var (
	RetryInterval        = time.Second * 5
	Timeout              = time.Second * 60
	CleanupRetryInterval = time.Second * 1
	CleanupTimeout       = time.Second * 30
)

func InitOperator(t *testing.T) (*framework.TestCtx, *framework.Framework) {
	ctx := framework.NewTestCtx(t)

	err := ctx.InitializeClusterResources(&framework.CleanupOptions{
		TestContext:   ctx,
		Timeout:       CleanupTimeout,
		RetryInterval: CleanupRetryInterval,
	})
	if err != nil {
		t.Fatalf("failed to initialize cluster resources: %v", err)
	}
	t.Log("Initialized cluster resources")
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}
	// get global framework variables
	f := framework.Global
	// wait for operator to be ready
	err = WaitForOperatorDeployment(t, f, namespace, "reaper-operator", RetryInterval, Timeout)
	if err != nil {
		t.Fatalf("Failed waiting for reaper-operator deployment: %s\n", err)
	}

	return ctx, f
}


