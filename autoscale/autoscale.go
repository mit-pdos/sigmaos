package autoscale

import (
	"fmt"
	"math"
	"sync"
	"time"

	db "sigmaos/debug"
)

type Metric interface {
	GetValue() float64
}

type AddReplicasFn func(n int) error
type RemoveReplicasFn func(n int) error

type Autoscaler struct {
	mu                 sync.Mutex
	currentReplicas    int
	desiredReplicas    int
	maxReplicas        int
	currentMetricValue float64
	desiredMetricValue float64
	tolerance          float64
	freq               time.Duration
	m                  Metric
	addReplicas        AddReplicasFn
	removeReplicas     RemoveReplicasFn
	done               bool
}

func NewAutoscaler(initialReplicas int, maxReplicas int, desiredMetricValue float64, freq time.Duration, tolerance float64, m Metric, addReplicas AddReplicasFn, removeReplicas RemoveReplicasFn) *Autoscaler {
	return &Autoscaler{
		currentReplicas:    initialReplicas,
		desiredReplicas:    initialReplicas,
		maxReplicas:        maxReplicas,
		desiredMetricValue: desiredMetricValue,
		freq:               freq,
		tolerance:          tolerance,
		m:                  m,
		addReplicas:        addReplicas,
		removeReplicas:     removeReplicas,
	}
}

func (a *Autoscaler) String() string {
	return fmt.Sprintf("&{ currentReplicas:%v desiredReplicas:%v maxReplicas:%v currentMetricValue:%v desiredMetricValue:%v tolerance:%v freq:%v m:%v }", a.currentReplicas, a.desiredReplicas, a.maxReplicas, a.currentMetricValue, a.desiredMetricValue, a.tolerance, a.freq, a.m)
}

func (a *Autoscaler) autoscalingRound() {
	a.mu.Lock()
	defer a.mu.Unlock()

	db.DPrintf(db.AUTOSCALER, "autoscalingRound")
	// Get current metric value
	a.currentMetricValue = a.m.GetValue()
	db.DPrintf(db.AUTOSCALER, "currentMetricValue:%v desiredMetricValue:%v", a.currentMetricValue, a.desiredMetricValue)
	// From: https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/
	//
	// Kubernetes HPA algorithm: desiredReplicas = ceil(currentReplicas * (currentMetricValue / desiredMetricValue))
	if a.desiredMetricValue > 0 {
		ratio := a.currentMetricValue / a.desiredMetricValue
		db.DPrintf(db.AUTOSCALER, "ratio:%v", ratio)
		// Check if ratio is within tolerance
		if math.Abs(ratio-1.0) <= a.tolerance {
			db.DPrintf(db.AUTOSCALER, "ratio %v within tolerance %v, not scaling", ratio, a.tolerance)
			return
		}
		a.desiredReplicas = int(math.Ceil(float64(a.currentReplicas) * ratio))
		// Ensure at least 1 replica
		if a.desiredReplicas < 1 {
			a.desiredReplicas = 1
		}
		// Enforce max replicas limit (if set)
		if a.maxReplicas > 0 && a.desiredReplicas > a.maxReplicas {
			a.desiredReplicas = a.maxReplicas
		}
		// Scale up or down
		delta := a.desiredReplicas - a.currentReplicas
		db.DPrintf(db.AUTOSCALER, "delta: %v", delta)
		if delta > 0 {
			// Scale up
			if err := a.addReplicas(delta); err != nil {
				db.DPrintf(db.AUTOSCALER_ERR, "Err addReplicas: %v", err)
				db.DPrintf(db.ERROR, "Err addReplicas: %v", err)
			} else {
				a.currentReplicas = a.desiredReplicas
				db.DPrintf(db.AUTOSCALER, "added %v replicas currentReplicas:%v", delta, a.currentReplicas)
			}
		} else if delta < 0 {
			// Scale down
			if err := a.removeReplicas(-1 * delta); err != nil {
				db.DPrintf(db.AUTOSCALER_ERR, "Err removeReplicas: %v", err)
				db.DPrintf(db.ERROR, "Err removeReplicas: %v", err)
			} else {
				a.currentReplicas = a.desiredReplicas
				db.DPrintf(db.AUTOSCALER, "removed %v replicas currentReplicas:%v", delta, a.currentReplicas)
			}
		}
	}
}

func (a *Autoscaler) isDone() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.done
}

func (a *Autoscaler) runAutoscaler() {
	for !a.isDone() {
		time.Sleep(a.freq)
		a.autoscalingRound()
	}
}

func (a *Autoscaler) Run() {
	go a.runAutoscaler()
}

func (a *Autoscaler) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.done = true
}
