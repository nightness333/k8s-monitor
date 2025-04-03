package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/nightness333/k8s-monitor/pkg/parser"
	"github.com/nightness333/k8s-monitor/pkg/types"
	"github.com/spf13/cobra"
)

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Расчёт стоимости ресурсов кластера",
	Long: `Анализирует стоимость потребления CPU и памяти:
		- Общая стоимость по кластеру
		- Стоимость по неймспейсам
		- ТОП-5 самых дорогих подов`,
	Run: func(cmd *cobra.Command, args []string) {
		filePath, _ := cmd.Flags().GetString("file")
		cpuPrice, _ := cmd.Flags().GetFloat64("cpu-price")
		memPrice, _ := cmd.Flags().GetFloat64("mem-price")

		metrics, err := parser.ParseCSV(filePath)
		if err != nil {
			fmt.Printf("Ошибка чтения метрик: %v\n", err)
			os.Exit(1)
		}

		calculateAndPrintCosts(metrics, cpuPrice, memPrice)
	},
}

func init() {
	rootCmd.AddCommand(costCmd)
	costCmd.Flags().StringP("file", "f", "data.csv", "Файл с метриками (CSV)")
	costCmd.Flags().Float64("cpu-price", 0.02, "Цена за 1 CPU-core/час ($)")
	costCmd.Flags().Float64("mem-price", 0.01, "Цена за 1 GiB памяти/час ($)")
}

type PodCost struct {
	Name      string
	Namespace string
	CPUCost   float64
	MemCost   float64
	Lines     int64
	TotalCost float64
}

func calculateAndPrintCosts(metrics []types.PodMetric, cpuPrice, memPrice float64) {
	podCosts := make(map[string]*PodCost)
	nsCosts := make(map[string]*PodCost)

	for _, m := range metrics {
		key := m.Namespace + "/" + m.Pod
		if _, exists := podCosts[key]; !exists {
			podCosts[key] = &PodCost{
				Name:      m.Pod,
				Namespace: m.Namespace,
				Lines:     0,
			}
		}

		podCosts[key].CPUCost += float64(m.CPU)
		podCosts[key].MemCost += float64(m.Memory)
		podCosts[key].Lines += 1

		if _, exists := nsCosts[m.Namespace]; !exists {
			nsCosts[m.Namespace] = &PodCost{
				Namespace: m.Namespace,
				Lines:     0,
			}
		}

		nsCosts[m.Namespace].CPUCost += float64(m.CPU)
		nsCosts[m.Namespace].MemCost += float64(m.Memory)
		nsCosts[m.Namespace].Lines += 1
	}

	totalCost := 0.0

	for _, m := range podCosts {
		m.CPUCost = (m.CPUCost / float64(m.Lines) / 1000.0) * cpuPrice
		m.MemCost = (m.MemCost / float64(m.Lines) / 1024.0) * memPrice
		m.TotalCost = m.CPUCost + m.MemCost

		totalCost += m.TotalCost
	}

	for _, m := range nsCosts {
		m.CPUCost = (m.CPUCost / float64(m.Lines) / 1000.0) * cpuPrice
		m.MemCost = (m.MemCost / float64(m.Lines) / 1024.0) * memPrice
		m.TotalCost = m.CPUCost + m.MemCost
	}

	sortedPods := make([]*PodCost, 0, len(podCosts))
	for _, cost := range podCosts {
		sortedPods = append(sortedPods, cost)
	}
	sort.Slice(sortedPods, func(i, j int) bool {
		return sortedPods[i].TotalCost > sortedPods[j].TotalCost
	})

	fmt.Printf("\n=== ОБЩАЯ СТОИМОСТЬ РЕСУРСОВ ===\n")
	fmt.Printf("За месяц: $%.2f\n\n", totalCost*float64(720))

	fmt.Println("=== ПО НЕЙМСПЕЙСАМ ===")
	for ns, cost := range nsCosts {
		fmt.Printf("%-20s: $%.2f (CPU: $%.2f, Memory: $%.2f)\n",
			ns, cost.TotalCost*float64(720), cost.CPUCost*float64(720), cost.MemCost*float64(720))
	}

	fmt.Println("\n=== ТОП-5 САМЫХ ДОРОГИХ ПОДОВ ===")
	for i := 0; i < len(sortedPods) && i < 5; i++ {
		p := sortedPods[i]
		fmt.Printf("%d. %-40s: $%.2f (CPU: $%.2f, Memory: $%.2f)\n",
			i+1, p.Namespace+"/"+p.Name, p.TotalCost*float64(720), p.CPUCost*float64(720), p.MemCost*float64(720))
	}
}
