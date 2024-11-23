package clnt

import (
	"gopkg.in/mgo.v2/bson"
	dbg "sigmaos/debug"
	"sigmaos/fslib"
	proto "sigmaos/mongo/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	sp "sigmaos/sigmap"
)

type MongoClnt struct {
	rpcc *rpcclnt.RPCClnt
}

func NewMongoClntWithName(fsl *fslib.FsLib, pn string) (*MongoClnt, error) {
	mongoc := &MongoClnt{}
	rpcc, err := sprpcclnt.NewRPCClnt(fsl, pn)
	if err != nil {
		return nil, err
	}
	mongoc.rpcc = rpcc
	return mongoc, nil
}

func NewMongoClnt(fsl *fslib.FsLib) (*MongoClnt, error) {
	return NewMongoClntWithName(fsl, sp.MONGO+sp.ANY+"/")
}

func (mongoc *MongoClnt) Insert(db, collection string, obj interface{}) error {
	objEncoded, err := bson.Marshal(obj)
	if err != nil {
		dbg.DPrintf(dbg.MONGO_ERR, "cannot encode insert object: %v. Err: %v", obj, err)
		return err
	}
	req := &proto.MongoRequest{Db: db, Collection: collection, Obj: objEncoded}
	res := &proto.MongoResponse{}
	return mongoc.rpcc.RPC("MongoSrv.Insert", req, res)
}

func (mongoc *MongoClnt) FindOne(db, collection string, query bson.M, result any) (bool, error) {
	allBytes, err := mongoc.FindAllEncoded(db, collection, query)
	if err != nil {
		return false, err
	}
	if len(allBytes) > 0 {
		if err := bson.Unmarshal(allBytes[0], result); err != nil {
			dbg.DPrintf(dbg.MONGO_ERR, "cannot decode result:%v", allBytes[0])
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// TODO use reflection to handle find all
func (mongoc *MongoClnt) FindAllEncoded(db, collection string, query bson.M) ([][]byte, error) {
	queryEncoded, _ := bson.Marshal(query)
	req := &proto.MongoRequest{Db: db, Collection: collection, Query: queryEncoded}
	res := &proto.MongoResponse{}
	if err := mongoc.rpcc.RPC("MongoSrv.Find", req, res); err != nil {
		return nil, err
	}
	return res.Objs, nil
}

func (mongoc *MongoClnt) Update(db, collection string, query, update bson.M) error {
	return mongoc.update(db, collection, query, update, false)
}

func (mongoc *MongoClnt) Upsert(db, collection string, query, update bson.M) error {
	return mongoc.update(db, collection, query, update, true)
}

func (mongoc *MongoClnt) update(db, collection string, query, update bson.M, upsert bool) error {
	qEncoded, err := bson.Marshal(query)
	if err != nil {
		dbg.DPrintf(dbg.MONGO_ERR, "cannot encode query bson %v\n", query)
		return err
	}
	uEncoded, err := bson.Marshal(update)
	if err != nil {
		dbg.DPrintf(dbg.MONGO_ERR, "cannot encode update bson %v\n", update)
		return err
	}
	req := &proto.MongoRequest{Db: db, Collection: collection, Query: qEncoded, Obj: uEncoded}
	res := &proto.MongoResponse{}
	if upsert {
		return mongoc.rpcc.RPC("MongoSrv.Upsert", req, res)
	} else {
		return mongoc.rpcc.RPC("MongoSrv.Update", req, res)
	}
}

func (mongoc *MongoClnt) DropCollection(db, collection string) error {
	req := &proto.MongoConfigRequest{Db: db, Collection: collection}
	res := &proto.MongoResponse{}
	return mongoc.rpcc.RPC("MongoSrv.Drop", req, res)
}

func (mongoc *MongoClnt) RemoveAll(db, collection string) error {
	req := &proto.MongoConfigRequest{Db: db, Collection: collection}
	res := &proto.MongoResponse{}
	return mongoc.rpcc.RPC("MongoSrv.Remove", req, res)
}

func (mongoc *MongoClnt) EnsureIndex(db, collection string, indexkeys []string) error {
	req := &proto.MongoConfigRequest{Db: db, Collection: collection, Indexkeys: indexkeys}
	res := &proto.MongoResponse{}
	return mongoc.rpcc.RPC("MongoSrv.Index", req, res)
}
