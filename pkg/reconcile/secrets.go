package reconcile

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	JmxAuthSecretName = "reaper-jmx"
)

type SecretsManager interface {
	GetJmxAuthCredentials(secret *corev1.Secret) (*corev1.EnvVar, *corev1.EnvVar, error)
}

type defaultSecretsManager struct {
}

func NewSecretsManager() SecretsManager {
	return &defaultSecretsManager{}
}

func (s *defaultSecretsManager) GetJmxAuthCredentials(secret *corev1.Secret) (*corev1.EnvVar, *corev1.EnvVar, error) {
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
}
