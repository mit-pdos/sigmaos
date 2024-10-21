package benchmarks_test

import (
	"os/exec"
	db "sigmaos/debug"
	"strconv"
	"strings"

	sp "sigmaos/sigmap"
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

func parseK8sUtil(utilStr, app string, realm sp.Trealm) float64 {
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
		// If the benchmark specified a realm
		if realm.String() != "" && app == "mr" {
			// Skip workers not part of this realm.
			if !strings.Contains(podName, realm.String()) {
				continue
			}
		}
		// Iterate backwards to find CPU util information.
		for i := len(words) - 1; i >= 0; i-- {
			word := words[i]
			// If this is the CPU util information.
			if len(word) > 0 && word[len(word)-1] == 'm' {
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

func k8sJobHasCompleted(jobname string) bool {
	b, err := exec.Command("kubectl", "get", "job", jobname).Output()
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error exec get job: %v", err)
		return false
	}
	jstr := string(b)
	jline := strings.Split(jstr, "\n")[1]
	jstatestr := strings.Fields(jline)[1]
	jstate := strings.Split(jstatestr, "/")
	return jstate[0] == jstate[1]
}

func k8sScaleUpGeo() error {
	b, err := exec.Command("kubectl", "apply", "-Rf", "~/DeathStarBench/hotelReservation/kubernetes-geo-scale-large").Output()
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error exec manual scale-up: %v\n%v", err, string(b))
		return err
	}
	return nil
}
