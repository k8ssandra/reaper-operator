package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ServerConfig struct {
	StorageType string `json:"storageType,omitempty" yaml:"storageType,omitempty"`
}

type AutoScheduling struct {
	// Enables or disables auto scheduling
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// The duration of the delay before the schedule period starts. Defaults to PT15S (15 seconds)
	InitialDelayPeriod string `json:"initialDelayPeriod,omitempty" yaml:"initialDelayPeriod,omitempty"`

	// The amount of time to wait before checking whether or not to start a repair task. Defaults to PT10M (10 minutes)
	PeriodBetweenPolls string `json:"periodBetweenPolls,omitempty" yaml:"periodBetweenPolls,omitempty"`

	// Grace period before the first repair in the schedule is started. Defaults to PT5M (5 minutes)
	TimeBeforeFirstSchedule string `json:"timeBeforeFirstSchedule,omitempty" yaml:"timeBeforeFirstSchedule,omitempty"`

	// The time spacing between each of the repair schedules that is to be carried out. Defaults to PT6H (6 hours)
	ScheduleSpreadPeriod string `json:"scheduleSpreadPeriod,omitempty" yaml:"scheduleSpreadPeriod,omitempty"`

	// The keyspaces that are to be excluded from the repair schedule.
	ExcludedKeyspaces []string `json:"excludedKeyspaces" yaml:"excludedKeyspaces"`
}

// CassandraReaperSpec defines the desired state of CassandraReaper
type CassandraReaperSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	ServerConfig ServerConfig `json:"serverConfig,omitempty"`
}

// CassandraReaperStatus defines the observed state of CassandraReaper
type CassandraReaperStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CassandraReaper is the Schema for the cassandrareapers API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=cassandrareapers,scope=Namespaced
type CassandraReaper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CassandraReaperSpec   `json:"spec,omitempty"`
	Status CassandraReaperStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CassandraReaperList contains a list of CassandraReaper
type CassandraReaperList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CassandraReaper `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CassandraReaper{}, &CassandraReaperList{})
}
