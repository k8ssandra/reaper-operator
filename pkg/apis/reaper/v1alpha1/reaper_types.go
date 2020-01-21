package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	DefaultHangingRepairTimeoutMins = 30
	DefaultRepairIntensity = "0.9"
	DefaultRepairParallelism = "DATACENTER_AWARE"
	DefaultRepairRunThreadCount = 15
	DefaultScheduleDaysBetween = 7
	DefaultStorageType = "memory"
)

type ServerConfig struct {
	AutoScheduling AutoScheduling `json:"autoScheduling,omitempty" yaml:"autoScheduling,omitempty"`

	StorageType string `json:"storageType,omitempty" yaml:"storageType,omitempty"`

	// The amount of time in minutes to wait for a single repair to finish. Defaults to 30. If this timeout is reached,
	// the repair segment in question will be cancelled, if possible, and then scheduled for later repair again within
	// the same repair run process.
	HangingRepairTimeoutMins *int32 `json:"hangingRepairTimeoutMins,omitempty" yaml:"hangingRepairTimeoutMins,omitempty"`

	// Sets the default repair type unless specifically defined for each run. Note that this is only supported with the
	// PARALLEL repairParallelism setting. For more details in incremental repair, please refer to the following
	// article http://www.datastax.com/dev/blog/more-efficient-repairs
	//
	// Note: It is recommended to avoid using incremental repair before Cassandra 4.0 as subtle bugs can lead to
	// overstreaming and cluster instabililty
	IncrementalRepair bool `json:"incrementalRepair,omitempty" yaml:"incrementalRepair"`
	//IncrementalRepair bool `json:"incrementalRepair,omitempty" yaml:"incrementalRepair,omitempty"`

	// Repair intensity defines the amount of time to sleep between triggering each repair segment while running a
	// repair run. When intensity is 1.0, it means that Reaper doesn’t sleep at all before triggering next segment, and
	// otherwise the sleep time is defined by how much time it took to repair the last segment divided by the intensity
	// value. 0.5 means half of the time is spent sleeping, and half running. Intensity 0.75 means that 25% of the
	// total time is used sleeping and 75% running. This value can also be overwritten per repair run when invoking
	// repairs.
	//
	// Defaults to 0.9.
	// TODO add validation to ensure the value is a number
	RepairIntensity string `json:"repairIntensity,omitempty" yaml:"repairIntensity,omitempty"`

	// Type of parallelism to apply by default to repair runs. The value must be either SEQUENTIAL, PARALLEL, or
	// DATACENTER_AWARE.
	//
	// Defaults to DATACENTER_AWARE
	RepairParallelism string `json:"repairParallelism,omitempty" yaml:"repairParallelism,omitempty"`

	// The amount of threads to use for handling the Reaper tasks. Have this big enough not to cause blocking in cause
	// some thread is waiting for I/O, like calling a Cassandra cluster through JMX.
	//
	// Defaults to 15
	RepairRunThreadCount *int32 `json:"repairRunThreadCount,omitempty" yaml:"repairRunThreadCount,omitempty"`

	// Defines the amount of days to wait between scheduling new repairs. The value configured here is the default for
	// new repair schedules, but you can also define it separately for each new schedule. Using value 0 for continuous
	// repairs is also supported.
	//
	// Defaults to 7
	ScheduleDaysBetween *int32 `json:"scheduleDaysBetween,omitempty" yaml:"scheduleDaysBetween,omitempty"`
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
	ExcludedKeyspaces []string `json:"excludedKeyspaces,omitempty" yaml:"excludedKeyspaces,omitempty"`
}

// ReaperSpec defines the desired state of Reaper
type ReaperSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	ServerConfig ServerConfig `json:"serverConfig,omitempty" yaml:"serverConfig,omitempty"`
}

// ReaperStatus defines the observed state of Reaper
type ReaperStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Total number of non-terminated pods targeted by this deployment (their labels match the selector).
	Replicas int32 `json:"replicas,omitempty"`

	// Total number of non-terminated pods targeted by this deployment that have the desired template spec.
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// Total number of ready pods targeted by this deployment.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Total number of available pods (ready for at least minReadySeconds) targeted by this deployment.
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Reaper is the Schema for the reapers API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=reapers,scope=Namespaced
type Reaper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReaperSpec   `json:"spec,omitempty"`
	Status ReaperStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReaperList contains a list of Reaper
type ReaperList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Reaper `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Reaper{}, &ReaperList{})
}
