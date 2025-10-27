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
	svc                string
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

func NewAutoscaler(svc string, initialReplicas int, maxReplicas int, desiredMetricValue float64, freq time.Duration, tolerance float64, m Metric, addReplicas AddReplicasFn, removeReplicas RemoveReplicasFn) *Autoscaler {
	return &Autoscaler{
		svc:                svc,
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

	db.DPrintf(db.AUTOSCALER, "[%v] autoscalingRound currentReplicas:%v", a.svc, a.currentReplicas)
	// Get current metric value
	a.currentMetricValue = a.m.GetValue()
	db.DPrintf(db.AUTOSCALER, "[%v] currentMetricValue:%v desiredMetricValue:%v", a.svc, a.currentMetricValue, a.desiredMetricValue)
	// From: https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/
	//
	// Kubernetes HPA algorithm: desiredReplicas = ceil(currentReplicas * (currentMetricValue / desiredMetricValue))
	if a.desiredMetricValue > 0 {
		ratio := a.currentMetricValue / a.desiredMetricValue
		db.DPrintf(db.AUTOSCALER, "[%v] ratio:%v", a.svc, ratio)
		// Check if ratio is within tolerance
		if math.Abs(ratio-1.0) <= a.tolerance {
			db.DPrintf(db.AUTOSCALER, "[%v] ratio %v within tolerance %v, not scaling", a.svc, ratio, a.tolerance)
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
			db.DPrintf(db.AUTOSCALER, "[%v] desiredReplicas %v > maxReplicas %v, capping scaling", a.svc, a.desiredReplicas, a.maxReplicas)
		}
		// Scale up or down
		delta := a.desiredReplicas - a.currentReplicas
		db.DPrintf(db.AUTOSCALER, "[%v] delta: %v", a.svc, delta)
		if delta > 0 {
			// Scale up
			if err := a.addReplicas(delta); err != nil {
				db.DPrintf(db.AUTOSCALER_ERR, "[%v] Err addReplicas: %v", a.svc, err)
				db.DPrintf(db.ERROR, "[%v] Err addReplicas: %v", a.svc, err)
			} else {
				a.currentReplicas = a.desiredReplicas
				db.DPrintf(db.AUTOSCALER, "[%v] added %v replicas currentReplicas:%v", a.svc, delta, a.currentReplicas)
			}
		} else if delta < 0 {
			// Scale down
			if err := a.removeReplicas(-1 * delta); err != nil {
				db.DPrintf(db.AUTOSCALER_ERR, "[%v] Err removeReplicas: %v", a.svc, err)
				db.DPrintf(db.ERROR, "[%v] Err removeReplicas: %v", a.svc, err)
			} else {
				a.currentReplicas = a.desiredReplicas
				db.DPrintf(db.AUTOSCALER, "[%v] removed %v replicas currentReplicas:%v", a.svc, delta, a.currentReplicas)
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
