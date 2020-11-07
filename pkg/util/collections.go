package util

func MergeMap(destination map[string]string, sources ...map[string]string) map[string]string {
	for _, source := range sources {
		for k, v := range source {
			destination[k] = v
		}
	}

	return destination
}
