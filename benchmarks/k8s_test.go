package benchmarks_test

import (
	"os/exec"
	db "sigmaos/debug"
	"strconv"
	"strings"
)

func k8sTop() string {
	b, err := exec.Command("kubectl", "top", "pods").Output()
	if err != nil {
		db.DFatalf("Error exec CPU util: %v", err)
	}
	return string(b)
}

// K8s MR pods are named worker-* or coordinator-*
func isMRPod(podName string) bool {
	return strings.Contains(podName, "worker") || strings.Contains(podName, "coordinator")
}

func parseK8sUtil(utilStr, app string) float64 {
	util := float64(0.0)
	entries := strings.Split(utilStr, "\n")
	// Skip the title line
	for _, entry := range entries[1:] {
		words := strings.Split(entry, " ")
		podName := words[0]
		switch app {
		case "mr":
			// Skip non-MR pods
			if !isMRPod(podName) {
				continue
			}
		case "hotel":
			// Skip MR pods
			if isMRPod(podName) {
				continue
			}
		default:
			db.DFatalf("unknown k8s app to parse: %v", app)
		}
		db.DPrintf(db.ALWAYS, "Monitoring app %v, include pod %v", app, podName)
		// Iterate backwards to find CPU util information.
		for i := len(words) - 1; i >= 0; i-- {
			word := words[i]
			// If this is the CPU util information.
			if word[len(word)-1] == 'm' {
				mcpu, err := strconv.Atoi(word[:len(word)-1])
				if err != nil {
					db.DFatalf("Can't parse mili-CPU: %v", err)
				}
				util += float64(mcpu) / 1000.0
				break
			}
		}
	}
	return util
}
