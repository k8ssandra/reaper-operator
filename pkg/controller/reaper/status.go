package reaper

import (
	"github.com/thelastpickle/reaper-operator/pkg/apis/reaper/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConditionReason string

type ConditionMessage string

const (
	ConfigurationUpdatedReason = "ConfigurationUpdated"
	RestartRequiredReason = "RestartRequired"
	ConfigurationUpdateCompleteReason = "ConfigurationUpdateComplete"


	ConfigurationUpdatedMessage = "Reaper configuration has been updated"
	RestartRequiredMessage = "Reaper restart required for changes to take effect"
	ConfigurationUpdateCompleteMessage = "Reaper has been restarted and configuration changes take effect"
)

func NewCondition(condType v1alpha1.ReaperConditionType, status corev1.ConditionStatus, reason ConditionReason, message ConditionMessage) v1alpha1.ReaperCondition {
	return v1alpha1.ReaperCondition{
		Type: condType,
		Status: status,
		LastTransitionTime: metav1.Now(),
		Reason: string(reason),
		Message: string(message),
	}
}

func GetCondition(status *v1alpha1.ReaperStatus, condType v1alpha1.ReaperConditionType) *v1alpha1.ReaperCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

func SetCondition(status *v1alpha1.ReaperStatus, cond v1alpha1.ReaperCondition) {
	currentCond := GetCondition(status, cond.Type)
	if currentCond != nil && currentCond.Status == cond.Status {
		cond.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := filterOutCondition(status.Conditions, cond.Type)
	status.Conditions = append(newConditions, cond)
}

func RemoveCondition(status *v1alpha1.ReaperStatus, condType v1alpha1.ReaperConditionType) {
	status.Conditions = filterOutCondition(status.Conditions, condType)
}

func filterOutCondition(conditions []v1alpha1.ReaperCondition, condType v1alpha1.ReaperConditionType) []v1alpha1.ReaperCondition {
	var newConditions []v1alpha1.ReaperCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

func CalculateStatus(status *v1alpha1.ReaperStatus, deployment *appsv1.Deployment) {
	status.AvailableReplicas = deployment.Status.AvailableReplicas
	status.ReadyReplicas = deployment.Status.ReadyReplicas
	status.Replicas = deployment.Status.Replicas
	status.UpdatedReplicas = deployment.Status.UpdatedReplicas

	// If Reaper is ready check to see if there have been any changes that will require
	// a restart.
	if IsReady(status) {
		cond := GetCondition(status, v1alpha1.ConfigurationUpdated)
		// If restart is needed then update the condition to indicate a restart is necessary
		if cond != nil {
			if cond.Status == corev1.ConditionTrue && cond.Reason == ConfigurationUpdatedReason {
				// When the configuration is updated, we need to restart Reaper for changes to take effect
				newCond := NewCondition(v1alpha1.ConfigurationUpdated, corev1.ConditionTrue, RestartRequiredReason, RestartRequiredMessage)
				SetCondition(status, newCond)
			} else if cond.Status == corev1.ConditionTrue && cond.Reason == RestartRequiredReason {
				// The restart has completed
				newCond := NewCondition(v1alpha1.ConfigurationUpdated, corev1.ConditionTrue, ConfigurationUpdateCompleteReason, ConfigurationUpdateCompleteMessage)
				SetCondition(status, newCond)
			} else if cond.Status == corev1.ConditionTrue && cond.Reason == ConfigurationUpdateCompleteReason {
				// The configuration update/restart is done so we can remove the condition
				RemoveCondition(status, v1alpha1.ConfigurationUpdated)
			}
		}
	}
}

func IsReady(status *v1alpha1.ReaperStatus) bool {
	return status.ReadyReplicas == status.Replicas
}

func IsRestartNeeded(status *v1alpha1.ReaperStatus) bool {
	cond := GetCondition(status, v1alpha1.ConfigurationUpdated)
	return cond != nil && cond.Status == corev1.ConditionTrue && cond.Reason == RestartRequiredReason
}

