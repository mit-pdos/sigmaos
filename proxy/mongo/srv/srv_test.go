package srv_test

import (
	"context"
	"fmt"
	"sigmaos/test"
	"strconv"
	"testing"
	"time"

	dbg "sigmaos/debug"
	sp "sigmaos/sigmap"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	bson2 "gopkg.in/mgo.v2/bson"
)

type MyObj struct {
	Key string `bson:key`
	Val string `bson:val`
}

func TestConnet(t *testing.T) {
	// Connect to mongo
	mongoUrl := "mongodb://172.17.0.3:27017"
	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoUrl).SetMaxPoolSize(1000))
	assert.Nil(t, err)
	assert.Nil(t, client.Ping(context.TODO(), nil))

	// insert an item
	col := client.Database("myDB").Collection("myTbl")
	col.Drop(context.TODO())
	indexModel := mongo.IndexModel{
		Keys: bson.D{{"key", 1}},
	}
	_, err = col.Indexes().CreateOne(context.TODO(), indexModel)
	assert.Nil(t, err)
	obj := MyObj{"objKey", "objVal"}
	_, err = col.InsertOne(context.TODO(), &obj)
	assert.Nil(t, err)

	// query the item
	var objs []MyObj
	res, err := col.Find(context.TODO(), &bson.M{"key": "objKey1"})
	assert.Nil(t, err)
	assert.Nil(t, res.All(context.TODO(), &objs))
	assert.Equal(t, 0, len(objs))

	res, err = col.Find(context.TODO(), &bson.M{"key": "objKey"})
	assert.Nil(t, err)
	assert.Nil(t, res.All(context.TODO(), &objs))
	assert.Equal(t, 1, len(objs))
	assert.Equal(t, "objVal", objs[0].Val)
}

func TestEncodeDecode(t *testing.T) {
	// Connect to mongo
	fmt.Printf("%v: Start\n", time.Now().String())
	mongoUrl := "mongodb://172.17.0.3:27017"
	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoUrl))
	assert.Nil(t, err)

	// set up index
	fmt.Printf("%v: Clear\n", time.Now().String())
	col := client.Database("myDB").Collection("myTbl")
	col.Drop(context.TODO())
	indexModel := mongo.IndexModel{
		Keys: bson.D{{"key", 1}},
	}
	_, err = col.Indexes().CreateOne(context.TODO(), indexModel)
	assert.Nil(t, err)

	// Insert client encode
	fmt.Printf("%v: Insert\n", time.Now().String())
	insert := MyObj{"objKey", "objVal"}
	insertEncode, err := bson.Marshal(insert)
	assert.Nil(t, err)

	// Insert: Server logic
	var insertServer bson.M
	assert.Nil(t, bson.Unmarshal(insertEncode, &insertServer))
	_, err = col.InsertOne(context.TODO(), &insertServer)
	assert.Nil(t, err)

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
	res, err := col.Find(context.TODO(), &serverM)
	assert.Nil(t, err)
	assert.Nil(t, res.All(context.TODO(), &serverObjs))

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
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	mongoc, err := mongoclnt.NewMongoClnt(ts.FsLib)
	assert.Nil(t, err)
	dbc, err := dbclnt.NewDbClnt(ts.FsLib, sp.DBD)
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
		f, err := mongoc.FindOne(db, col, bson2.M{"key": keys[i]}, &result)
		assert.Nil(t, err)
		assert.True(t, f)
		assert.Equal(t, vals[i], result.Val)
	}
	dbg.DPrintf(dbg.TEST, "Mongo Read Time: %v", time.Since(t0Mongo).Microseconds())

	//MySql write
	dbc.Exec("DROP TABLE test_table")
	dbc.Exec(
		"CREATE TABLE test_table (newey VARCHAR(128), mval VARCHAR(128), PRIMARY KEY (`newey`));")
	t0Sql := time.Now()
	for i := 0; i < N_test; i++ {
		assert.Nil(t, dbc.Exec(fmt.Sprintf(
			"INSERT INTO test_table (newey, mval) VALUES ('%v', '%v')", keys[i], vals[i])))
	}
	dbg.DPrintf(dbg.TEST, "Sql Time: %v", time.Since(t0Sql).Microseconds())
	// MySql read
	t0Sql = time.Now()
	for i := 0; i < N_test; i++ {
		assert.Nil(t, dbc.Exec(fmt.Sprintf(
			"SELECT mval FROM test_table WHERE newey='%v'", keys[i])))
	}
	dbg.DPrintf(dbg.TEST, "Sql Read Time: %v", time.Since(t0Sql).Microseconds())

	// shutdown test system
	assert.Nil(t, ts.Shutdown())

}
