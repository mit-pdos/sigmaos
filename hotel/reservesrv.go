package hotel

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"sigmaos/dbclnt"
	"sigmaos/fs"
	"sigmaos/fslib"
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
	*fslib.FsLib
	dbc *dbclnt.DbClnt
}

func RunReserveSrv(n string) error {
	r := &Reserve{}
	r.FsLib = fslib.MakeFsLib(n)
	dbc, err := dbclnt.MkDbClnt(r.FsLib, np.DBD)
	if err != nil {
		return err
	}
	r.dbc = dbc
	err = r.initDb()
	if err != nil {
		return err
	}
	protdevsrv.Run(np.HOTELRESERVE, r.mkStream)
	return nil
}

type StreamReserve struct {
	rep     []byte
	reserve *Reserve
}

func (reserve *Reserve) mkStream() (fs.File, *np.Err) {
	st := &StreamReserve{}
	st.reserve = reserve
	return st, nil
}

// XXX wait on close before processing data?
func (st *StreamReserve) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	var args ReserveRequest
	err := json.Unmarshal(b, &args)
	log.Printf("reserve %v\n", args)
	res, err := st.reserve.MakeReservation(&args)
	if err != nil {
		return 0, np.MkErrError(err)
	}
	st.rep, err = json.Marshal(res)
	if err != nil {
		return 0, np.MkErrError(err)
	}
	return np.Tsize(len(b)), nil
}

// XXX incremental read
func (st *StreamReserve) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if len(st.rep) == 0 || off > 0 {
		return nil, nil
	}
	return st.rep, nil
}

func (s *Reserve) initDb() error {
	q := fmt.Sprintf("truncate number;")
	_, err := s.dbc.Query(q)
	if err != nil {
		return err
	}
	q = fmt.Sprintf("truncate reservation;")
	_, err = s.dbc.Query(q)
	if err != nil {
		return err
	}
	q = fmt.Sprintf("INSERT INTO number (hotelid, number) VALUES ('%v', '%v');", "1", 200)
	_, err = s.dbc.Query(q)
	if err != nil {
		return err
	}
	return nil
}

// MakeReservation makes a reservation based on given information
func (s *Reserve) MakeReservation(req *ReserveRequest) (*ReserveResult, error) {
	res := new(ReserveResult)
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
		q := fmt.Sprintf("SELECT * from reservation where hotelid='%s';", hotelId)
		b, err := s.dbc.Query(q)
		if err != nil {
			return nil, fmt.Errorf("Unknown %v\n", hotelId)
		}
		var reserves []reservation
		err = json.Unmarshal(b, &reserves)
		if err != nil {
			return nil, err
		}
		log.Printf("reserves %v\n", reserves)
		for _, r := range reserves {
			count += r.Number
		}
		// check capacity
		hotel_cap := 0
		q = fmt.Sprintf("SELECT * from number where hotelid='%s';", hotelId)
		b, err = s.dbc.Query(q)
		if err != nil {
			return nil, fmt.Errorf("Unknown %v\n", hotelId)
		}
		var nums []number
		err = json.Unmarshal(b, &nums)
		if err != nil {
			return nil, err
		}
		if len(nums) == 0 {
			return nil, fmt.Errorf("Unknown %v\n", hotelId)
		}

		hotel_cap = 200
		if count+int(req.Number) > hotel_cap {
			return res, nil
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
		_, err := s.dbc.Query(q)
		if err != nil {
			return nil, fmt.Errorf("Insert failed %v\n", req)
		}
		indate = outdate
	}

	res.HotelIds = append(res.HotelIds, hotelId)

	return res, nil
}

// // CheckAvailability checks if given information is available
// func (s *Reserve) CheckAvailability(req *ReserveRequest) (*ReserveResult, error) {
// 	res := new(pb.Result)
// 	res.HotelId = make([]string, 0)

// 	c := session.DB("reservation-db").C("reservation")
// 	c1 := session.DB("reservation-db").C("number")

// 	for _, hotelId := range req.HotelId {
// 		inDate, _ := time.Parse(
// 			time.RFC3339,
// 			req.InDate+"T12:00:00+00:00")

// 		outDate, _ := time.Parse(
// 			time.RFC3339,
// 			req.OutDate+"T12:00:00+00:00")

// 		indate := inDate.String()[0:10]

// 		for inDate.Before(outDate) {
// 			// check reservations
// 			count := 0
// 			inDate = inDate.AddDate(0, 0, 1)
// 			log.Trace().Msgf("reservation check date %s", inDate.String()[0:10])
// 			outdate := inDate.String()[0:10]

// 			// memcached miss
// 			reserve := make([]reservation, 0)
// 			err := c.Find(&bson.M{"hotelId": hotelId, "inDate": indate, "outDate": outdate}).All(&reserve)
// 			if err != nil {
// 				log.Panic().Msgf("Tried to find hotelId [%v] from date [%v] to date [%v], but got error", hotelId, indate, outdate, err.Error())
// 			}
// 			for _, r := range reserve {
// 				log.Trace().Msgf("reservation check reservation number = %d", hotelId)
// 				count += r.Number
// 			}

// 			// update memcached
// 			s.MemcClient.Set(&memcache.Item{Key: memc_key, Value: []byte(strconv.Itoa(count))})

// 			// check capacity
// 			hotel_cap := 0
// 			var num number
// 			err = c1.Find(&bson.M{"hotelId": hotelId}).One(&num)
// 			if err != nil {
// 				log.Panic().Msgf("Tried to find hotelId [%v], but got error", hotelId, err.Error())
// 			}
// 			hotel_cap = int(num.Number)
// 			// update memcached
// 			s.MemcClient.Set(&memcache.Item{Key: memc_cap_key, Value: []byte(strconv.Itoa(hotel_cap))})

// 			if count+int(req.RoomNumber) > hotel_cap {
// 				break
// 			}
// 			indate = outdate

// 			if inDate.Equal(outDate) {
// 				res.HotelId = append(res.HotelId, hotelId)
// 			}
// 		}
// 	}

// 	return res, nil
// }
