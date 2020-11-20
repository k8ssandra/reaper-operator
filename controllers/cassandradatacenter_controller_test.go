package controllers

import (
	"context"
	cassdcapi "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CassandraDatacenterName = "dc1"
	ControllerTestNamespace = "dc-test"
)

var _ = Describe("Verify functionality of CassandraDatacenterReconciler", func() {
	Specify("..", func() {
		testNamespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ControllerTestNamespace,
			},
		}
		Expect(k8sClient.Create(context.Background(), testNamespace)).Should(Succeed())
		reaper := createReaper(ControllerTestNamespace)
		Expect(k8sClient.Create(context.Background(), reaper)).Should(Succeed())

		testDc := &cassdcapi.CassandraDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      CassandraDatacenterName,
				Namespace: ControllerTestNamespace,
				Annotations: map[string]string{
					"reaper.cassandra-reaper.io/instance": "notHere",
				},
			},
			Spec: cassdcapi.CassandraDatacenterSpec{
				ClusterName:   "test-dc",
				ServerType:    "cassandra",
				ServerVersion: "3.11.7",
				Size:          3,
			},
			Status: cassdcapi.CassandraDatacenterStatus{
				CassandraOperatorProgress: cassdcapi.ProgressReady,
				Conditions: []cassdcapi.DatacenterCondition{
					{
						Status: corev1.ConditionTrue,
						Type:   cassdcapi.DatacenterReady,
					},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), testDc)).Should(Succeed())

		By("update annotation to target existing reaper instance")
		testDcPatch := client.MergeFrom(testDc.DeepCopy())
		testDc.Annotations = map[string]string{
			"reaper.cassandra-reaper.io/instance": reaper.Name,
		}
		Expect(k8sClient.Patch(context.Background(), testDc, testDcPatch)).Should(Succeed())

		By("make Reaper ready")
		reaper.Status.Ready = true
		reaper.Status.Clusters = append(reaper.Status.Clusters, "test-dc")
		k8sClient.Status().Update(context.Background(), reaper)
		verifyReaperReady(types.NamespacedName{Namespace: ControllerTestNamespace, Name: ReaperName})
	})
})
