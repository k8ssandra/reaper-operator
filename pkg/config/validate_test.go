package config

import (
	"testing"

	api "github.com/thelastpickle/reaper-operator/api/v1alpha1"
)

func TestValidate(t *testing.T) {
	validator := NewValidator()
	tests := []struct {
		name     string
		reaper   *api.Reaper
		expected error
	}{
		{
			name:     "NoBackend",
			reaper:   &api.Reaper{},
			expected: nil,
		},
		{
			name: "MemoryBackend",
			reaper: &api.Reaper{
				Spec: api.ReaperSpec{
					ServerConfig: api.ServerConfig{
						StorageType: api.StorageTypeMemory,
					},
				},
			},
			expected: nil,
		},
		{
			name: "CassandraStorageTypeAndCassandraBackendUndefined",
			reaper: &api.Reaper{
				Spec: api.ReaperSpec{
					ServerConfig: api.ServerConfig{
						StorageType: api.StorageTypeCassandra,
					},
				},
			},
			expected: ClusterNameRequired,
		},
		{
			name: "CassandraBackendNoClusterName",
			reaper: &api.Reaper{
				Spec: api.ReaperSpec{
					ServerConfig: api.ServerConfig{
						StorageType: api.StorageTypeCassandra,
						CassandraBackend: &api.CassandraBackend{
							CassandraService: "localhost",
						},
					},
				},
			},
			expected: ClusterNameRequired,
		},
		{
			name: "CassandraBackendNoContactPoints",
			reaper: &api.Reaper{
				Spec: api.ReaperSpec{
					ServerConfig: api.ServerConfig{
						StorageType: api.StorageTypeCassandra,
						CassandraBackend: &api.CassandraBackend{
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
	reaper := &api.Reaper{}

	if updated := validator.SetDefaults(reaper); !updated {
		t.Errorf("Expected ServerConfig to get updated")
	}

	cfg := reaper.Spec.ServerConfig

	if cfg.StorageType != api.DefaultStorageType {
		t.Errorf("StorageType (%s) is not the expected value (%s)", cfg.StorageType, api.DefaultStorageType)
	}
}

func TestSetDefaultsWithCassandraBackend(t *testing.T) {
	validator := NewValidator()
	reaper := &api.Reaper{
		Spec: api.ReaperSpec{
			ServerConfig: api.ServerConfig{
				StorageType: api.StorageTypeCassandra,
				CassandraBackend: &api.CassandraBackend{
					ClusterName: "test",
				},
			},
		},
	}

	if updated := validator.SetDefaults(reaper); !updated {
		t.Errorf("Expected ServerConfig to get updated")
	}

	cfg := reaper.Spec.ServerConfig

	if (*cfg.CassandraBackend).Keyspace != api.DefaultKeyspace {
		t.Errorf("Keyspace (%s) is not the expectedAuthProvider value (%s)", (*cfg.CassandraBackend).Keyspace, api.DefaultKeyspace)
	}

	expectedAuthProvider := api.AuthProvider{
		Type:     "plainText",
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
