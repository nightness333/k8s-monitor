package cmd

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/spf13/cobra"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Собирает информацию о подах в кластере Kubernetes с фильтрацией",
	Run: func(cmd *cobra.Command, args []string) {
		interval, _ := cmd.Flags().GetInt("interval")
		output, _ := cmd.Flags().GetString("output")
		namespaces, _ := cmd.Flags().GetStringSlice("namespaces")
		labelSelector, _ := cmd.Flags().GetStringToString("labels")

		fmt.Printf("Запуск мониторинга (интервал: %d сек, файл: %s)...\n", interval, output)
		fmt.Printf("Фильтры: namespaces=%v, labels=%v\n", namespaces, labelSelector)
		startMonitoring(interval, output, namespaces, labelSelector)
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)

	monitorCmd.Flags().IntP("interval", "i", 10, "Интервал сбора данных в секундах")
	monitorCmd.Flags().StringP("output", "o", "/data/output.csv", "Файл для сохранения данных")
	monitorCmd.Flags().StringSliceP("namespaces", "n", []string{}, "Фильтр по namespace (через запятую)")
	monitorCmd.Flags().StringToStringP("labels", "l", map[string]string{}, "Фильтр по labels (key=value)")
}

func startMonitoring(interval int, output string, namespaces []string, labelSelector map[string]string) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			fmt.Printf("Ошибка подключения к Kubernetes: %v\n", err)
			return
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Ошибка создания клиента Kubernetes: %v\n", err)
		return
	}

	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		fmt.Printf("Ошибка создания клиента метрик: %v\n", err)
		return
	}

	if err := checkMetricsServerAvailable(metricsClient); err != nil {
		fmt.Printf("Metrics Server недоступен: %v\n", err)
		return
	}

	file, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Ошибка открытия файла: %v\n", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if stat, _ := file.Stat(); stat.Size() == 0 {
		writer.Write([]string{"Timestamp", "Namespace", "Pod", "CPU", "Memory", "Status"})
	}

	for {
		var pods *corev1.PodList
		var totalPods, successPods, errorPods int

		listOptions := metav1.ListOptions{
			LabelSelector: labels.Set(labelSelector).String(),
		}

		if len(namespaces) == 0 {
			pods, err = clientset.CoreV1().Pods("").List(context.TODO(), listOptions)
			if err != nil {
				fmt.Printf("Ошибка получения подов: %v\n", err)
				continue
			}
		} else {
			pods = &corev1.PodList{}
			for _, ns := range namespaces {
				nsPods, err := clientset.CoreV1().Pods(ns).List(context.TODO(), listOptions)
				if err != nil {
					fmt.Printf("Ошибка получения подов в ns %s: %v\n", ns, err)
					continue
				}
				pods.Items = append(pods.Items, nsPods.Items...)
			}
		}

		totalPods = len(pods.Items)

		for _, pod := range pods.Items {
			record := []string{
				time.Now().Format(time.RFC3339),
				pod.Namespace,
				pod.Name,
				"N/A", // CPU
				"N/A", // Memory
				"OK",  // Status
			}

			if pod.Status.Phase == corev1.PodRunning {
				cpu, mem, err := getPodMetricsWithRetry(metricsClient, pod.Namespace, pod.Name)
				if err != nil {
					record[5] = fmt.Sprintf("ERROR: %v", err)
					errorPods++
					fmt.Printf("Ошибка для пода %s/%s: %v\n", pod.Namespace, pod.Name, err)
				} else {
					record[3] = cpu
					record[4] = mem
					successPods++
					fmt.Printf("Под %s/%s: CPU=%s, Memory=%s\n", pod.Namespace, pod.Name, cpu, mem)
				}
			} else {
				record[5] = fmt.Sprintf("SKIP: status=%s", pod.Status.Phase)
			}

			writer.Write(record)
		}

		writer.Flush()
		if err := writer.Error(); err != nil {
			fmt.Printf("Ошибка записи в CSV: %v\n", err)
		}

		fmt.Printf("[Итог] Обработано: %d, Успешно: %d, Ошибки: %d\n\n",
			totalPods, successPods, errorPods)

		time.Sleep(time.Duration(interval) * time.Second)
	}
}

func checkMetricsServerAvailable(metricsClient *metrics.Clientset) error {
	_, err := metricsClient.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("не удалось получить метрики: %v", err)
	}
	return nil
}

func getPodMetricsWithRetry(metricsClient *metrics.Clientset, namespace, name string) (string, string, error) {
	var lastErr error
	for i := 0; i < 2; i++ {
		cpu, mem, err := getPodMetrics(metricsClient, namespace, name)
		if err == nil {
			return cpu, mem, nil
		}
		lastErr = err
		time.Sleep(1 * time.Second)
	}
	return "", "", lastErr
}

func getPodMetrics(metricsClient *metrics.Clientset, namespace, name string) (string, string, error) {
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	var totalCPU, totalMem int64
	for _, container := range podMetrics.Containers {
		totalCPU += container.Usage.Cpu().MilliValue()
		totalMem += container.Usage.Memory().Value()
	}

	return fmt.Sprintf("%dm", totalCPU), fmt.Sprintf("%dMi", totalMem/1024/1024), nil
}
