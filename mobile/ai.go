package mobile

import "github.com/AMINOPY/pg-core/internal/ai"

// AnalyzeNetworkJSON returns an explainable analysis of sanitized network measurements.
// The host should collect carrier/SIM metadata only with user consent and must never
// include subscriber identifiers, phone numbers, credentials, or precise location.
func AnalyzeNetworkJSON(reportJSON string) (string, error) {
	return ai.AnalyzeJSON(reportJSON)
}
