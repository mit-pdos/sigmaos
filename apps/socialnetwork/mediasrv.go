package socialnetwork

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"

	"gopkg.in/mgo.v2/bson"

	"sigmaos/api/fs"
	"sigmaos/apps/cache"
	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	"sigmaos/apps/socialnetwork/proto"
	dbg "sigmaos/debug"
	"sigmaos/proc"
	mongoclnt "sigmaos/proxy/mongo/clnt"
	"sigmaos/sigmasrv"
)

// YH:
// Media Storage service for social network

const (
	MEDIA_QUERY_OK     = "OK"
	MEDIA_CACHE_PREFIX = "media_"
)

type MediaSrv struct {
	mongoc *mongoclnt.MongoClnt
	cachec *cachegrpclnt.CachedSvcClnt
	sid    int32
	ucount int32
	mu     sync.Mutex
}

func RunMediaSrv(jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_MEDIA, "Creating media service\n")
	msrv := &MediaSrv{}
	msrv.sid = rand.Int31n(536870912) // 2^29
	ssrv, err := sigmasrv.NewSigmaSrv(SOCIAL_NETWORK_MEDIA, msrv, proc.GetProcEnv())
	if err != nil {
		return err
	}
	mongoc, err := mongoclnt.NewMongoClnt(ssrv.MemFs.SigmaClnt().FsLib)
	if err != nil {
		return err
	}
	mongoc.EnsureIndex(SN_DB, MEDIA_COL, []string{"mediaid"})
	msrv.mongoc = mongoc
	fsl, err := NewFsLib(SOCIAL_NETWORK_MEDIA, ssrv.MemFs.SigmaClnt().GetDialProxyClnt())
	if err != nil {
		return err
	}
	msrv.cachec = cachegrpclnt.NewCachedSvcClnt(fsl, jobname)
	dbg.DPrintf(dbg.SOCIAL_NETWORK_MEDIA, "Starting media service\n")
	return ssrv.RunServer()
}

func (msrv *MediaSrv) StoreMedia(ctx fs.CtxI, req proto.StoreMediaReq, res *proto.StoreMediaRep) error {
	res.Ok = "No"
	mId := msrv.getNextMediaId()
	newMedia := Media{mId, req.Mediatype, req.Mediadata}
	if err := msrv.mongoc.Insert(SN_DB, MEDIA_COL, newMedia); err != nil {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_MEDIA, "Mongo Error: %v", err)
		return err
	}
	res.Ok = POST_QUERY_OK
	res.Mediaid = mId
	return nil
}

func (msrv *MediaSrv) ReadMedia(ctx fs.CtxI, req proto.ReadMediaReq, res *proto.ReadMediaRep) error {
	res.Ok = "No."
	mediatypes := make([]string, len(req.Mediaids))
	mediadatas := make([][]byte, len(req.Mediaids))
	missing := false
	for idx, mediaid := range req.Mediaids {
		media, err := msrv.getMedia(mediaid)
		if err != nil {
			return err
		}
		if media == nil {
			missing = true
			res.Ok = res.Ok + fmt.Sprintf(" Missing %v.", mediaid)
		} else {
			mediatypes[idx] = media.Type
			mediadatas[idx] = media.Data
		}
	}
	res.Mediatypes = mediatypes
	res.Mediadatas = mediadatas
	if !missing {
		res.Ok = MEDIA_QUERY_OK
	}
	return nil
}

func (msrv *MediaSrv) getMedia(mediaid int64) (*Media, error) {
	key := MEDIA_CACHE_PREFIX + strconv.FormatInt(mediaid, 10)
	media := &Media{}
	cacheItem := &proto.CacheItem{}
	if err := msrv.cachec.Get(key, cacheItem); err != nil {
		if !cache.IsMiss(err) {
			return nil, err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_MEDIA, "Media %v cache miss\n", key)
		found, err := msrv.mongoc.FindOne(SN_DB, MEDIA_COL, bson.M{"mediaid": mediaid}, media)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
		encoded, _ := bson.Marshal(media)
		msrv.cachec.Put(key, &proto.CacheItem{Key: key, Val: encoded})
		dbg.DPrintf(dbg.SOCIAL_NETWORK_MEDIA, "Found media for %v in DB: %v", mediaid, media)
	} else {
		bson.Unmarshal(cacheItem.Val, media)
		dbg.DPrintf(dbg.SOCIAL_NETWORK_MEDIA, "Found media %v in cache!\n", mediaid)
	}
	return media, nil
}

func (msrv *MediaSrv) incCountSafe() int32 {
	msrv.mu.Lock()
	defer msrv.mu.Unlock()
	msrv.ucount++
	return msrv.ucount
}

func (msrv *MediaSrv) getNextMediaId() int64 {
	return int64(msrv.sid)*1e10 + int64(msrv.incCountSafe())
}

type Media struct {
	Mediaid int64  `bson:mediaid`
	Type    string `bson:type`
	Data    []byte `bson:data`
}
