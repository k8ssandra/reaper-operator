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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/thelastpickle/reaper-operator/api/v1alpha1"
	cassdcv1beta1 "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	reapergo "github.com/jsanda/reaper-client-go/reaper"
)

// CassandraDatacenterReconciler reconciles a CassandraDatacenter object
type CassandraDatacenterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cassandra.datastax.com,namespace="reaper-operator",resources=cassandradatacenters,verbs=get;list;watch;create

func (r *CassandraDatacenterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("cassandradatacenter", req.NamespacedName)

	instance := &cassdcv1beta1.CassandraDatacenter{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}

	cassdc := instance.DeepCopy()

	if reaperName, ok := cassdc.Annotations["reaper.cassandra-reaper.io/instance"]; ok {
		reaperKey := getReaperKey(reaperName, cassdc.Namespace)
		reaper := &api.Reaper{}

		err := r.Get(ctx, reaperKey, reaper)
		if err != nil {
			if errors.IsNotFound(err) {
				// It is possible that the Reaper has not been deployed yet or that it has
				// been deleted, or the annotation could specify an incorrect value.
				r.Log.Info("reaper instance not found", "reaper", reaperKey)
				return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
			} else {
				r.Log.Error(err, "failed to retrieve reaper instance", "reaper", reaperKey)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err
			}
		}

		if !reaper.Status.Ready {
			r.Log.Info("waiting for reaper to become ready", "reaper", reaperKey)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		// Include the namespace in case Reaper is deployed in a different namespace than
		// the CassandraDatacenter.
		reaperSvc := reaper.Name + "-reaper-service" + "." + reaper.Namespace
		restClient, err := reapergo.NewReaperClient(fmt.Sprintf("http://%s:8080", reaperSvc))
		if err != nil {
			r.Log.Error(err, "failed to create reaper rest client", "reaperService", reaperSvc)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}

		_, err = restClient.GetCluster(ctx, cassdc.Spec.ClusterName)

		if err == nil {
			// The cluster was found in Reaper, so our work here is done; however, we still
			// requeue the request to periodically check that the cluster has not be removed
			// from Reaper.
			return ctrl.Result{RequeueAfter: 30 * time.Minute}, nil
		}

		if err == reapergo.CassandraClusterNotFound {
			r.Log.Info("registering cluster with reaper", "reaper", reaperKey)
			if err = restClient.AddCluster(ctx, cassdc.Spec.ClusterName, cassdc.GetDatacenterServiceName()); err == nil {
				return ctrl.Result{RequeueAfter: 30 * time.Minute}, nil
			} else {
				r.Log.Error(err, "failed to register cluster with reaper", "reaper", reaperKey)
				return ctrl.Result{RequeueAfter: 30 *time.Second}, err
			}
		}
	}

	// The CassandraDatacenter does not have the annotation which means it is not using
	// Reaper to manage repairs. We requeue the request though to periodically check if
	// the cluster has been updated to be managed with Reaper.
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

func getReaperKey(instanceName, cassdcNamespace string) types.NamespacedName {
	parts := strings.Split(instanceName,".")
	if len(parts) == 1 {
		return types.NamespacedName{Namespace: cassdcNamespace, Name: instanceName}
	} else {
		return types.NamespacedName{Namespace: parts[1], Name: parts[0]}
	}
}

func (r *CassandraDatacenterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cassdcv1beta1.CassandraDatacenter{}).
		Complete(r)
}
