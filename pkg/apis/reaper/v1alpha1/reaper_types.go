package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type StorageType string

const (
	ReaperImage = "jsanda/reaper-k8s:2.0.2-b6bfb774ccbb"

	Memory = StorageType("memory")
	Cassandra = StorageType("cassandra")

	DefaultHangingRepairTimeoutMins = 30
	DefaultRepairIntensity = "0.9"
	DefaultRepairParallelism = "DATACENTER_AWARE"
	DefaultRepairRunThreadCount = 15
	DefaultScheduleDaysBetween = 7
	DefaultEnableCrossOrigin = true
	DefaultEnableDynamicSeedList = false
	DefaultJmxConnectionTimeoutInSeconds = 20
	DefaultSegmentCountPerNode = 16
	DefaultKeyspace = "reaper"
	DefaultStorageType = Memory
)

type ServerConfig struct {
	AutoScheduling AutoScheduling `json:"autoScheduling,omitempty" yaml:"autoScheduling,omitempty"`

	StorageType StorageType `json:"storageType,omitempty" yaml:"storageType,omitempty"`

	CassandraBackend *CassandraBackend `json:"cassandraBackend,omitempty" yaml:"cassandra,omitempty"`

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

	// Optional setting which can be used to enable the CORS headers for running an external GUI application. When
	// enabled it will allow REST requests incoming from other origins than the domain that hosts Reaper.
	//
	// Defaults to true
	EnableCrossOrigin *bool `json:"enableCrossOrigin,omitempty" yaml:"enableCrossOrigin,omitempty"`

	// Allow Reaper to add all nodes in the cluster as contact points when adding a new cluster, instead of just
	//adding the provided node.
	//
	// Defaults to true
	EnableDynamicSeedList *bool `json:"enableDynamicSeedList,omitempty" yaml:"enableDynamicSeedList,omitempty"`

	// Disables repairs of any tables that use either the TimeWindowCompactionStrategy or DateTieredCompactionStrategy.
	// This automatic blacklisting is not stored in schedules or repairs. It is applied when repairs are triggered and
	// visible in the UI for running repairs. Not storing which tables are TWCS/DTCS ensures changes to a table’s
	// compaction strategy are honored on every new repair.
	//
	// Note: It is recommended to enable this option as repairing these tables, when they contain TTL’d data, causes
	// overlaps between partitions across the configured time windows the sstables reside in. This leads to an
	// increased disk usage as the older sstables are unable to be expired despite only containing TTL’s data.
	// Repairing DTCS tables has additional issues and is generally not recommended.
	//
	// Defaults to false
	BlacklistTwcsTables bool `json:"blacklistTwcsTables,omitempty" yaml:"blacklistTwcsTables"`

	// Controls the timeout for establishing JMX connections. The value should be low enough to avoid stalling simple
	// operations in multi region clusters, but high enough to allow connections under normal conditions.
	//
	// Defaults to 20
	JmxConnectionTimeoutInSeconds *int32 `json:"jmxConnectionTimeoutInSeconds,omitempty" yaml:"jmxConnectionTimeoutInSeconds,omitempty"`

	// Defines the default amount of repair segments to create for newly registered Cassandra repair runs, for each
	// node in the cluster. When running a repair run by Reaper, each segment is repaired separately by the Reaper
	// process, until all the segments in a token ring are repaired. The count might be slightly off the defined value,
	// as clusters residing in multiple data centers require additional small token ranges in addition to the expected.
	// This value can be overwritten when executing a repair run via Reaper.
	//
	// Defaults to 16
	SegmentCountPerNode *int32 `json:"segmentCountPerNode,omitempty" yaml:"segmentCountPerNode,omitempty"`
}

// Specifies the replication strategy for a keyspace
type ReplicationConfig struct {
	// Specifies the replication_factor when SimpleStrategy is used
	SimpleStrategy *int32 `json:"simpleStrategy,omitempty"`

	// Specifies the replication_factor when NetworkTopologyStrategy is used. The mapping is data center name to RF.
	NetworkTopologyStrategy *map[string]int32 `json:"networkTopologyStrategy,omitempty"`
}

type AuthProvider struct {
	Type string `json:"type,omitempty" yaml:"type,omitempty"`

	Username string `json:"username,omitempty" yaml:"username,omitempty"`

	Password string `json:"password,omitempty" yaml:"password,omitempty"`
}

type CassandraBackend struct {
	ClusterName string `json:"clusterName" yaml:"clusterName"`

	// The headless service that provides endpoints for the Cassandra pods
	ContactPoints []string `json:"contactPoints" yaml:"contactPoints"`

	// Defaults to reaper
	Keyspace string `json:"keyspace,omitempty" yaml:"keyspace,omitempty"`

	Replication ReplicationConfig `json:"replication" yaml:"-"`

	AuthProvider AuthProvider `json:"authProvider,omitempty" yaml:"authProvider,omitempty"`
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

type DeploymentConfiguration struct {
	//DeploymentStrategy appsv1.DeploymentStrategy `json:"strategy,omitempty"`

	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

type CassandraCluster struct {
	Name    string `json:"name"`
	Service CassandraService `json:"service"`
}

type CassandraService struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// ReaperSpec defines the desired state of Reaper
type ReaperSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	Image string `json:"image,omitempty"`

	DeploymentConfiguration DeploymentConfiguration `json:"deploymentConfiguration,omitempty"`

	ServerConfig ServerConfig `json:"serverConfig,omitempty" yaml:"serverConfig,omitempty"`

	Clusters []CassandraCluster `json:"clusters,omitempty"`
}

type ReaperConditionType string

const (
	// With a Cassandra Reaper currently does all schema initialization except for creating the keyspace. Ideally
	// all schema initialized will be moved out of Reaper to avoid race conditions that can cause schema disagreement.
	SchemaInitialized ReaperConditionType = "SchemaInitialized"

	ConfigurationUpdated ReaperConditionType = "ConfigurationUpdated"
)

type ReaperCondition struct {
	// Type of reaper condition
	Type ReaperConditionType `json:"type"`

	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`

	// Last time the condition transit from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// (brief) reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`

	// Human readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

// ReaperStatus defines the observed state of Reaper
type ReaperStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// The latest available observations of an object's current state.
	Conditions []ReaperCondition `json:"conditions,omitempty"`

	// Total number of non-terminated pods targeted by this deployment (their labels match the selector).
	Replicas int32 `json:"replicas,omitempty"`

	// Total number of non-terminated pods targeted by this deployment that have the desired template spec.
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// Total number of ready pods targeted by this deployment.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Total number of available pods (ready for at least minReadySeconds) targeted by this deployment.
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// A hash of the latest known Reaper configuration. If the hash of Reaper.Spec.ServerConfig differs from
	// this value, it will trigger an update of Reaper's configuration file followed by a restart of Reaper
	// itself.
	Configuration string `json:"configuration,omitempty"`

	Clusters []CassandraCluster `json:"clusters,omitempty"`
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
