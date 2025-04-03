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

var optimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Анализирует метрики и предлагает оптимизацию ресурсов",
	Run: func(cmd *cobra.Command, args []string) {
		filePath, _ := cmd.Flags().GetString("file")
		margin, _ := cmd.Flags().GetInt("margin")

		metrics, err := parser.ParseCSV(filePath)
		if err != nil {
			fmt.Printf("Ошибка чтения метрик: %v\n", err)
			os.Exit(1)
		}

		optimizeClusterResources(metrics, margin)
	},
}

func init() {
	rootCmd.AddCommand(optimizeCmd)
	optimizeCmd.Flags().StringP("file", "f", "/data/output.csv", "Файл с метриками (CSV)")
	optimizeCmd.Flags().IntP("margin", "m", 20, "Запас прочности (%)")
}

func optimizeClusterResources(metrics []types.PodMetric, margin int) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			fmt.Printf("ошибка подключения к Kubernetes: %v\n", err)

			return
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("ошибка создания клиента Kubernetes: %v\n", err)

		return
	}

	fmt.Print("=== ОПТИМИЗАЦИЯ РЕСУРСОВ ===\n\n")

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

	for key, p := range podStats {
		cpuAvg := utils.Avg(p.CPU)
		cpuMax := utils.Max(p.CPU)

		memAvg := utils.Avg(p.Memory)
		memMax := utils.Max(p.Memory)

		ns, name := utils.SplitPodKey(key)

		limits, err := utils.GetPodLimits(clientset, ns, name)
		if err != nil {
			fmt.Printf("Ошибка получения конфигурации для %-20s: %v\n", key, err)

			continue
		}

		requests, err := utils.GetPodRequests(clientset, ns, name)
		if err != nil {
			fmt.Printf("Ошибка получения конфигурации для %-20s: %v\n", key, err)

			continue
		}

		fmt.Printf("[Под %-20s]:\n", key)

		fmt.Println("• Текущие значения:")
		fmt.Printf("  Средние:      CPU=%4dm, Mem=%4dMi\n", cpuAvg, memAvg)
		fmt.Printf("  Максимальные: CPU=%4dm, Mem=%4dMi\n", cpuMax, memMax)

		if limits != nil && requests != nil {
			fmt.Printf("  CPU: requests=%4dm, limit=%4dm\n", requests.CPU, limits.CPU)
			fmt.Printf("  Память: requests=%4dMi, limit=%4dMi\n\n", requests.Memory, limits.Memory)
		}

		cpuReqRec := int(float64(cpuAvg)*(1.0+float64(margin)/100.0)) + 1
		cpuLimRec := int(float64(cpuMax)*(1.0+float64(margin)/100.0)) + 1
		memReqRec := int(float64(memAvg)*(1.0+float64(margin)/100.0)) + 1
		memLimRec := int(float64(memMax)*(1.0+float64(margin)/100.0)) + 1

		fmt.Println("• Рекомендации:")
		fmt.Printf(
			"  CPU: requests=%4dm, limit=%4dm\n",
			cpuReqRec,
			cpuLimRec,
		)
		fmt.Printf(
			"  Память: requests=%4dMi, limit=%4dMi\n\n",
			memReqRec,
			memLimRec,
		)
	}
}
