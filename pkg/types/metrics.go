package types

import "time"

type PodMetric struct {
	Timestamp time.Time
	Namespace string
	Pod       string
	CPU       int64
	Memory    int64
	Status    string
}

type PodConfiguration struct {
	CPU    int64
	Memory int64
}

type PodStats struct {
	CPU    []int64
	Memory []int64
	Status string
}
