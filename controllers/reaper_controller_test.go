package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/thelastpickle/reaper-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Reaper controller", func() {
	const (
		ReaperName      = "test-reaper"
		ReaperNamespace = "default"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Specify("create a new Reaper instance", func() {
		By("create the Reaper object")
		reaper := &api.Reaper{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ReaperNamespace,
				Name:      ReaperName,
			},
		}
		Expect(k8sClient.Create(context.Background(), reaper)).Should(Succeed())

		By("check that the deployment is created")
		deploymentKey := types.NamespacedName{Namespace: ReaperNamespace, Name: ReaperName}
		deployment := &appsv1.Deployment{}

		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), deploymentKey, deployment)
			if err != nil {
				return false
			}
			return true
		}, timeout, interval)
	})
})
