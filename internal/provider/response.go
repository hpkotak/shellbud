package provider

import "encoding/json"

func isStructuredJSON(expectJSON bool, text string) bool {
	if !expectJSON {
		return false
	}

	var decoded any
	return json.Unmarshal([]byte(text), &decoded) == nil
}
