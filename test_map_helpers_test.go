package ares

func makeSingleStringMap(key, value string) map[string]string {
	result := make(map[string]string, 1)
	result[key] = value
	return result
}
