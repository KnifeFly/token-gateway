package repository

import ()

type rowScanner interface {
	Scan(dest ...any) error
}

func jsonOrDefault(data []byte, fallback string) []byte {
	if len(data) == 0 || string(data) == "null" {
		return []byte(fallback)
	}
	return data
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}
