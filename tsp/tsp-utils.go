package tsp

import "errors"

const DEBUG_TSP = "TSP"
const MAX_CITIES = 19
const INFINITY = int(^uint(0) >> 1)

func mkErr(msg string) error {
	return errors.New("TSP: " + msg + "\n")
}
