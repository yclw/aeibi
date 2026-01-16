package util

// BoolToInt64 converts a boolean to 1 or 0.
func BoolToInt64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
