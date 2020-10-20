package reconcile

import (
	"context"
	"fmt"

	api "github.com/thelastpickle/reaper-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	JmxAuthSecretName = "reaper-jmx"
)

type SecretsManager interface {
	GetJmxAuthCredentials(reaper *api.Reaper) (*corev1.EnvVar, *corev1.EnvVar, error)
}

type defaultSecretsManager struct {
	client.Client
}

func NewSecretsManager(client client.Client) SecretsManager {
	return &defaultSecretsManager{Client: client}
}

func (s *defaultSecretsManager) GetJmxAuthCredentials(reaper *api.Reaper) (*corev1.EnvVar, *corev1.EnvVar, error) {
	key := types.NamespacedName{Namespace: reaper.Namespace, Name: reaper.Spec.ServerConfig.JmxUserSecretName}
	secret := &corev1.Secret{}

	if err := s.Get(context.Background(), key, secret); err == nil {
		if _, ok := secret.Data["username"]; !ok {
			return nil, nil, fmt.Errorf("username key not found in jmx auth secret %s", secret.Name)
		}

		if _, ok := secret.Data["password"]; !ok {
			return nil, nil, fmt.Errorf("password key not found in jmx auth secret %s", secret.Name)
		}

		usernameEnvVar := corev1.EnvVar{
			Name: "REAPER_JMX_AUTH_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret.Name,
					},
					Key: "username",
				},
			},
		}

		passwordEnvVar := corev1.EnvVar{
			Name: "REAPER_JMX_AUTH_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret.Name,
					},
					Key: "password",
				},
			},
		}

		return &usernameEnvVar, &passwordEnvVar, nil
	} else {
		return nil, nil, err
	}
}
