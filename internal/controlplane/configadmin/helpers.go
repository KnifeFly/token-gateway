package configadmin

import (
	"encoding/json"
)

type rowScanner interface {
	Scan(dest ...any) error
}

func jsonOrDefault(data []byte, fallback string) []byte {
	if len(data) == 0 || string(data) == "null" {
		return []byte(fallback)
	}
	return data
}

func clone[T any](value T) *T {
	content, _ := json.Marshal(value)
	var out T
	_ = json.Unmarshal(content, &out)
	return &out
}
