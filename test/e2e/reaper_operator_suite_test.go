package e2e

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/k8ssandra/reaper-operator/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

func TestOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"reaper-operator Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	framework.Init()
	close(done)
}, 60)
