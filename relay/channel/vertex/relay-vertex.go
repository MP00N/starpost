package vertex

import "github.com/QuantumNous/new-api/common"

func GetModelRegion(other string, localModelName string) string {
	// if other is json string
	if common.IsJsonObject(other) {
		m, err := common.StrToMap(other)
		if err != nil {
			return other // return original if parsing fails
		}
		// Use checked assertions: a non-string region value in the channel
		// configuration must not panic the request handler.
		if region, ok := m[localModelName].(string); ok {
			return region
		}
		if region, ok := m["default"].(string); ok {
			return region
		}
		return "global"
	}
	return other
}
