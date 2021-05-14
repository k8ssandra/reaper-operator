package reconcile

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	JmxAuthSecretName       = "reaper-jmx"
	jmxAuthEnvPasswordName  = "REAPER_JMX_AUTH_PASSWORD"
	jmxAuthEnvUsernameName  = "REAPER_JMX_AUTH_USERNAME"
	cassAuthEnvPasswordName = "REAPER_CASS_AUTH_PASSWORD"
	cassAuthEnvUsernameName = "REAPER_CASS_AUTH_USERNAME"
	secretUsernameName      = "username"
	secretPasswordName      = "password"
)

type SecretsManager interface {
	GetJmxAuthCredentials(secret *corev1.Secret) (*corev1.EnvVar, *corev1.EnvVar, error)
	GetCassandraAuthCredentials(secret *corev1.Secret) (*corev1.EnvVar, *corev1.EnvVar, error)
}

type defaultSecretsManager struct {
}

func NewSecretsManager() SecretsManager {
	return &defaultSecretsManager{}
}

func (s *defaultSecretsManager) GetCassandraAuthCredentials(secret *corev1.Secret) (*corev1.EnvVar, *corev1.EnvVar, error) {
	return s.authCredentials(secret, cassAuthEnvUsernameName, cassAuthEnvPasswordName)
}

func (s *defaultSecretsManager) GetJmxAuthCredentials(secret *corev1.Secret) (*corev1.EnvVar, *corev1.EnvVar, error) {
	return s.authCredentials(secret, jmxAuthEnvUsernameName, jmxAuthEnvPasswordName)
}

func (s *defaultSecretsManager) authCredentials(secret *corev1.Secret, envUsernameParam, envPasswordParam string) (*corev1.EnvVar, *corev1.EnvVar, error) {
	if _, ok := secret.Data[secretUsernameName]; !ok {
		return nil, nil, fmt.Errorf("username key not found in jmx auth secret %s", secret.Name)
	}

	if _, ok := secret.Data[secretPasswordName]; !ok {
		return nil, nil, fmt.Errorf("password key not found in jmx auth secret %s", secret.Name)
	}

	usernameEnvVar := corev1.EnvVar{
		Name: envUsernameParam,
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
		Name: envPasswordParam,
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
