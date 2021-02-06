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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type StorageType string

const (
	DefaultReaperImage     = "thelastpickle/cassandra-reaper:2.1.3"
	DefaultImagePullPolicy = corev1.PullIfNotPresent

	StorageTypeMemory    = StorageType("memory")
	StorageTypeCassandra = StorageType("cassandra")

	DefaultKeyspace    = "reaper_db"
	DefaultStorageType = StorageTypeMemory
)

type ServerConfig struct {
	StorageType StorageType `json:"storageType,omitempty"`

	CassandraBackend *CassandraBackend `json:"cassandraBackend,omitempty" yaml:"cassandra,omitempty"`

	// Defines the username and password that Reaper will use to authenticate JMX connections to Cassandra
	// clusters. These credentials need to be stored on each Cassandra node.
	JmxUserSecretName string `json:"jmxUserSecretName,omitempty"`

	// If the autoscheduling should be enabled
	AutoScheduling *AutoScheduler `json:"autoScheduling,omitempty"`
}

// AutoScheduler includes options to configure the autoscheduling of repairs for new clusters
type AutoScheduler struct {
	Enabled bool `json:"enabled,omitempty"`
}

// Specifies the replication strategy for a keyspace
type ReplicationConfig struct {
	// Specifies the replication_factor when SimpleStrategy is used
	SimpleStrategy *int32 `json:"simpleStrategy,omitempty"`

	// Specifies the replication_factor when NetworkTopologyStrategy is used. The mapping is data center name to RF.
	NetworkTopologyStrategy *map[string]int32 `json:"networkTopologyStrategy,omitempty"`
}

type CassandraDatacenterRef struct {
	Name string `json:"name"`

	// If empty we could default the Reaper namespace.
	Namespace string `json:"namespace,omitempty"`
}

type CassandraBackend struct {
	CassandraDatacenter CassandraDatacenterRef `json:"cassandraDatacenter"`

	// Defaults to reaper
	Keyspace string `json:"keyspace,omitempty" yaml:"keyspace,omitempty"`

	Replication ReplicationConfig `json:"replication" yaml:"-"`

	CassandraUserSecretName string `json:"cassandraUserSecretName,omitempty" yaml:"cassandraUserSecretName,omitempty"`
}

// ReaperSpec defines the desired state of Reaper
type ReaperSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Image string `json:"image,omitempty"`

	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	ServerConfig ServerConfig `json:"serverConfig,omitempty" yaml:"serverConfig,omitempty"`
}

// ReaperStatus defines the observed state of Reaper
type ReaperStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Ready bool `json:"ready,omitempty"`

	Clusters []string `json:"clusters,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=reapers,scope=Namespaced

// Reaper is the Schema for the reapers API
type Reaper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReaperSpec   `json:"spec,omitempty"`
	Status ReaperStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReaperList contains a list of Reaper
type ReaperList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Reaper `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Reaper{}, &ReaperList{})
}
