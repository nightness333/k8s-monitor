package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/nightness333/k8s-monitor/pkg/parser"
	"github.com/nightness333/k8s-monitor/pkg/types"
	"github.com/spf13/cobra"
)

const (
	hoursInMonth = 720
	cpuDivisor   = 1000.0
	memDivisor   = 1024.0
)

var (
	costCmd = &cobra.Command{
		Use:   "cost",
		Short: "Расчёт стоимости ресурсов кластера",
		Long: `Анализирует стоимость потребления CPU и памяти:
	- Общая стоимость по кластеру
	- Стоимость по неймспейсам
	- ТОП-5 самых дорогих подов`,
		Run: runCostCommand,
	}

	defaultCPUPrice = 0.02
	defaultMemPrice = 0.01
)

type PodCost struct {
	Name      string
	Namespace string
	CPUCost   float64
	MemCost   float64
	Lines     int64
	TotalCost float64
}

func init() {
	rootCmd.AddCommand(costCmd)
	costCmd.Flags().StringP("file", "f", "/data/output.csv", "Файл с метриками (CSV)")
	costCmd.Flags().Float64("cpu-price", defaultCPUPrice, "Цена за 1 CPU-core/час ($)")
	costCmd.Flags().Float64("mem-price", defaultMemPrice, "Цена за 1 GiB памяти/час ($)")
}

func runCostCommand(cmd *cobra.Command, args []string) {
	filePath, _ := cmd.Flags().GetString("file")
	cpuPrice, _ := cmd.Flags().GetFloat64("cpu-price")
	memPrice, _ := cmd.Flags().GetFloat64("mem-price")

	metrics, err := parser.ParseCSV(filePath)
	if err != nil {
		fmt.Printf("Ошибка чтения метрик: %v\n", err)
		os.Exit(1)
	}

	calculateAndPrintCosts(metrics, cpuPrice, memPrice)
}

func calculateAndPrintCosts(metrics []types.PodMetric, cpuPrice, memPrice float64) {
	podCosts := calculatePodCosts(metrics, cpuPrice, memPrice)
	nsCosts := calculateNamespaceCosts(metrics, cpuPrice, memPrice)

	totalCost := calculateTotalCost(podCosts)

	printTotalCost(totalCost)
	printNamespaceCosts(nsCosts)
	printTopPods(podCosts)
}

func calculatePodCosts(metrics []types.PodMetric, cpuPrice, memPrice float64) map[string]*PodCost {
	podCosts := make(map[string]*PodCost)

	for _, m := range metrics {
		key := m.Namespace + "/" + m.Pod
		if _, exists := podCosts[key]; !exists {
			podCosts[key] = &PodCost{
				Name:      m.Pod,
				Namespace: m.Namespace,
			}
		}

		podCosts[key].CPUCost += float64(m.CPU)
		podCosts[key].MemCost += float64(m.Memory)
		podCosts[key].Lines++
	}

	for _, cost := range podCosts {
		cost.CPUCost = (cost.CPUCost / float64(cost.Lines) / cpuDivisor) * cpuPrice
		cost.MemCost = (cost.MemCost / float64(cost.Lines) / memDivisor) * memPrice
		cost.TotalCost = cost.CPUCost + cost.MemCost
	}

	return podCosts
}

func calculateNamespaceCosts(metrics []types.PodMetric, cpuPrice, memPrice float64) map[string]*PodCost {
	nsCosts := make(map[string]*PodCost)

	for _, m := range metrics {
		if _, exists := nsCosts[m.Namespace]; !exists {
			nsCosts[m.Namespace] = &PodCost{
				Namespace: m.Namespace,
			}
		}

		nsCosts[m.Namespace].CPUCost += float64(m.CPU)
		nsCosts[m.Namespace].MemCost += float64(m.Memory)
		nsCosts[m.Namespace].Lines++
	}

	for _, cost := range nsCosts {
		cost.CPUCost = (cost.CPUCost / float64(cost.Lines) / cpuDivisor) * cpuPrice
		cost.MemCost = (cost.MemCost / float64(cost.Lines) / memDivisor) * memPrice
		cost.TotalCost = cost.CPUCost + cost.MemCost
	}

	return nsCosts
}

func calculateTotalCost(podCosts map[string]*PodCost) float64 {
	total := 0.0
	for _, cost := range podCosts {
		total += cost.TotalCost
	}
	return total
}

func printTotalCost(totalCost float64) {
	fmt.Printf("\n=== ОБЩАЯ СТОИМОСТЬ РЕСУРСОВ ===\n")
	fmt.Printf("За месяц: $%.2f\n\n", totalCost*hoursInMonth)
}

func printNamespaceCosts(nsCosts map[string]*PodCost) {
	fmt.Println("=== ПО НЕЙМСПЕЙСАМ ===")
	for ns, cost := range nsCosts {
		fmt.Printf("%-20s: $%.2f (CPU: $%.2f, Memory: $%.2f)\n",
			ns, cost.TotalCost*hoursInMonth, cost.CPUCost*hoursInMonth, cost.MemCost*hoursInMonth)
	}
}

func printTopPods(podCosts map[string]*PodCost) {
	sortedPods := make([]*PodCost, 0, len(podCosts))
	for _, cost := range podCosts {
		sortedPods = append(sortedPods, cost)
	}

	sort.Slice(sortedPods, func(i, j int) bool {
		return sortedPods[i].TotalCost > sortedPods[j].TotalCost
	})

	fmt.Println("\n=== ТОП-5 САМЫХ ДОРОГИХ ПОДОВ ===")
	for i := 0; i < len(sortedPods) && i < 5; i++ {
		p := sortedPods[i]

		fmt.Printf("%d. %-40s: $%.2f (CPU: $%.2f, Memory: $%.2f)\n",
			i+1, p.Namespace+"/"+p.Name, p.TotalCost*hoursInMonth, p.CPUCost*hoursInMonth, p.MemCost*hoursInMonth)
	}
}
