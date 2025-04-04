package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nightness333/k8s-monitor/pkg/parser"
	"github.com/nightness333/k8s-monitor/pkg/types"
	"github.com/nightness333/k8s-monitor/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultMargin = 20
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Анализирует метрики и предлагает оптимизацию ресурсов",
	Run:   runOptimizeCommand,
}

func init() {
	rootCmd.AddCommand(optimizeCmd)
	optimizeCmd.Flags().StringP("file", "f", "/data/output.csv", "Файл с метриками (CSV)")
	optimizeCmd.Flags().IntP("margin", "m", defaultMargin, "Запас прочности (%)")
}

func runOptimizeCommand(cmd *cobra.Command, args []string) {
	filePath, _ := cmd.Flags().GetString("file")
	margin, _ := cmd.Flags().GetInt64("margin")

	metrics, err := parser.ParseCSV(filePath)
	if err != nil {
		fmt.Printf("Ошибка чтения метрик: %v\n", err)
		os.Exit(1)
	}

	optimizeClusterResources(metrics, margin)
}

func optimizeClusterResources(metrics []types.PodMetric, margin int64) {
	clientset, err := createKubernetesClient()
	if err != nil {
		fmt.Printf("Ошибка подключения к Kubernetes: %v\n", err)
		return
	}

	fmt.Print("=== ОПТИМИЗАЦИЯ РЕСУРСОВ ===\n\n")

	podStats := aggregatePodMetrics(metrics)

	for key, stats := range podStats {
		ns, name := utils.SplitPodKey(key)
		printPodOptimization(clientset, ns, name, stats, margin)
	}
}

func createKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(config)
}

func aggregatePodMetrics(metrics []types.PodMetric) map[string]*types.PodStats {
	podStats := make(map[string]*types.PodStats)

	for _, m := range metrics {
		key := m.Namespace + "/" + m.Pod

		if _, exists := podStats[key]; !exists {
			podStats[key] = &types.PodStats{
				Status: "OK",
			}
		}

		podStats[key].CPU = append(podStats[key].CPU, m.CPU)
		podStats[key].Memory = append(podStats[key].Memory, m.Memory)
	}

	return podStats
}

func printPodOptimization(clientset *kubernetes.Clientset, ns, name string, stats *types.PodStats, margin int64) {
	key := ns + "/" + name
	cpuAvg := utils.Avg(stats.CPU)
	cpuMax := utils.Max(stats.CPU)
	memAvg := utils.Avg(stats.Memory)
	memMax := utils.Max(stats.Memory)

	limits, err := utils.GetPodLimits(clientset, ns, name)
	if err != nil {
		fmt.Printf("Ошибка получения конфигурации для %-20s: %v\n", key, err)
		return
	}

	requests, err := utils.GetPodRequests(clientset, ns, name)
	if err != nil {
		fmt.Printf("Ошибка получения конфигурации для %-20s: %v\n", key, err)
		return
	}

	fmt.Printf("[Под %-20s]:\n", key)
	printCurrentMetrics(cpuAvg, cpuMax, memAvg, memMax, limits, requests)
	printRecommendations(cpuAvg, cpuMax, memAvg, memMax, margin)
}

func printCurrentMetrics(cpuAvg, cpuMax, memAvg, memMax int64, limits, requests *types.PodConfiguration) {
	fmt.Println("• Текущие значения:")
	fmt.Printf("  Средние:      CPU=%4dm, Mem=%4dMi\n", cpuAvg, memAvg)
	fmt.Printf("  Максимальные: CPU=%4dm, Mem=%4dMi\n", cpuMax, memMax)

	if limits != nil && requests != nil {
		fmt.Printf("  CPU: requests=%4dm, limit=%4dm\n", requests.CPU, limits.CPU)
		fmt.Printf("  Память: requests=%4dMi, limit=%4dMi\n\n", requests.Memory, limits.Memory)
	}
}

func printRecommendations(cpuAvg, cpuMax, memAvg, memMax int64, margin int64) {
	cpuReqRec := calculateWithMargin(cpuAvg, margin)
	cpuLimRec := calculateWithMargin(cpuMax, margin)
	memReqRec := calculateWithMargin(memAvg, margin)
	memLimRec := calculateWithMargin(memMax, margin)

	fmt.Println("• Рекомендации:")
	fmt.Printf("  CPU: requests=%4dm, limit=%4dm\n", cpuReqRec, cpuLimRec)
	fmt.Printf("  Память: requests=%4dMi, limit=%4dMi\n\n", memReqRec, memLimRec)
}

func calculateWithMargin(value int64, margin int64) int64 {
	return int64(float64(value)*(1.0+float64(margin)/100.0)) + 1
}
