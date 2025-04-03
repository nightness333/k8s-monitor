package utils

import "strings"

func Avg(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	var sum int64
	for _, v := range values {
		sum += v
	}
	return sum / int64(len(values))
}

func Max(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	var maxVal int64
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

func SplitPodKey(key string) (string, string) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
