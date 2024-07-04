package autoscaler

import (
	"math"
)

// Based on Kubernetes' Horizontal Pod Autoscaler: https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/
func k8sCalcDesiredNReplicas(currentReplicas int, currentMetricValue, desiredMetricValue, tolerance float64) int {
	ratio := currentMetricValue / desiredMetricValue
	// If ratio between current & desired metric values is within the tolerance,
	// desired number of replicas == current number of replicas
	if math.Abs(1.0-ratio) <= tolerance {
		return currentReplicas
	}
	desiredReplicas := math.Ceil(float64(currentReplicas) * ratio)
	return int(desiredReplicas)
}
