package observability

import (
	"sort"
	"strings"
)

func metricKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}

	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString(name)
	builder.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(key)
		builder.WriteString(`="`)
		builder.WriteString(escapeLabelValue(labels[key]))
		builder.WriteByte('"')
	}
	builder.WriteByte('}')

	return builder.String()
}

func escapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
