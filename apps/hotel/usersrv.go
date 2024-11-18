package hotel

import (
	"crypto/sha256"
	"fmt"
	"strconv"

	//	"go.opentelemetry.io/otel/trace"
	//	"context"

	"sigmaos/apps/hotel/proto"
	"sigmaos/dbclnt"
	"sigmaos/fs"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/tracing"
)

const (
	NUSER = 500
)

//type UserRequest struct {
//	Name     string
//	Password string
//}
//
//type UserResult struct {
//	OK string
//}

type User struct {
	Username string
	Password string
}

type Users struct {
	dbc    *dbclnt.DbClnt
	tracer *tracing.Tracer
}

func RunUserSrv(n string) error {
	u := &Users{}
	ssrv, err := sigmasrv.NewSigmaSrv(HOTELUSER, u, proc.GetProcEnv())
	if err != nil {
		return err
	}
	dbc, err := dbclnt.NewDbClnt(ssrv.MemFs.SigmaClnt().FsLib, sp.DBD)
	if err != nil {
		return err
	}
	u.dbc = dbc
	err = u.initDB()
	if err != nil {
		return err
	}
	//	u.tracer = tracing.Init("user", proc.GetSigmaJaegerIP())
	//	defer u.tracer.Flush()
	return ssrv.RunServer()
}

func NewPassword(u string) string {
	p := ""
	for j := 0; j < 10; j++ {
		p += u
	}
	return p
}

func (s *Users) initDB() error {
	q := fmt.Sprintf("truncate user;")
	err := s.dbc.Exec(q)
	if err != nil {
		return err
	}
	for i := 0; i <= NUSER; i++ {
		suffix := strconv.Itoa(i)
		u := "Cornell_" + suffix
		p := NewPassword(suffix)
		sum := sha256.Sum256([]byte(p))
		q = fmt.Sprintf("INSERT INTO user (username, password) VALUES ('%v', '%x');", u, sum)
		err = s.dbc.Exec(q)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Users) CheckUser(ctx fs.CtxI, req proto.UserRequest, res *proto.UserResult) error {
	//	var sctx context.Context
	//	var span trace.Span
	//	if TRACING {
	//		sctx, span = s.tracer.StartRPCSpan(&req, "CheckUser")
	//		defer span.End()
	//	}

	q := fmt.Sprintf("SELECT * from user where username='%s';", req.Name)
	var users []User
	//	var dbspan trace.Span
	//	if TRACING {
	//		_, dbspan = s.tracer.StartContextSpan(sctx, "db.Query")
	//	}
	error := s.dbc.Query(q, &users)
	//	if TRACING {
	//		dbspan.End()
	//	}
	res.OK = "False"
	if error != nil {
		return error
	}
	if len(users) == 0 {
		return fmt.Errorf("Unknown user %v", req.Name)
	}
	sum := sha256.Sum256([]byte(req.Password))
	if fmt.Sprintf("%x", sum) != users[0].Password {
		return fmt.Errorf("Wrong password")
	}
	res.OK = "True"
	return nil
}
