package config

import (
	"errors"
	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
)

type ValidationError error

var (
	ClusterNameRequired   ValidationError = errors.New("CassandraBackend.ClusterName is required")
	ContactPointsRequired ValidationError = errors.New("CassandraBackend.ContactPoints is required")
)

type Validator interface {
	Validate(cfg v1alpha1.ServerConfig) error

	SetDefaults(cfg *v1alpha1.ServerConfig) bool
}

type validator struct{}

func NewValidator() Validator {
	return &validator{}
}

func (v *validator) Validate(cfg v1alpha1.ServerConfig) error {
	if cfg.StorageType == "" || cfg.StorageType == v1alpha1.Memory {
		return nil
	}

	if cfg.StorageType == v1alpha1.Cassandra {
		if cfg.CassandraBackend == nil || cfg.CassandraBackend.ClusterName == "" {
			return ClusterNameRequired
		}

		if len(cfg.CassandraBackend.ContactPoints) == 0 {
			return ContactPointsRequired
		}
	}

	return nil
}

func (v *validator) SetDefaults(cfg *v1alpha1.ServerConfig) bool {
	updated := false

	if cfg.HangingRepairTimeoutMins == nil {
		cfg.HangingRepairTimeoutMins = int32Ptr(v1alpha1.DefaultHangingRepairTimeoutMins)
		updated = true
	}

	if cfg.RepairIntensity == "" {
		cfg.RepairIntensity = v1alpha1.DefaultRepairIntensity
		updated = true
	}

	if cfg.RepairParallelism == "" {
		cfg.RepairParallelism = v1alpha1.DefaultRepairParallelism
		updated = true
	}

	if cfg.RepairRunThreadCount == nil {
		cfg.RepairRunThreadCount = int32Ptr(v1alpha1.DefaultRepairRunThreadCount)
		updated = true
	}

	if cfg.ScheduleDaysBetween == nil {
		cfg.ScheduleDaysBetween = int32Ptr(v1alpha1.DefaultScheduleDaysBetween)
		updated = true
	}

	if cfg.StorageType == "" {
		cfg.StorageType = v1alpha1.DefaultStorageType
		updated = true
	}

	if cfg.EnableCrossOrigin == nil {
		cfg.EnableCrossOrigin = boolPtr(v1alpha1.DefaultEnableCrossOrigin)
		updated = true
	}

	if cfg.EnableDynamicSeedList == nil {
		cfg.EnableDynamicSeedList = boolPtr(v1alpha1.DefaultEnableDynamicSeedList)
		updated = true
	}

	if cfg.JmxConnectionTimeoutInSeconds == nil {
		cfg.JmxConnectionTimeoutInSeconds = int32Ptr(v1alpha1.DefaultJmxConnectionTimeoutInSeconds)
		updated = true
	}

	if cfg.SegmentCountPerNode == nil {
		cfg.SegmentCountPerNode = int32Ptr(v1alpha1.DefaultSegmentCountPerNode)
		updated = true
	}

	if cfg.StorageType == v1alpha1.Cassandra {
		cassandra := cfg.CassandraBackend
		if cassandra.Keyspace == "" {
			cassandra.Keyspace = v1alpha1.DefaultKeyspace
			updated = true
		}

		if cassandra.AuthProvider == (v1alpha1.AuthProvider{}) {
			cassandra.AuthProvider = v1alpha1.AuthProvider{
				Type: "plainText",
				Username: "cassandra",
				Password: "cassandra",
			}
			updated = true
		}

		if cassandra.Replication == (v1alpha1.ReplicationConfig{}) {
			cassandra.Replication = v1alpha1.ReplicationConfig{
				SimpleStrategy: int32Ptr(1),
			}
			updated = true
		}
	}

	return updated
}

func int32Ptr(n int32) *int32 {
	return &n
}

func boolPtr(b bool) *bool {
	return &b
}