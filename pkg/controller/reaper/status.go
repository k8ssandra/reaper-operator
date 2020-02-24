package reaper

import (
	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
)

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
