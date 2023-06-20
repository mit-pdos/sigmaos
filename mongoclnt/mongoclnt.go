package mongoclnt

import (
	//"reflect"
	"gopkg.in/mgo.v2/bson"
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/mongod/proto"
	"sigmaos/fslib"
	"sigmaos/protdevclnt"
)

type MongoClnt struct {
	pdc *protdevclnt.ProtDevClnt
}

func MkMongoClnt(fsl *fslib.FsLib) (*MongoClnt, error) {
	mongoc := &MongoClnt{}
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{fsl}, sp.MONGOD)
	if err != nil {
		return nil, err
	}
	mongoc.pdc = pdc
	return mongoc, nil
}

func (mongoc *MongoClnt) Insert(db, collection string, obj interface{}) error {
	objEncoded, err := bson.Marshal(obj)
	if err != nil {
		dbg.DFatalf("cannot encode insert object %v\n", obj)
		return err
	} 
	req := &proto.MongoRequest{Db: db, Collection: collection, Obj: objEncoded}
	res := &proto.MongoResponse{}
	return mongoc.pdc.RPC("Mongo.Insert", req, res);
} 

func (mongoc *MongoClnt) FindOne(db, collection string, query bson.M, result any) error {
	allBytes, err := mongoc.FindAllEncoded(db, collection, query)
	if err != nil {
		return err
	}
	if len(allBytes) > 0 {
		if err := bson.Unmarshal(allBytes[0], result); err != nil {
			dbg.DFatalf("cannot decode result:%v", allBytes[0])
			return err
		}
	}
	return nil
} 

//TODO use reflection to handle find all
func (mongoc *MongoClnt) FindAllEncoded(db, collection string, query bson.M) ([][]byte, error) {
	queryEncoded, _ := bson.Marshal(query)
	req := &proto.MongoRequest{Db: db, Collection: collection, Query: queryEncoded}
	res := &proto.MongoResponse{}
	if err := mongoc.pdc.RPC("Mongo.Find", req, res); err != nil {
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
		dbg.DFatalf("cannot encode query bson %v\n", query)
		return err
	} 
	uEncoded, err := bson.Marshal(update)
	if err != nil {
		dbg.DFatalf("cannot encode update bson %v\n", update)
		return err
	}
	req := &proto.MongoRequest{Db: db, Collection: collection, Query: qEncoded, Obj: uEncoded}
	res := &proto.MongoResponse{}
	if upsert {
		return mongoc.pdc.RPC("Mongo.Upsert", req, res);
	} else {
		return mongoc.pdc.RPC("Mongo.Update", req, res);
	}
}

func (mongoc *MongoClnt) DropCollection(db, collection string) error {
	req := &proto.MongoConfigRequest{Db: db, Collection: collection}
	res := &proto.MongoResponse{}
	return mongoc.pdc.RPC("Mongo.Drop", req, res)
}

func (mongoc *MongoClnt) EnsureIndex(db, collection string, indexkeys []string) error {
	req := &proto.MongoConfigRequest{Db: db, Collection: collection, Indexkeys: indexkeys}
	res := &proto.MongoResponse{}
	return mongoc.pdc.RPC("Mongo.Index", req, res)
}

