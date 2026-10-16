package storage

func init() {
	errorCodes["local_budget"] = true
	diagnosticMessages["rate_limit/local_budget"] = "Local aggregate request budget deferred collection; no provider request made for this page"
	errorCodes["aggregate_request"] = true
	diagnosticMessages["configuration/aggregate_request"] = "Requested aggregate plant or calendar day does not match the persisted job"
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
