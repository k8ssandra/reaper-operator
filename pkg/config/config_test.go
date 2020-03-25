package config

import (
	"testing"
	"github.com/thelastpickle/reaper-operator/pkg/apis/reaper/v1alpha1"
)

func TestValidate(t *testing.T) {
	validator := NewValidator()
	tests := []struct{
		name         string
		reaper       *v1alpha1.Reaper
		expected     error
	}{
		{
			name: "NoBackend",
			reaper: &v1alpha1.Reaper{},
			expected: nil,
		},
		{
			name: "MemoryBackend",
			reaper: &v1alpha1.Reaper{
				Spec: v1alpha1.ReaperSpec{
					ServerConfig: v1alpha1.ServerConfig{
						StorageType: v1alpha1.Memory,
					},
				},
			},
			expected: nil,
		},
		{
			name: "CassandraStorageTypeAndCassandraBackendUndefined",
			reaper: &v1alpha1.Reaper{
				Spec: v1alpha1.ReaperSpec{
					ServerConfig: v1alpha1.ServerConfig{
						StorageType: v1alpha1.Cassandra,
					},
				},
			},
			expected:ClusterNameRequired,
		},
		{
			name: "CassandraBackendNoClusterName",
			reaper: &v1alpha1.Reaper{
				Spec: v1alpha1.ReaperSpec{
					ServerConfig: v1alpha1.ServerConfig{
						StorageType: v1alpha1.Cassandra,
						CassandraBackend: &v1alpha1.CassandraBackend{
							ContactPoints: []string{"localhost"},
						},
					},
				},
			},
			expected: ClusterNameRequired,
		},
		{
			name: "CassandraBackendNoContactPoints",
			reaper: &v1alpha1.Reaper{
				Spec: v1alpha1.ReaperSpec{
					ServerConfig: v1alpha1.ServerConfig{
						StorageType: v1alpha1.Cassandra,
						CassandraBackend: &v1alpha1.CassandraBackend{
							ClusterName: "test",
						},
					},
				},
			},
			expected: ContactPointsRequired,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validator.Validate(tt.reaper); got != tt.expected {
				t.Errorf("expected (%s), got (%s)", tt.expected, got)
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	validator := NewValidator()
	reaper := &v1alpha1.Reaper{}

	if updated := validator.SetDefaults(reaper); !updated {
		t.Errorf("Expected ServerConfig to get updated")
	}

	cfg := reaper.Spec.ServerConfig

	if cfg.HangingRepairTimeoutMins == nil {
		t.Errorf("HangingRepairTimeoutMins is nil. Expected (%d)", v1alpha1.DefaultHangingRepairTimeoutMins)
	} else if *cfg.HangingRepairTimeoutMins != v1alpha1.DefaultHangingRepairTimeoutMins {
		t.Errorf("HangingRepairTimeoutMins (%d) is not the expected value (%d)", *cfg.HangingRepairTimeoutMins, v1alpha1.DefaultHangingRepairTimeoutMins)
	}

	if cfg.RepairIntensity != v1alpha1.DefaultRepairIntensity {
		t.Errorf("RepairIntensity (%s) is not the expected value (%s)", cfg.RepairIntensity, v1alpha1.DefaultRepairIntensity)
	}

	if cfg.RepairParallelism != v1alpha1.DefaultRepairParallelism {
		t.Errorf("RepairParallelism (%s) is not the expected value (%s)", cfg.RepairParallelism, v1alpha1.DefaultRepairParallelism)
	}

	if cfg.RepairRunThreadCount == nil {
		t.Errorf("RepairRunThreadCount is nil. Expected(%d)", v1alpha1.DefaultRepairRunThreadCount)
	} else if *cfg.RepairRunThreadCount != v1alpha1.DefaultRepairRunThreadCount {
		t.Errorf("RepairRunThreadCount (%d) is not the expected value (%d)", *cfg.RepairRunThreadCount, v1alpha1.DefaultRepairRunThreadCount)
	}

	if cfg.ScheduleDaysBetween == nil {
		t.Errorf("ScheduleDaysBetween is nil. Expected (%d)", v1alpha1.DefaultScheduleDaysBetween)
	} else if *cfg.ScheduleDaysBetween != v1alpha1.DefaultScheduleDaysBetween {
		t.Errorf("ScheduleDaysBetween (%d) is not the expected value (%d)", *cfg.ScheduleDaysBetween, v1alpha1.DefaultScheduleDaysBetween)
	}

	if cfg.StorageType != v1alpha1.DefaultStorageType {
		t.Errorf("StorageType (%s) is not the expected value (%s)", cfg.StorageType, v1alpha1.DefaultStorageType)
	}

	if cfg.EnableCrossOrigin == nil {
		t.Errorf("EnableCrossOrigin is nil. Expected (%t)", v1alpha1.DefaultEnableCrossOrigin)
	} else if *cfg.EnableCrossOrigin != v1alpha1.DefaultEnableCrossOrigin {
		t.Errorf("EnableCrossOrigin (%t) is not the expected value (%t)", *cfg.EnableCrossOrigin, v1alpha1.DefaultEnableCrossOrigin)
	}

	if cfg.EnableDynamicSeedList == nil {
		t.Errorf("EnableDynamicSeedList is nil. Expected (%t)", v1alpha1.DefaultEnableDynamicSeedList)
	} else if *cfg.EnableDynamicSeedList != v1alpha1.DefaultEnableDynamicSeedList {
		t.Errorf("EnableDynamicSeedList (%t) is not the expected value (%t)", *cfg.EnableDynamicSeedList, v1alpha1.DefaultEnableDynamicSeedList)
	}

	if cfg.JmxConnectionTimeoutInSeconds == nil {
		t.Errorf("JmxConnectionTimeoutInSeconds is nil. Expected (%d)", v1alpha1.DefaultJmxConnectionTimeoutInSeconds)
	} else if *cfg.JmxConnectionTimeoutInSeconds != v1alpha1.DefaultJmxConnectionTimeoutInSeconds {
		t.Errorf("JmxConnectionTimeoutInSeconds (%d) is not the expected value (%d)", *cfg.JmxConnectionTimeoutInSeconds, v1alpha1.DefaultJmxConnectionTimeoutInSeconds)
	}

	if cfg.SegmentCountPerNode == nil {
		t.Errorf("SegmentCountPerNode is nil. Expected (%d)", v1alpha1.DefaultSegmentCountPerNode)
	} else if *cfg.SegmentCountPerNode != v1alpha1.DefaultSegmentCountPerNode {
		t.Errorf("SegmentCountPerNode (%d) is not the expected value (%d)", *cfg.SegmentCountPerNode, v1alpha1.DefaultSegmentCountPerNode)
	}
}

func TestSetDefaultsWithCassandraBackend(t *testing.T) {
	validator := NewValidator()
	reaper := &v1alpha1.Reaper{
		Spec: v1alpha1.ReaperSpec{
			ServerConfig: v1alpha1.ServerConfig{
				StorageType: v1alpha1.Cassandra,
				CassandraBackend: &v1alpha1.CassandraBackend{
					ClusterName: "test",
				},
			},
		},
	}

	if updated := validator.SetDefaults(reaper); !updated {
		t.Errorf("Expected ServerConfig to get updated")
	}

	cfg := reaper.Spec.ServerConfig

	if (*cfg.CassandraBackend).Keyspace != v1alpha1.DefaultKeyspace {
		t.Errorf("Keyspace (%s) is not the expectedAuthProvider value (%s)", (*cfg.CassandraBackend).Keyspace, v1alpha1.DefaultKeyspace)
	}

	expectedAuthProvider := v1alpha1.AuthProvider{
		Type: "plainText",
		Username: "cassandra",
		Password: "cassandra",
	}
	if (*cfg.CassandraBackend).AuthProvider != expectedAuthProvider {
		t.Errorf("AuthProvider (%+v) is not the expectedAuthProvider value (%+v)", (*cfg.CassandraBackend).AuthProvider, expectedAuthProvider)
	}

	if *cfg.CassandraBackend.Replication.SimpleStrategy != 1 {
		t.Errorf("SimpleStrategy (%+v) is not the expected value (%+v)", *cfg.CassandraBackend.Replication.SimpleStrategy, 1)
	}

	if cfg.CassandraBackend.Replication.NetworkTopologyStrategy != nil {
		t.Errorf("NetworkTopologyStrategy (%+v) should be nil", *cfg.CassandraBackend.Replication.NetworkTopologyStrategy)
	}
}
