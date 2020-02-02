package backend

import (
	"testing"
	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
)

func TestValidate(t *testing.T) {
	tests := []struct{
		name         string
		serverConfig v1alpha1.ServerConfig
		expected     error
	}{
		{
			name: "NoBackend",
			serverConfig: v1alpha1.ServerConfig{},
			expected: nil,
		},
		{
			name: "MemoryBackend",
			serverConfig: v1alpha1.ServerConfig{
				StorageType: v1alpha1.Memory,
			},
			expected: nil,
		},
		{
			name: "CassandraStorageTypeAndCassandraBackendUndefined",
			serverConfig: v1alpha1.ServerConfig{
				StorageType: v1alpha1.Cassandra,
			},
			expected:ClusterNameRequired,
		},
		{
			name: "CassandraBackendNoClusterName",
			serverConfig: v1alpha1.ServerConfig{
				StorageType: v1alpha1.Cassandra,
				CassandraBackend: &v1alpha1.CassandraBackend{
					ContactPoints: []string{"localhost"},
				},
			},
			expected: ClusterNameRequired,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Validate(tt.serverConfig); got != tt.expected {
				t.Errorf("expected (%s), got (%s)", tt.expected, got)
			}
		})
	}
}
