package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nightness333/k8s-monitor/pkg/parser"
	"github.com/nightness333/k8s-monitor/pkg/types"
	"github.com/nightness333/k8s-monitor/pkg/utils"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/spf13/cobra"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Анализ утилизации ресурсов кластера",
	Long: `Генерирует отчет с ключевыми метриками:
		- Общая статистика по CPU/памяти
		- ТОП-5 подов по потреблению
		- Анализ по неймспейсам
		- Выявление аномалий`,
	Run: func(cmd *cobra.Command, args []string) {
		file, _ := cmd.Flags().GetString("file")
		last, _ := cmd.Flags().GetString("last")

		if err := analyzeClusterResources(file, last); err != nil {
			fmt.Printf("Ошибка: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.Flags().StringP("file", "f", "data.csv", "Файл с метриками")
	reportCmd.Flags().StringP("last", "l", "24h", "Анализировать данные за период (1h, 24h, 7d)")
}

func analyzeClusterResources(filePath, timeRange string) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return fmt.Errorf("ошибка подключения к Kubernetes: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("ошибка создания клиента Kubernetes: %v", err)
	}

	metrics, err := parser.ParseCSV(filePath)
	if err != nil {
		return err
	}

	duration, err := time.ParseDuration(timeRange)
	if err != nil {
		return fmt.Errorf("неверный формат периода: %v", err)
	}
	timeThreshold := time.Now().Add(-duration)

	metricsMap := make(map[string]*types.PodStats)
	for _, m := range metrics {
		if m.Timestamp.Before(timeThreshold) {
			continue
		}

		key := m.Namespace + "/" + m.Pod
		if _, exists := metricsMap[key]; !exists {
			metricsMap[key] = &types.PodStats{
				Status: m.Status,
			}
		}
		metricsMap[key].CPU = append(metricsMap[key].CPU, m.CPU)
		metricsMap[key].Memory = append(metricsMap[key].Memory, m.Memory)
	}

	printSummary(metricsMap)
	printNamespaceStats(metricsMap)
	printTopConsumers(metricsMap, clientset)
	printAnomalies(metricsMap)

	return nil
}

func printSummary(data map[string]*types.PodStats) {
	fmt.Println("\n=== ОБЩАЯ СТАТИСТИКА ===")
	fmt.Printf("Анализируется %d подов\n", len(data))

	var totalCPU, totalMem int64
	for _, m := range data {
		totalCPU += utils.Avg(m.CPU)
		totalMem += utils.Avg(m.Memory)
	}

	fmt.Printf("Среднее по кластеру:\nCPU: %dm | Память: %dMi\n",
		totalCPU/int64(len(data)),
		totalMem/int64(len(data)))
}

func printNamespaceStats(data map[string]*types.PodStats) {
	nsStats := make(map[string]*struct{ cpu, mem, count int64 })

	for key, m := range data {
		ns := strings.Split(key, "/")[0]
		if _, ok := nsStats[ns]; !ok {
			nsStats[ns] = &struct{ cpu, mem, count int64 }{}
		}
		nsStats[ns].cpu += utils.Avg(m.CPU)
		nsStats[ns].mem += utils.Avg(m.Memory)
		nsStats[ns].count++
	}

	fmt.Println("\n=== ПО НЕЙМСПЕЙСАМ ===")
	for ns, stats := range nsStats {
		fmt.Printf("%-15s: %3d подов | CPU: %4dm | Память: %4dMi\n",
			ns, stats.count, stats.cpu/stats.count, stats.mem/stats.count)
	}
}

func printTopConsumers(data map[string]*types.PodStats, clientset *kubernetes.Clientset) {
	type rankedPod struct {
		Name   string
		CPU    int64
		Memory int64
	}

	pods := make([]rankedPod, 0, len(data))
	for key, m := range data {
		pods = append(pods, rankedPod{
			Name:   key,
			CPU:    utils.Max(m.CPU),
			Memory: utils.Max(m.Memory),
		})
	}

	sort.Slice(pods, func(i, j int) bool { return pods[i].CPU > pods[j].CPU })
	fmt.Println("\n=== ТОП-5 ПО CPU ===")
	for i := 0; i < len(pods) && i < 5; i++ {
		ns, name := utils.SplitPodKey(pods[i].Name)
		limits, _ := utils.GetPodLimits(clientset, ns, name)

		fmt.Printf("%d. %-40s: %4dm", i+1, pods[i].Name, pods[i].CPU)
		if limits != nil && limits.CPU > 0 {
			utilization := 100 * pods[i].CPU / limits.CPU
			fmt.Printf(" (Лимит: %dm, Использование: %d%%)", limits.CPU, utilization)
		}
		fmt.Println()
	}

	sort.Slice(pods, func(i, j int) bool { return pods[i].Memory > pods[j].Memory })
	fmt.Println("\n=== ТОП-5 ПО ПАМЯТИ ===")
	for i := 0; i < len(pods) && i < 5; i++ {
		ns, name := utils.SplitPodKey(pods[i].Name)
		limits, _ := utils.GetPodLimits(clientset, ns, name)

		fmt.Printf("%d. %-40s: %4dMi", i+1, pods[i].Name, pods[i].Memory)
		if limits != nil && limits.Memory > 0 {
			utilization := 100 * pods[i].Memory / limits.Memory
			fmt.Printf(" (Лимит: %dMi, Использование: %d%%)", limits.Memory, utilization)
		}
		fmt.Println()
	}
}

func printAnomalies(data map[string]*types.PodStats) {
	fmt.Println("\n=== АНОМАЛИИ ===")
	found := false

	for key, m := range data {
		if len(m.CPU) < 10 {
			continue
		}

		avgCPU := utils.Avg(m.CPU)
		maxCPU := utils.Max(m.CPU)
		cpuSpike := float64(maxCPU)/float64(avgCPU) > 3 && maxCPU > 500

		avgMem := utils.Avg(m.Memory)
		maxMem := utils.Max(m.Memory)
		memSpike := float64(maxMem)/float64(avgMem) > 3 && maxMem > 1024

		if cpuSpike || memSpike {
			found = true
			fmt.Printf("Под %s:\n", key)
			if cpuSpike {
				fmt.Printf("  - CPU: скачок с %dm до %dm (x%.1f)\n",
					avgCPU, maxCPU, float64(maxCPU)/float64(avgCPU))
			}
			if memSpike {
				fmt.Printf("  - Память: скачок с %dMi до %dMi (x%.1f)\n",
					avgMem, maxMem, float64(maxMem)/float64(avgMem))
			}
		}
	}

	if !found {
		fmt.Println("Критических аномалий не обнаружено")
	}
}
