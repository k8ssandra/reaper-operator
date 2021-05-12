package config

import (
	"fmt"
	"strconv"
	"strings"

	api "github.com/k8ssandra/reaper-operator/api/v1alpha1"
)

func ReplicationToString(r api.ReplicationConfig) string {
	if r.SimpleStrategy != nil {
		replicationFactor := strconv.FormatInt(int64(*r.SimpleStrategy), 10)
		return fmt.Sprintf(`{'class': 'SimpleStrategy', 'replication_factor': %s}`, replicationFactor)
	} else {
		var sb strings.Builder
		dcs := make([]string, 0)
		for k, v := range *r.NetworkTopologyStrategy {
			sb.WriteString("'")
			sb.WriteString(k)
			sb.WriteString("': ")
			sb.WriteString(strconv.FormatInt(int64(v), 10))
			dcs = append(dcs, sb.String())
			sb.Reset()
		}
		return fmt.Sprintf("{'class': 'NetworkTopologyStrategy', %s}", strings.Join(dcs, ", "))
	}
}

func ReplicationToConfig(dcName string, r api.ReplicationConfig) []map[string]string {
	replicationConfig := make([]map[string]string, 1)
	if r.SimpleStrategy != nil {
		replicationFactor := strconv.FormatInt(int64(*r.SimpleStrategy), 10)
		replicationConfig = append(replicationConfig, map[string]string{
			"dc_name":            dcName,
			"replication_factor": replicationFactor,
		})
	} else {
		for k, v := range *r.NetworkTopologyStrategy {
			replicationFactor := strconv.FormatInt(int64(v), 10)
			replicationConfig = append(replicationConfig, map[string]string{
				"dc_name":            k,
				"replication_factor": replicationFactor,
			})
		}
	}
	return replicationConfig
}
