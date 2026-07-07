package httpheader

import (
	"net/http"
	"net/textproto"
	"strings"
)

// AddVary adds Vary header values without duplicating existing entries.
func AddVary(h http.Header, values ...string) {
	existing := make(map[string]struct{})
	for _, raw := range h.Values("Vary") {
		for part := range strings.SplitSeq(raw, ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				continue
			}
			if value == "*" {
				return
			}
			existing[strings.ToLower(value)] = struct{}{}
		}
	}

	for _, raw := range values {
		value := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(raw))
		if value == "" {
			continue
		}
		if value == "*" {
			h.Set("Vary", "*")
			return
		}

		key := strings.ToLower(value)
		if _, ok := existing[key]; ok {
			continue
		}
		h.Add("Vary", value)
		existing[key] = struct{}{}
	}
}
