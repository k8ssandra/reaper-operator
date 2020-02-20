package reaper

import (
	"context"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var logger = logf.Log.WithName("reconcile")

type ReaperConfigMapReconciler interface {
	// Called to reconcile the Reaper ConfigMap. Note that reconciliation will continue after calling this function
	// only when both return values are nil.
	ReconcileConfigMap(ctx context.Context, r *v1alpha1.Reaper) (*reconcile.Result, error)
}

type configMapReconciler struct {
	client client.Client

	scheme *runtime.Scheme
}

func NewConfigMapReconciler(c client.Client, s *runtime.Scheme) ReaperConfigMapReconciler {
	return &configMapReconciler{client: c, scheme: s}
}

func (r *configMapReconciler) ReconcileConfigMap(ctx context.Context, reaper *v1alpha1.Reaper) (*reconcile.Result, error) {
	reqLogger := log.WithValues("Reaper.Namespace", reaper.Namespace, "Reaper.Name", reaper.Name)
	reqLogger.Info("Reconciling configmap")
	serverConfig := &corev1.ConfigMap{}
	err := r.client.Get(ctx, types.NamespacedName{Name: reaper.Name, Namespace: reaper.Namespace}, serverConfig)
	if err != nil && errors.IsNotFound(err) {
		// create server config configmap
		cm, err := r.newServerConfigMap(reaper)
		if err != nil {
			reqLogger.Error(err, "Failed to create new ConfigMap")
			return &reconcile.Result{}, err
		}
		reqLogger.Info("Creating configmap", "ConfigMap.Name", cm.Name)
		if err = controllerutil.SetControllerReference(reaper, cm, r.scheme); err != nil {
			reqLogger.Error(err, "Failed to set owner reference on Reaper server config ConfigMap")
			return &reconcile.Result{}, err
		}
		if err = r.client.Create(ctx, cm); err != nil {
			reqLogger.Error(err, "Failed to save ConfigMap")
			return &reconcile.Result{}, err
		} else {
			return &reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get ConfigMap")
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *configMapReconciler) newServerConfigMap(reaper *v1alpha1.Reaper) (*corev1.ConfigMap, error) {
	output, err := yaml.Marshal(&reaper.Spec.ServerConfig)
	if err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      reaper.Name,
			Namespace: reaper.Namespace,
			Labels:    createLabels(reaper),
		},
		Data: map[string]string{
			"reaper.yaml": string(output),
		},
	}

	return cm, nil
}
