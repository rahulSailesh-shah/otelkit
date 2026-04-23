package config

func Merge(base, overlay map[string]any) map[string]any {
	if base == nil {
		base = make(map[string]any)
	}

	for k, v := range overlay {
		if v == nil {
			delete(base, k)
			continue
		}
		if overlayMap, ok := v.(map[string]any); ok {
			if baseMap, ok := base[k].(map[string]any); ok {
				base[k] = Merge(baseMap, overlayMap)
				continue
			}
		}
		base[k] = v
	}
	return base
}
