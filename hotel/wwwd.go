package hotel

import (
	"encoding/json"
	"log"
	"net/http"

	"sigmaos/fslib"
	np "sigmaos/ninep"
	"sigmaos/procclnt"
	"sigmaos/protdevclnt"
)

type Www struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	userc *protdevclnt.ProtDevClnt
}

// Run starts the server
func RunWww(n string) error {
	www := &Www{}
	www.FsLib = fslib.MakeFsLib(n)
	www.ProcClnt = procclnt.MakeProcClnt(www.FsLib)
	pdc, err := protdevclnt.MkProtDevClnt(www.FsLib, np.HOTELUSER)
	if err != nil {
		return err
	}
	www.userc = pdc
	if err := www.Started(); err != nil {
		return err
	}
	srv := &http.Server{
		Addr:    ":8090",
		Handler: nil,
	}
	http.HandleFunc("/user", www.userHandler)
	return srv.ListenAndServe()
}

func (s *Www) userHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	username := r.FormValue("username")
	password := r.FormValue("password")

	log.Printf("username %v\n", username)

	if username == "" || password == "" {
		http.Error(w, "Please specify username and password", http.StatusBadRequest)
		return
	}

	var res UserResult

	// Check username and password
	err := s.userc.RPC("User.CheckUser", UserRequest{
		Name:     username,
		Password: password,
	}, &res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	str := "Login successfully!"
	if res.OK == "False" {
		str = "Failed. Please check your username and password. "
	}

	reply := map[string]interface{}{
		"message": str,
	}

	json.NewEncoder(w).Encode(reply)
}
