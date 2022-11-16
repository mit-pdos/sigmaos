package hotel

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/harlow/go-micro-services/data"

	"sigmaos/dbclnt"
	np "sigmaos/ninep"
	"sigmaos/protdevsrv"
)

type ProfileFlat struct {
	Id           string
	Name         string
	PhoneNumber  string
	Description  string
	StreetNumber string
	StreetName   string
	City         string
	State        string
	Country      string
	PostalCode   string
	Lat          float32
	Lon          float32
}

type ProfRequest struct {
	HotelIds []string
	Locale   string
}

type ProfResult struct {
	Hotels []*ProfileFlat
}

type ProfSrv struct {
	dbc *dbclnt.DbClnt
}

func RunProfSrv(n string) error {
	ps := &ProfSrv{}
	pds := protdevsrv.MakeProtDevSrv(np.HOTELPROF, ps)
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.FsLib, np.DBD)
	if err != nil {
		return err
	}
	ps.dbc = dbc
	file := data.MustAsset("data/hotels.json")
	profs := []*Profile{}
	if err := json.Unmarshal(file, &profs); err != nil {
		return err
	}
	ps.initDB(profs)
	return pds.RunServer()
}

// Inserts a flatten profile into db
func (ps *ProfSrv) insertProf(p *Profile) error {
	q := fmt.Sprintf("INSERT INTO profile (hotelid, name, phone, description, streetnumber, streetname, city, state, country, postal, lat, lon) VALUES ('%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%f', '%f');", p.Id, p.Name, p.PhoneNumber, p.Description, p.Address.StreetNumber, p.Address.StreetName, p.Address.City, p.Address.State, p.Address.Country, p.Address.PostalCode, p.Address.Lat, p.Address.Lon)
	if err := ps.dbc.Exec(q); err != nil {
		return err
	}
	return nil
}

func (ps *ProfSrv) getProf(id string) (*ProfileFlat, error) {
	q := fmt.Sprintf("SELECT * from profile where hotelid='%s';", id)
	var profs []ProfileFlat
	if error := ps.dbc.Query(q, &profs); error != nil {
		return nil, error
	}
	if len(profs) == 0 {
		return nil, fmt.Errorf("unknown hotel %s", id)
	}
	return &profs[0], nil
}

func (ps *ProfSrv) initDB(profs []*Profile) error {
	q := fmt.Sprintf("truncate profile;")
	if err := ps.dbc.Exec(q); err != nil {
		return err
	}
	for _, p := range profs {
		if err := ps.insertProf(p); err != nil {
			return err
		}
	}

	// add up to 80 hotels
	for i := 7; i <= 80; i++ {
		p := Profile{
			strconv.Itoa(i),
			"St. Regis San Francisco",
			"(415) 284-40" + strconv.Itoa(i),
			"St. Regis Museum Tower is a 42-story, 484 ft skyscraper in the South of Market district of San Francisco, California, adjacent to Yerba Buena Gardens, Moscone Center, PacBell Building and the San Francisco Museum of Modern Art.",
			&Address{
				"125",
				"3rd St",
				"San Francisco",
				"CA",
				"United States",
				"94109",
				37.7835 + float32(i)/500.0*3,
				-122.41 + float32(i)/500.0*4,
			},
			nil,
		}
		if err := ps.insertProf(&p); err != nil {
			return err
		}
	}

	return nil
}

func (ps *ProfSrv) GetProfiles(req ProfRequest, res *ProfResult) error {
	for _, id := range req.HotelIds {
		p, err := ps.getProf(id)
		if err != nil {
			return err
		}
		res.Hotels = append(res.Hotels, p)
	}
	return nil
}
