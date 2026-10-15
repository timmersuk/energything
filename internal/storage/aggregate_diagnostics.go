package storage

func init() {
	for code, message := range map[string]string{
		"aggregate_unit":     "Aggregate response resolution does not match requested day",
		"aggregate_date":     "Aggregate period is missing or outside the requested provider date",
		"aggregate_decimal":  "Aggregate energy is absent, null, a placeholder or not a nonnegative decimal string",
		"aggregate_conflict": "Conflicting generation values for the same plant calendar period",
	} {
		errorCodes[code] = true
		diagnosticMessages["validation/"+code] = message
	}
}
