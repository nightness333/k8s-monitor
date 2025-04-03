package parser

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nightness333/k8s-monitor/pkg/types"
)

func ParseCSV(filePath string) ([]types.PodMetric, error) {
	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var metrics []types.PodMetric
	for i, record := range records {
		if i == 0 {
			continue
		}

		timestamp, _ := time.Parse(time.RFC3339, record[0])
		cpu, _ := strconv.ParseInt(strings.TrimSuffix(record[3], "m"), 10, 64)
		mem, _ := strconv.ParseInt(strings.TrimSuffix(record[4], "Mi"), 10, 64)

		metrics = append(metrics, types.PodMetric{
			Timestamp: timestamp,
			Namespace: record[1],
			Pod:       record[2],
			CPU:       cpu,
			Memory:    mem,
			Status:    record[5],
		})
	}

	return metrics, nil
}
