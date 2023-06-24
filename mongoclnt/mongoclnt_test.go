package mongoclnt_test

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
	"sigmaos/test"
	"sigmaos/mongoclnt"
)

type MyObj struct {
	Key string   `bson:key`
	Val []string `bson:val`
}

func TestMongoClnt(t *testing.T) {
	// create a client
	ts := test.MakeTstateAll(t)
	mongoc, err := mongoclnt.MkMongoClnt(ts.FsLib)
	assert.Nil(t, err)
	assert.NotNil(t, mongoc)
	
	// Configure table
	db := "TestDB"
	col := "TestTable"
	mongoc.DropCollection(db, col)
	mongoc.EnsureIndex(db, col, []string{"key"})
	
	// Insert 
	obj := MyObj{"k1", []string{"v1"}}
	assert.Nil(t, mongoc.Insert(db, col, obj))
	
	// Find
	var result MyObj
	f, err := mongoc.FindOne(db, col, bson.M{"key": "k1"}, &result)
	assert.Nil(t, err)
	assert.True(t, f)
	assert.Equal(t, []string{"v1"}, result.Val)
	
	// Update
	var result1 MyObj
	assert.Nil(t, mongoc.Update(db, col, bson.M{"key": "k1"}, bson.M{"$push": bson.M{"val": "v2"}}))
	assert.Nil(t, mongoc.Update(db, col, bson.M{"key": "k1"}, bson.M{"$pull": bson.M{"val": "v1"}}))
	f, err = mongoc.FindOne(db, col, bson.M{"key": "k1"}, &result1)
	assert.Nil(t, err)
	assert.True(t, f)
	assert.Equal(t, []string{"v2"}, result1.Val)
	
	// Upsert
	var result2 MyObj
	assert.Nil(t, mongoc.Upsert(db, col, bson.M{"key": "k2"}, bson.M{"$push": bson.M{"val": "vv1"}}))
	assert.Nil(t, mongoc.Upsert(db, col, bson.M{"key": "k2"}, bson.M{"$push": bson.M{"val": "vv2"}}))
	f, err = mongoc.FindOne(db, col, bson.M{"key": "k2"}, &result2)
	assert.Nil(t, err)
	assert.True(t, f)
	assert.Equal(t, []string{"vv1", "vv2"}, result2.Val)

	// shutdown test system
	assert.Nil(t, ts.Shutdown())
}


