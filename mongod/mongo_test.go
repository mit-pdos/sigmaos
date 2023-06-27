package mongod_test

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"fmt"
	"time"
	"sigmaos/dbclnt"
	"sigmaos/mongoclnt"
	"sigmaos/test"
	"strconv"
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
)

type MyObj struct {
	Key string `bson:key`
	Val string `bson:val`
}

func TestConnet(t *testing.T) {
	// Connect to mongo
	mongoUrl := "172.17.0.4:27017"
	session, err := mgo.Dial(mongoUrl)
	assert.Nil(t, err)
	assert.Nil(t, session.Ping())
	
	// insert an item
	col := session.DB("myDB").C("myTbl")
	col.DropCollection()
	col.EnsureIndexKey("key")
	obj := MyObj{"objKey", "objVal"}
	err = col.Insert(&obj)
	assert.Nil(t, err)

	// query the item
	var objs []MyObj
	err = col.Find(&bson.M{"key": "objKey1"}).All(&objs)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(objs))

	err = col.Find(&bson.M{"key": "objKey"}).All(&objs)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(objs))
	assert.Equal(t, "objVal", objs[0].Val)
}

func TestEncodeDecode(t *testing.T) {
	// Connect to mongo
	fmt.Printf("%v: Start\n", time.Now().String())
	mongoUrl := "172.17.0.4:27017"
	session, err := mgo.Dial(mongoUrl)
	assert.Nil(t, err)
	fmt.Printf("%v: Clear\n", time.Now().String())
	col := session.DB("myDB").C("myTbl")
	col.DropCollection()
	col.EnsureIndexKey("key")

	// Insert client encode
	fmt.Printf("%v: Insert\n", time.Now().String())
	insert := MyObj{"objKey", "objVal"}
	insertEncode, err := bson.Marshal(insert)
	assert.Nil(t, err)

	// Insert: Server logic
	var insertServer bson.M
	assert.Nil(t, bson.Unmarshal(insertEncode, &insertServer))
	assert.Nil(t, col.Insert(&insertServer))

	// Find: client encode
	m := bson.M{"key": "objKey"}
	fmt.Printf("%v: original query: %v\n", time.Now().String(), m)
	mEncoded, err := bson.Marshal(m) 
	assert.Nil(t, err)

	// Find: server logic
	var serverM bson.M
	var serverObjs []bson.M
	assert.Nil(t, bson.Unmarshal(mEncoded, &serverM))
	fmt.Printf("%v: server query: %v\n", time.Now().String(), serverM)
	assert.Nil(t, col.Find(&serverM).All(&serverObjs))
	fmt.Printf("%v: server results %v\n", time.Now().String(), serverObjs)
	objsEncoded := make([][]byte, len(serverObjs))
	for i, serverObj := range serverObjs {
		objsEncoded[i], err = bson.Marshal(serverObj)
		assert.Nil(t, err)
	}

	// Find: client decode
	objs := make([]MyObj, len(objsEncoded))
	for i, objEncoded := range objsEncoded {
		var o MyObj
		assert.Nil(t, bson.Unmarshal(objEncoded, &o))
		objs[i] = o
	}
	fmt.Printf("%v: final results %v\n", time.Now().String(), objs)
	assert.Equal(t, 1, len(objs))
	assert.Equal(t, "objVal", objs[0].Val)
}

func TestQuerySpeed(t *testing.T) {
	// create mongo and sql dbs
	ts := test.MakeTstateAll(t)
	mongoc, err := mongoclnt.MkMongoClnt(ts.FsLib)
	assert.Nil(t, err)
	dbc, err := dbclnt.MkDbClnt(ts.FsLib, sp.DBD)
	assert.Nil(t, err)
	assert.NotNil(t, dbc)

	// prepare queries
	N_test := 1000
	keys := make([]string, N_test)
	vals := make([]string, N_test)
	for i := 0; i < N_test; i++ {
		keys[i] = "key" + strconv.Itoa(i)
		vals[i] = "val" + strconv.Itoa(i)
	}
	
	// mongo write
	db := "TestDB"
	col := "TestTable"
	mongoc.DropCollection(db, col)
	mongoc.EnsureIndex(db, col, []string{"key"})
	t0Mongo := time.Now()
	for i := 0; i < N_test; i++ {
		assert.Nil(t, mongoc.Insert(db, col, MyObj{keys[i], vals[i]}))
	}
	dbg.DPrintf(dbg.TEST, "Mongo Write Time: %v", time.Since(t0Mongo).Microseconds())
	// mongo read
	t0Mongo = time.Now()
	for i := 0; i < N_test; i++ {
		var result MyObj
		f, err := mongoc.FindOne(db, col, bson.M{"key": keys[i]}, &result)
		assert.Nil(t, err)
		assert.True(t, f)
		assert.Equal(t, vals[i], result.Val)
	}
	dbg.DPrintf(dbg.TEST, "Mongo Read Time: %v", time.Since(t0Mongo).Microseconds())

	//MySql write
	dbc.Exec("DROP TABLE test_table")
	dbc.Exec(
		"CREATE TABLE test_table (mkey VARCHAR(128), mval VARCHAR(128), PRIMARY KEY (`mkey`));")
	t0Sql := time.Now()
	for i := 0; i < N_test; i++ {
		assert.Nil(t, dbc.Exec(fmt.Sprintf(
			"INSERT INTO test_table (mkey, mval) VALUES ('%v', '%v')", keys[i], vals[i])))
	}
	dbg.DPrintf(dbg.TEST, "Sql Time: %v", time.Since(t0Sql).Microseconds())
	// MySql read
	t0Sql = time.Now()
	for i := 0; i < N_test; i++ {
		assert.Nil(t, dbc.Exec(fmt.Sprintf(
			"SELECT mval FROM test_table WHERE mkey='%v'", keys[i])))
	}
	dbg.DPrintf(dbg.TEST, "Sql Read Time: %v", time.Since(t0Sql).Microseconds())

	// shutdown test system
	assert.Nil(t, ts.Shutdown())

}

