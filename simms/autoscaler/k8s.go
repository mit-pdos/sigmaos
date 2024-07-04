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
func k8sCalcDesiredNReplicas(ctx *Ctx, currentReplicas int, currentMetricValue, desiredMetricValue, tolerance float64) int {
	ratio := currentMetricValue / desiredMetricValue
	// If ratio between current & desired metric values is within the tolerance,
	// desired number of replicas == current number of replicas
	if math.Abs(1.0-ratio) <= tolerance {
		db.DPrintf(db.SIM_AUTOSCALE, "%v NReplicas within tolerance range", ctx)
		return currentReplicas
	}
	desiredReplicas := math.Ceil(float64(currentReplicas) * ratio)
	return int(desiredReplicas)
}
