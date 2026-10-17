package storage

import "strings"

func init() {
	for key, message := range map[string]string{
		"pagination/page_budget":        "Pagination exceeded the bounded page budget",
		"pagination/unsafe_destination": "Pagination destination does not match the approved provider endpoint and requested window",
		"pagination/invalid_next":       "Provider next-page field is not a nonempty URL or null",
		"pagination/premature_end":      "Provider returned an empty page before terminating pagination",
		"protocol/missing_next":         "Provider page is missing the required next-page field",
		"validation/reading_structure":  "Provider reading rows do not have the required object structure",
	} {
		_, code, _ := strings.Cut(key, "/")
		errorCodes[code] = true
		diagnosticMessages[key] = message
	}
}
