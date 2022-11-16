package hotel

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"sigmaos/dbclnt"
	np "sigmaos/ninep"
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

type reservation struct {
	HotelID      string
	CustomerName string
	InDate       string
	OutDate      string
	Number       int
}

type number struct {
	HotelId string
	Number  int
}

type Reserve struct {
	dbc *dbclnt.DbClnt
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

	for i := 0; i < 7; i++ {
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
	pds := protdevsrv.MakeProtDevSrv(np.HOTELRESERVE, r)
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.FsLib, np.DBD)
	if err != nil {
		return err
	}
	r.dbc = dbc
	err = r.initDb()
	if err != nil {
		return err
	}
	return pds.RunServer()
}

// MakeReservation makes a reservation based on given information
func (s *Reserve) MakeReservation(req ReserveRequest, res *ReserveResult) error {
	res.HotelIds = make([]string, 0)

	inDate, _ := time.Parse(
		time.RFC3339,
		req.InDate+"T12:00:00+00:00")

	outDate, _ := time.Parse(
		time.RFC3339,
		req.OutDate+"T12:00:00+00:00")
	hotelId := req.HotelId[0]

	indate := inDate.String()[0:10]

	for inDate.Before(outDate) {
		// check reservations
		count := 0
		inDate = inDate.AddDate(0, 0, 1)
		outdate := inDate.String()[0:10]

		// XXX add indate and outdate
		var reserves []reservation
		q := fmt.Sprintf("SELECT * from reservation where hotelid='%s';", hotelId)
		err := s.dbc.Query(q, &reserves)
		if err != nil {
			log.Printf("reserves err %v\n", err)
			return err
		}
		log.Printf("reserves %v\n", reserves)
		for _, r := range reserves {
			count += r.Number
		}

		// check capacity
		hotel_cap := 0
		var nums []number
		q = fmt.Sprintf("SELECT * from number where hotelid='%s';", hotelId)
		err = s.dbc.Query(q, &nums)
		if err != nil {
			return err
		}
		if len(nums) == 0 {
			return fmt.Errorf("Unknown %v\n", hotelId)
		}
		hotel_cap = int(nums[0].Number)
		if count+int(req.Number) > hotel_cap {
			return nil
		}
		indate = outdate
	}

	inDate, _ = time.Parse(
		time.RFC3339,
		req.InDate+"T12:00:00+00:00")

	indate = inDate.String()[0:10]

	for inDate.Before(outDate) {
		inDate = inDate.AddDate(0, 0, 1)
		outdate := inDate.String()[0:10]

		q := fmt.Sprintf("INSERT INTO reservation (hotelid, customer, indate, outdate, number) VALUES ('%v', '%v', '%v', '%v', '%v');", hotelId, req.CustomerName, indate, outdate, req.Number)
		err := s.dbc.Exec(q)
		if err != nil {
			return fmt.Errorf("Insert failed %v\n", req)
		}
		indate = outdate
	}

	res.HotelIds = append(res.HotelIds, hotelId)

	return nil
}

// CheckAvailability checks if given information is available
func (s *Reserve) CheckAvailability(req ReserveRequest, res *ReserveResult) error {
	res.HotelIds = make([]string, 0)

	for _, hotelId := range req.HotelId {
		inDate, _ := time.Parse(
			time.RFC3339,
			req.InDate+"T12:00:00+00:00")

		outDate, _ := time.Parse(
			time.RFC3339,
			req.OutDate+"T12:00:00+00:00")

		// indate := inDate.String()[0:10]

		for inDate.Before(outDate) {
			// check reservations
			count := 0
			inDate = inDate.AddDate(0, 0, 1)
			//outdate := inDate.String()[0:10]

			// XXX add indate and outdate; factor out query
			var reserves []reservation
			q := fmt.Sprintf("SELECT * from reservation where hotelid='%s';", hotelId)
			err := s.dbc.Query(q, &reserves)
			if err != nil {
				log.Printf("check reserves err %v\n", err)
				return err
			}
			log.Printf("reserves %v\n", reserves)

			for _, r := range reserves {
				count += r.Number
			}

			// check capacity
			hotel_cap := 0
			var nums []number
			q = fmt.Sprintf("SELECT * from number where hotelid='%s';", hotelId)
			err = s.dbc.Query(q, &nums)
			if err != nil {
				return err
			}
			if len(nums) == 0 {
				return fmt.Errorf("Unknown %v\n", hotelId)
			}
			hotel_cap = int(nums[0].Number)
			if count+int(req.Number) > hotel_cap {
				break
			}
			// indate = outdate
			if inDate.Equal(outDate) {
				res.HotelIds = append(res.HotelIds, hotelId)
			}
		}
	}

	return nil
}
