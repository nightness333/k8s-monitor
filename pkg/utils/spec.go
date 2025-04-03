package utils

import (
	"context"

	"github.com/nightness333/k8s-monitor/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetPodLimits(clientset *kubernetes.Clientset, namespace, podName string) (*types.PodConfiguration, error) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	limits := &types.PodConfiguration{}
	for _, container := range pod.Spec.Containers {
		if container.Resources.Limits != nil {
			limits.CPU += container.Resources.Limits.Cpu().MilliValue()
			limits.Memory += container.Resources.Limits.Memory().Value() / (1024 * 1024)
		}
	}
	return limits, nil
}

func GetPodRequests(clientset *kubernetes.Clientset, namespace, podName string) (*types.PodConfiguration, error) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	limits := &types.PodConfiguration{}
	for _, container := range pod.Spec.Containers {
		if container.Resources.Limits != nil {
			limits.CPU += container.Resources.Requests.Cpu().MilliValue()
			limits.Memory += container.Resources.Requests.Memory().Value() / (1024 * 1024)
		}
	}
	return limits, nil
}
