package remote

import (
	"fmt"

	db "sigmaos/debug"
)

func getK8sHotelFrontendAddr(bcfg *BenchConfig, lcfg *LocalFSConfig) (string, error) {
	args := []string{
		"--svc", "frontend",
	}
	ip, err := lcfg.RunScriptGetOutput("./get-k8s-svc-addr.sh", args...)
	if err != nil {
		return "", fmt.Errorf("Err GetK8sFrontendAddr: %v", err)
	}
	return ip + ":5000", nil
}

// TODO: do this properly
func startK8sHotelApp(bcfg *BenchConfig, lcfg *LocalFSConfig) error {
	return nil
	db.DFatalf("Unimplemented")
	// Always stop the k8s app first
	if err := stopK8sHotelApp(bcfg, lcfg); err != nil {
		return err
	}
	// First, start the cache deployment
	args1 := []string{
		"--path", "DeathStarBench/hotelReservation/kubernetes-cached",
		"--nrunning", "3",
	}
	if err := lcfg.RunScriptRedirectOutputStdout("./start-k8s-app.sh", args1...); err != nil {
		return fmt.Errorf("Err startK8sHotelApp 1: %v", err)
	}
	// Then, start the rest of the app
	args2 := []string{
		"--path", "DeathStarBench/hotelReservation/kubernetes-geo-noscale",
		"--nrunning", "4",
	}
	if err := lcfg.RunScriptRedirectOutputStdout("./start-k8s-app.sh", args2...); err != nil {
		return fmt.Errorf("Err startK8sHotelApp 2: %v", err)
	}
	// Then, start the rest of the app
	args3 := []string{
		"--path", "DeathStarBench/hotelReservation/kubernetes",
		"--nrunning", "19",
	}
	if err := lcfg.RunScriptRedirectOutputStdout("./start-k8s-app.sh", args3...); err != nil {
		return fmt.Errorf("Err startK8sHotelApp 2: %v", err)
	}
	return nil
}

// TODO: do this properly
func stopK8sHotelApp(bcfg *BenchConfig, lcfg *LocalFSConfig) error {
	return nil
	db.DFatalf("Unimplemented")
	args1 := []string{
		"--path", "DeathStarBench/hotelReservation/kubernetes",
	}
	if err := lcfg.RunScriptRedirectOutputStdout("./stop-k8s-app.sh", args1...); err != nil {
		return fmt.Errorf("Err stopK8sHotelApp 1: %v", err)
	}
	args2 := []string{
		"--path", "DeathStarBench/hotelReservation/kubernetes-geo-noscale",
	}
	if err := lcfg.RunScriptRedirectOutputStdout("./stop-k8s-app.sh", args2...); err != nil {
		return fmt.Errorf("Err stopK8sHotelApp 1: %v", err)
	}
	args3 := []string{
		"--path", "DeathStarBench/hotelReservation/kubernetes-cached",
	}
	if err := lcfg.RunScriptRedirectOutputStdout("./stop-k8s-app.sh", args3...); err != nil {
		return fmt.Errorf("Err stopK8sHotelApp 2: %v", err)
	}
	return nil
}

func startK8sSocialnetApp(bcfg *BenchConfig, lcfg *LocalFSConfig) error {
	db.DFatalf("Unimplemented")
	return nil
}

func stopK8sSocialnetApp(bcfg *BenchConfig, lcfg *LocalFSConfig) error {
	db.DFatalf("Unimplemented")
	return nil
}
