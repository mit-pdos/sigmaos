package socialnetwork

import (
	"fmt"
	"gopkg.in/mgo.v2/bson"
	"sigmaos/apps/socialnetwork/proto"
	"sigmaos/apps/cache"
	cachegrpclnt "sigmaos/cachegrp/clnt"
	dbg "sigmaos/debug"
	"sigmaos/fs"
	mongoclnt "sigmaos/mongo/clnt"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/sigmasrv"
	"strconv"
)

// YH:
// Post Storage service for social network
// for now we use sql instead of MongoDB

const (
	POST_QUERY_OK     = "OK"
	POST_CACHE_PREFIX = "post_"
)

type PostSrv struct {
	mongoc *mongoclnt.MongoClnt
	cachec *cachegrpclnt.CachedSvcClnt
}

func RunPostSrv(jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_POST, "Creating post service\n")
	psrv := &PostSrv{}
	ssrv, err := sigmasrv.NewSigmaSrv(SOCIAL_NETWORK_POST, psrv, proc.GetProcEnv())
	if err != nil {
		return err
	}
	mongoc, err := mongoclnt.NewMongoClnt(ssrv.MemFs.SigmaClnt().FsLib)
	if err != nil {
		return err
	}
	mongoc.EnsureIndex(SN_DB, POST_COL, []string{"postid"})
	psrv.mongoc = mongoc
	fsls, err := NewFsLibs(SOCIAL_NETWORK_POST, ssrv.MemFs.SigmaClnt().GetNetProxyClnt())
	if err != nil {
		return err
	}
	psrv.cachec = cachegrpclnt.NewCachedSvcClnt(fsls, jobname)
	dbg.DPrintf(dbg.SOCIAL_NETWORK_POST, "Starting post service\n")
	perf, err := perf.NewPerf(fsls[0].ProcEnv(), perf.SOCIAL_NETWORK_POST)
	if err != nil {
		dbg.DFatalf("NewPerf err %v\n", err)
	}
	defer perf.Done()

	return ssrv.RunServer()
}

func (psrv *PostSrv) StorePost(ctx fs.CtxI, req proto.StorePostRequest, res *proto.StorePostResponse) error {
	res.Ok = "No"
	postBson := postToBson(req.Post)
	if err := psrv.mongoc.Insert(SN_DB, POST_COL, postBson); err != nil {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_POST, "Error storing post %v", err)
		return err
	}
	res.Ok = POST_QUERY_OK
	return nil
}

func (psrv *PostSrv) ReadPosts(ctx fs.CtxI, req proto.ReadPostsRequest, res *proto.ReadPostsResponse) error {
	res.Ok = "No."
	posts := make([]*proto.Post, len(req.Postids))
	missing := false
	for idx, postid := range req.Postids {
		postBson, err := psrv.getPost(postid)
		if err != nil {
			return err
		}
		if postBson == nil {
			missing = true
			res.Ok = res.Ok + fmt.Sprintf(" Missing %v.", postid)
		} else {
			posts[idx] = bsonToPost(postBson)
		}
	}
	res.Posts = posts
	if !missing {
		res.Ok = POST_QUERY_OK
	}
	return nil
}

func (psrv *PostSrv) getPost(postid int64) (*PostBson, error) {
	key := POST_CACHE_PREFIX + strconv.FormatInt(postid, 10)
	postBson := &PostBson{}
	cacheItem := &proto.CacheItem{}
	if err := psrv.cachec.Get(key, cacheItem); err != nil {
		if !cache.IsMiss(err) {
			return nil, err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_POST, "Post %v cache miss\n", key)
		found, err := psrv.mongoc.FindOne(SN_DB, POST_COL, bson.M{"postid": postid}, postBson)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
		encoded, _ := bson.Marshal(postBson)
		psrv.cachec.Put(key, &proto.CacheItem{Key: key, Val: encoded})
		dbg.DPrintf(dbg.SOCIAL_NETWORK_POST, "Found post %v in DB: %v", postid, postBson)
	} else {
		bson.Unmarshal(cacheItem.Val, postBson)
		dbg.DPrintf(dbg.SOCIAL_NETWORK_POST, "Found post %v in cache!\n", postid)
	}
	return postBson, nil
}

func postToBson(post *proto.Post) *PostBson {
	return &PostBson{
		Postid:       post.Postid,
		Posttype:     int32(post.Posttype),
		Timestamp:    post.Timestamp,
		Creator:      post.Creator,
		CreatorUname: post.Creatoruname,
		Text:         post.Text,
		Usermentions: post.Usermentions,
		Medias:       post.Medias,
		Urls:         post.Urls,
	}
}

func bsonToPost(bson *PostBson) *proto.Post {
	return &proto.Post{
		Postid:       bson.Postid,
		Posttype:     proto.POST_TYPE(bson.Posttype),
		Timestamp:    bson.Timestamp,
		Creator:      bson.Creator,
		Creatoruname: bson.CreatorUname,
		Text:         bson.Text,
		Usermentions: bson.Usermentions,
		Medias:       bson.Medias,
		Urls:         bson.Urls,
	}
}

type PostBson struct {
	Postid       int64    `bson:postid`
	Posttype     int32    `bson:posttype`
	Timestamp    int64    `bson:timestamp`
	Creator      int64    `bson:creator`
	CreatorUname string   `bson:creatoruname`
	Text         string   `bson:text`
	Usermentions []int64  `bson:usermentions`
	Medias       []int64  `bson:medias`
	Urls         []string `bson:urls`
}
