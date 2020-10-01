package labels

const (
	ManagedByLabel      = "app.kubernetes.io/managed-by"
	ManagedByLabelValue = "reaper-operator"
	ReaperLabel         = "reaper.cassandra-reaper.io/reaper"
)

func SetOperatorLabels(m map[string]string) {
	m[ManagedByLabel] = ManagedByLabelValue
}
