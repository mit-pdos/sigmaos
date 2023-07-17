package tsp

type Tour struct {
	c     *[]City
	order []int
	dist  float64
}

func InitTour(c *[]City, order []int) (Tour, error) {
	if len(*c) != len(order) {
		return Tour{}, mkErr("Invalid Tour: city length must match order length")
	}
	return Tour{c: c, order: order, dist: -1.0}, nil
}

func (t *Tour) CalcDist() {
	t.dist = 0
	for i := 0; i < len(t.order)-1; i++ {
		t.dist += Distance((*t.c)[t.order[i]], (*t.c)[t.order[i+1]])
	}
}
