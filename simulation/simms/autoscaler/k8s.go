package autoscaler

import (
	"math"

	db "sigmaos/debug"
)

// Based on Kubernetes' Horizontal Pod Autoscaler:
// https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/
//
// TODO: Implement K8s' more complex plan for handling missing metrics, and
// scale-up/scale-down dampening.
func k8sCalcDesiredNInstances(ctx *Ctx, currentInstances int, currentMetricValue, desiredMetricValue, tolerance float64, maxNReplicas int) int {
	ratio := currentMetricValue / desiredMetricValue
	// If ratio between current & desired metric values is within the tolerance,
	// desired number of instances == current number of instances
	if math.Abs(1.0-ratio) <= tolerance {
		db.DPrintf(db.SIM_AUTOSCALE, "%v NInstances within tolerance range", ctx)
		return currentInstances
	}
	desiredInstances := math.Ceil(float64(currentInstances) * ratio)
	return max(1, min(int(desiredInstances), maxNReplicas))
}
