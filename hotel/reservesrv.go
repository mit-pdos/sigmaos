package hotel

import (
	"fmt"
	"strconv"
	"time"

	"sigmaos/cacheclnt"
	"sigmaos/dbclnt"
	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/perf"
	"sigmaos/protdevsrv"
)

type ReserveRequest struct {
	CustomerName string
	HotelId      []string
	InDate       string
	OutDate      string
	Number       int
}

type ReserveResult struct {
	HotelIds []string
}

type Reservation struct {
	HotelID  string
	Customer string
	InDate   string
	OutDate  string
	Number   int
}

type Number struct {
	HotelId string
	Number  int
}

type Reserve struct {
	dbc    *dbclnt.DbClnt
	cachec *cacheclnt.CacheClnt
}

func (s *Reserve) initDb() error {
	q := fmt.Sprintf("truncate number;")
	err := s.dbc.Exec(q)
	if err != nil {
		return err
	}
	q = fmt.Sprintf("truncate reservation;")
	err = s.dbc.Exec(q)
	if err != nil {
		return err
	}

	q = fmt.Sprintf("INSERT INTO reservation (hotelid, customer, indate, outdate, number) VALUES ('%s', '%s', '%s', '%s', '%d');", "4", "Alice", "2015-04-09", "2015-04-10", 1)
	err = s.dbc.Exec(q)
	if err != nil {
		return err
	}

	for i := 1; i < 7; i++ {
		q = fmt.Sprintf("INSERT INTO number (hotelid, number) VALUES ('%v', '%v');",
			strconv.Itoa(i), 200)
		err = s.dbc.Exec(q)
		if err != nil {
			return err
		}
	}
	for i := 7; i <= NHOTEL; i++ {
		hotel_id := strconv.Itoa(i)
		room_num := 200
		if i%3 == 1 {
			room_num = 300
		} else if i%3 == 2 {
			room_num = 250
		}
		q = fmt.Sprintf("INSERT INTO number (hotelid, number) VALUES ('%v', '%v');",
			hotel_id, room_num)
		err = s.dbc.Exec(q)
		if err != nil {
			return err
		}
	}
	return nil
}

func RunReserveSrv(n string) error {
	r := &Reserve{}
	pds, err := protdevsrv.MakeProtDevSrv(np.HOTELRESERVE, r)
	if err != nil {
		return err
	}
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.FsLib(), np.DBD)
	if err != nil {
		return err
	}
	r.dbc = dbc
	cachec, err := cacheclnt.MkCacheClnt(pds.MemFs.FsLib(), NCACHE)
	if err != nil {
		return err
	}
	r.cachec = cachec
	err = r.initDb()
	if err != nil {
		return err
	}
	p := perf.MakePerf("HOTEL_RESERVE")
	defer p.Done()
	return pds.RunServer()
}

// checkAvailability checks if given information is available
func (s *Reserve) checkAvailability(hotelId string, req ReserveRequest) (bool, map[string]int, error) {
	inDate, _ := time.Parse(
		time.RFC3339,
		req.InDate+"T12:00:00+00:00")

	outDate, _ := time.Parse(
		time.RFC3339,
		req.OutDate+"T12:00:00+00:00")

	num_date := make(map[string]int)
	indate := inDate.String()[0:10]
	for inDate.Before(outDate) {
		// check reservations
		count := 0
		inDate = inDate.AddDate(0, 0, 1)
		outdate := inDate.String()[0:10]

		key := hotelId + "_" + indate + "_" + outdate

		var reserves []Reservation
		if err := s.cachec.Get(key, &count); err != nil {
			if err.Error() != cacheclnt.ErrMiss.Error() {
				return false, nil, err
			}
			db.DPrintf("HOTELRESERVE", "Check: cache miss res: key %v\n", key)
			q := fmt.Sprintf("SELECT * from reservation where hotelid='%s' AND indate='%s' AND outdate='%s';", hotelId, indate, outdate)
			err := s.dbc.Query(q, &reserves)
			if err != nil {
				return false, nil, err
			}
			for _, r := range reserves {
				count += r.Number
			}
			if err := s.cachec.Set(key, &count); err != nil {
				return false, nil, err
			}
		}

		num_date[key] = count + int(req.Number)

		// check capacity
		hotel_cap := 0
		key = hotelId + "_cap"
		if err := s.cachec.Get(key, &hotel_cap); err != nil {
			if err.Error() != cacheclnt.ErrMiss.Error() {
				return false, nil, err
			}
			db.DPrintf("HOTELRESERVE", "Check: cache miss id: key %v\n", key)
			var nums []Number
			q := fmt.Sprintf("SELECT * from number where hotelid='%s';", hotelId)
			err = s.dbc.Query(q, &nums)
			if err != nil {
				return false, nil, err
			}
			if len(nums) == 0 {
				return false, nil, fmt.Errorf("Unknown %v", hotelId)
			}
			hotel_cap = int(nums[0].Number)
			if err := s.cachec.Set(key, &hotel_cap); err != nil {
				return false, nil, err
			}
		}
		if count+int(req.Number) > hotel_cap {
			return false, nil, nil
		}
		indate = outdate
	}
	return true, num_date, nil
}

// MakeReservation makes a reservation based on given information
// XXX make check and reservation atomic
func (s *Reserve) MakeReservation(req ReserveRequest, res *ReserveResult) error {
	hotelId := req.HotelId[0]
	res.HotelIds = make([]string, 0)
	b, date_num, err := s.checkAvailability(hotelId, req)
	if err != nil {
		return err
	}
	if !b {
		return nil
	}

	// update reservation number
	db.DPrintf("HOTELRESERVE", "Update cache %v\n", date_num)
	for key, cnt := range date_num {
		if err := s.cachec.Set(key, &cnt); err != nil {
			return err
		}
	}

	inDate, _ := time.Parse(
		time.RFC3339,
		req.InDate+"T12:00:00+00:00")

	outDate, _ := time.Parse(
		time.RFC3339,
		req.OutDate+"T12:00:00+00:00")

	indate := inDate.String()[0:10]

	for inDate.Before(outDate) {
		inDate = inDate.AddDate(0, 0, 1)
		outdate := inDate.String()[0:10]

		q := fmt.Sprintf("INSERT INTO reservation (hotelid, customer, indate, outdate, number) VALUES ('%s', '%s', '%s', '%s', '%d');", hotelId, req.CustomerName, indate, outdate, req.Number)
		err := s.dbc.Exec(q)
		if err != nil {
			return fmt.Errorf("Insert failed %v", req)
		}
		indate = outdate
	}
	res.HotelIds = append(res.HotelIds, hotelId)
	return nil
}

func (s *Reserve) CheckAvailability(req ReserveRequest, res *ReserveResult) error {
	hotelids := make([]string, 0)
	for _, hotelId := range req.HotelId {
		b, _, err := s.checkAvailability(hotelId, req)
		if err != nil {
			return err
		}
		if b {
			hotelids = append(hotelids, hotelId)
		}
	}
	res.HotelIds = hotelids
	return nil
}
