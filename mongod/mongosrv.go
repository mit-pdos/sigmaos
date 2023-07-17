package mongod

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/protdevsrv"
	"sigmaos/mongod/proto"
	"time"
)

const (
	MONGO_NO = "No"
	MONGO_OK = "OK"
	DIAL_TIMEOUT_SEC = 1
	SOCKET_TIMEOUT_MIN = 5
	SYNC_TIMEOUT_SEC = 10

)

type Server struct {
	session *mgo.Session
}

func makeServer(mongodUrl string) (*Server, error) {
	s := &Server{}
	session, err := mgo.DialWithTimeout(mongodUrl, DIAL_TIMEOUT_SEC * time.Second)
	if err != nil {
		dbg.DFatalf("mongo dial err %v\n", err)
		return nil, err
	}
	session.SetSocketTimeout(SOCKET_TIMEOUT_MIN * time.Minute)
	session.SetSyncTimeout(SYNC_TIMEOUT_SEC * time.Second)
	s.session = session
	if err = s.session.Ping(); err != nil {
		dbg.DFatalf("mongo ping err %v\n", err)
	}
	return s, nil
}

func RunMongod(mongodUrl string) error {
	dbg.DPrintf(dbg.MONGO, "Making mongo proxy server at %v", mongodUrl)
	s, err := makeServer(mongodUrl)
	if err != nil {
		return err
	}
	dbg.DPrintf(dbg.MONGO, "Starting mongo proxy server")
	pds, err := protdevsrv.MakeProtDevSrv(sp.MONGO, s)
	if err != nil {
		return err
	}
	return pds.RunServer()
}

func (s *Server) Insert(ctx fs.CtxI, req proto.MongoRequest, res *proto.MongoResponse) error {
	res.Ok = MONGO_NO
	var m bson.M
	if err := bson.Unmarshal(req.Obj, &m); err != nil {
		dbg.DPrintf(dbg.MONGO_ERR, "Cannot decode insert request: %v", err)
		return err
	}
	dbg.DPrintf(dbg.MONGO, "Received insert request: %v, %v, %v", req.Db, req.Collection, m)
	if err := s.session.DB(req.Db).C(req.Collection).Insert(&m); err != nil {
		dbg.DPrintf(dbg.MONGO_ERR, "Cannot insert: %v", err)
		return err
	}
	res.Ok = MONGO_OK
	return nil
}

func (s *Server) Update(ctx fs.CtxI, req proto.MongoRequest, res *proto.MongoResponse) error {
	return s.update(ctx, req, res, false)
}

func (s *Server) Upsert(ctx fs.CtxI, req proto.MongoRequest, res *proto.MongoResponse) error {
	return s.update(ctx, req, res, true)
}

func (s *Server) update(
		ctx fs.CtxI, req proto.MongoRequest, res *proto.MongoResponse, upsert bool) error {
	res.Ok = MONGO_NO
	rpcName := "update"
	if upsert {
		rpcName = "upsert"
	}
	var q, u bson.M
	if err := bson.Unmarshal(req.Query, &q); err != nil {
		dbg.DPrintf(dbg.MONGO_ERR, "Cannot decode query in %v request: %v", rpcName, err)
		return err
	}
	if err := bson.Unmarshal(req.Obj, &u); err != nil {
		dbg.DPrintf(dbg.MONGO_ERR, "Cannot decode object in %v request: %v", rpcName, err)
		return err
	}
	dbg.DPrintf(
		dbg.MONGO, "Received %v request: %v, %v, %v, %v", rpcName, req.Db, req.Collection, q, u)
	var err error
	if upsert {
		_, err = s.session.DB(req.Db).C(req.Collection).Upsert(&q, &u)
	} else {
		err = s.session.DB(req.Db).C(req.Collection).Update(&q, &u)
	}
	if err != nil {
		dbg.DPrintf(dbg.MONGO_ERR, "Cannot %v: %v", rpcName, err)
		return err
	}
	res.Ok = MONGO_OK
	return nil
}

func (s *Server) Find(ctx fs.CtxI, req proto.MongoRequest, res *proto.MongoResponse) error {
	res.Ok = MONGO_NO
	var m bson.M
	if err := bson.Unmarshal(req.Query, &m); err != nil {
		dbg.DPrintf(dbg.MONGO_ERR, "Cannot decode find query request: %v", err)
		return err
	}
	dbg.DPrintf(dbg.MONGO, "Received Find request. %v, %v, %v", req.Db, req.Collection, m)
	var objs []bson.M
	if err := s.session.DB(req.Db).C(req.Collection).Find(&m).All(&objs); err != nil {
		dbg.DPrintf(dbg.MONGO_ERR, "Cannot find objects: %v. Err: %v", m, err)
		return err
	}
	res.Objs = make([][]byte, len(objs))
	for i, obj := range objs {
		res.Objs[i], _ = bson.Marshal(obj)
	}
	res.Ok = MONGO_OK
	return nil
}

func (s *Server) Drop(ctx fs.CtxI, req proto.MongoConfigRequest, res *proto.MongoResponse) error {
	dbg.DPrintf(dbg.MONGO, "Received drop request: %v", req)
	res.Ok = MONGO_NO
	if err := s.session.DB(req.Db).C(req.Collection).DropCollection(); err != nil {
		return err
	}
	res.Ok = MONGO_OK
	return nil
}

func (s *Server) Remove(ctx fs.CtxI, req proto.MongoConfigRequest, res *proto.MongoResponse) error {
	dbg.DPrintf(dbg.MONGO, "Received remove request: %v", req)
	res.Ok = MONGO_NO
	if _, err := s.session.DB(req.Db).C(req.Collection).RemoveAll(&bson.M{}); err != nil {
		return err
	}
	res.Ok = MONGO_OK
	return nil
}

func (s *Server) Index(ctx fs.CtxI, req proto.MongoConfigRequest, res *proto.MongoResponse) error {
	dbg.DPrintf(dbg.MONGO, "Received index request: %v", req)
	res.Ok = MONGO_NO
	if err := s.session.DB(req.Db).C(req.Collection).EnsureIndexKey(req.Indexkeys...); err != nil {
		return err
	}
	res.Ok = MONGO_OK
	return nil

}
