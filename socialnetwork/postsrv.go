package socialnetwork

import (
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/dbclnt"
	"sigmaos/cacheclnt"
	"sigmaos/fs"
	"sigmaos/socialnetwork/proto"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"fmt"
)

// YH:
// Post Storage service for social network
// for now we use sql instead of MongoDB

const (
	POST_QUERY_OK = "OK"
	POST_CACHE_PREFIX = "post_"
)

type PostSrv struct {
	dbc    *dbclnt.DbClnt
	cachec *cacheclnt.CacheClnt
}

func RunPostSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_POST, "Creating post service\n")
	psrv := &PostSrv{}
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_POST, psrv, public)
	if err != nil {
		return err
	}
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.SigmaClnt().FsLib, sp.DBD)
	if err != nil {
		return err
	}
	psrv.dbc = dbc
	cachec, err := cacheclnt.MkCacheClnt(pds.MemFs.SigmaClnt().FsLib, jobname)
	if err != nil {
		return err
	}
	psrv.cachec = cachec
	dbg.DPrintf(dbg.SOCIAL_NETWORK_POST, "Starting post service\n")
	return pds.RunServer()
}

func (psrv *PostSrv) StorePost(ctx fs.CtxI, req proto.StorePostRequest, res *proto.StorePostResponse) error {
	res.Ok = "No"
	post := req.Post
	encode, err := EncodePost(*post) 
	if err != nil {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_POST, "Error enconding post %v\n", post)
		return err
	} 
	q := fmt.Sprintf(
		"INSERT INTO socialnetwork_post (postid, postcontent) VALUES ('%v', '%v')", 
		post.Postid, encode)
	if err = psrv.dbc.Exec(q); err != nil {
		res.Ok = "DB Failure."
		return nil
	}
	res.Ok = POST_QUERY_OK
	return nil
}

func (psrv *PostSrv) ReadPosts(ctx fs.CtxI, req proto.ReadPostsRequest, res *proto.ReadPostsResponse) error {
	res.Ok = "No."
	posts := make([]*proto.Post, len(req.Postids))
	missing := false
	for idx, postid := range req.Postids {
		post, err := psrv.getPost(postid)
		if err != nil {
			return err
		} 
		if post == nil {
			missing = true
			res.Ok = res.Ok + fmt.Sprintf(" Missing %v.", postid)
		} else {
			posts[idx] = post
		}
	}
	res.Posts = posts
	if !missing {
		res.Ok = POST_QUERY_OK
	}
	return nil
}


func EncodePost(post proto.Post) (string, error) {
	postBytes, err := json.Marshal(post)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(postBytes), nil
}

func DecodePost(encoded string, postDecoded *proto.Post) error {
	postBytesDecoded, err := hex.DecodeString(encoded)
	if err != nil {
		return err
	}
	return json.Unmarshal(postBytesDecoded, postDecoded)
}

func (psrv *PostSrv) getPost(postid int64) (*proto.Post, error) {
	key := POST_CACHE_PREFIX + strconv.FormatInt(postid, 10) 
	post := &proto.Post{}
	if err := psrv.cachec.Get(key, post); err != nil {
		if !psrv.cachec.IsMiss(err) {
			return nil, err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_POST, "Post %v cache miss\n", key)
		q := fmt.Sprintf("SELECT * from socialnetwork_post where postid='%v';", postid)
		var postEncodes []proto.PostEncode
		if err := psrv.dbc.Query(q, &postEncodes); err != nil {
			return nil, err
		}
		if len(postEncodes) == 0 {
			return nil, nil
		}
		postEncode := &postEncodes[0]
		dbg.DPrintf(dbg.SOCIAL_NETWORK_POST, "Found encoded post %v in DB: %v\n", postid, postEncode)
		if err := DecodePost(postEncode.Postcontent, post); err != nil {
			return nil, err
		}	
		psrv.cachec.Put(key, post)
	} else {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_POST, "Found post %v in cache!\n", postid)
	}
	return post, nil
}
