package tsp

import (
	"errors"
	"math"
	"math/rand"
	db "sigmaos/debug"
	"time"
)

func mkErr(msg string) error {
	return errors.New("TSP: " + msg + "\n")
}

type City struct {
	x int
	y int
}

func GenCity(maxX int, maxY int) City {
	c := City{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	c.x = r.Intn(maxX)
	c.y = r.Intn(maxY)
	return c
}

func (c *City) Set(x int, y int) error {
	if x < 0 {
		db.DPrintf(DEBUG_TSP, "City Set X Out of Range: %v", x)
		return mkErr("City Set X Out of Range")
	}
	if y < 0 {
		db.DPrintf(DEBUG_TSP, "City Set Y Out of Range: %v", x)
		return mkErr("City Set Y Out of Range")
	}
	c.x = x
	c.y = y
	return nil
}

func (c *City) Get() (int, int) {
	return c.x, c.y
}

func Distance(c1 City, c2 City) float64 {
	distX := math.Abs(float64(c1.x - c2.x))
	distY := math.Abs(float64(c1.y - c2.y))
	return math.Sqrt((distX * distX) + (distY * distY))
}
