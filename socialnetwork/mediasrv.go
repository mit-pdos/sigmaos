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
	"strconv"
	"fmt"
	"sync"
	"math/rand"
)

// YH:
// Media Storage service for social network

const (
	MEDIA_QUERY_OK = "OK"
	MEDIA_CACHE_PREFIX = "media_"
)

type MediaSrv struct {
	dbc    *dbclnt.DbClnt
	cachec *cacheclnt.CacheClnt
	sid    int32
	ucount int32
	mu     sync.Mutex
}

func RunMediaSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_MEDIA, "Creating media service\n")
	msrv := &MediaSrv{}
	msrv.sid = rand.Int31n(536870912) // 2^29
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_MEDIA, msrv, public)
	if err != nil {
		return err
	}
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.SigmaClnt().FsLib, sp.DBD)
	if err != nil {
		return err
	}
	msrv.dbc = dbc
	fsls := MakeFsLibs(sp.SOCIAL_NETWORK_MEDIA, pds.MemFs.SigmaClnt().FsLib)
	cachec, err := cacheclnt.MkCacheClnt(fsls, jobname)
	if err != nil {
		return err
	}
	msrv.cachec = cachec
	dbg.DPrintf(dbg.SOCIAL_NETWORK_MEDIA, "Starting media service\n")
	return pds.RunServer()
}

func (msrv *MediaSrv) StoreMedia(ctx fs.CtxI, req proto.StoreMediaRequest, res *proto.StoreMediaResponse) error {
	res.Ok = "No"
	mId := msrv.getNextMediaId()
	mContent := hex.EncodeToString(req.Mediadata) 
	q := fmt.Sprintf(
		"INSERT INTO socialnetwork_media (mediaid,mediatype,mediacontent) VALUES ('%v','%v','%v')", 
		mId, req.Mediatype, mContent)
	if err := msrv.dbc.Exec(q); err != nil {
		return err
	}
	res.Ok = POST_QUERY_OK
	res.Mediaid = mId
	return nil
}

func (msrv *MediaSrv) ReadMedia(ctx fs.CtxI, req proto.ReadMediaRequest, res *proto.ReadMediaResponse) error {
	res.Ok = "No."
	medias := make([]*proto.Media, len(req.Mediaids))
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
			medias[idx] = media
		}
	}
	res.Medias = medias
	if !missing {
		res.Ok = MEDIA_QUERY_OK
	}
	return nil
}

func (msrv *MediaSrv) getMedia(mediaid int64) (*proto.Media, error) {
	key := MEDIA_CACHE_PREFIX + strconv.FormatInt(mediaid, 10) 
	media := &proto.Media{}
	if err := msrv.cachec.Get(key, media); err != nil {
		if !msrv.cachec.IsMiss(err) {
			return nil, err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_MEDIA, "Media %v cache miss\n", key)
		q := fmt.Sprintf("SELECT * from socialnetwork_media where mediaid='%v';", mediaid)
		var mEncodes []proto.MediaEncode
		if err := msrv.dbc.Query(q, &mEncodes); err != nil {
			return nil, err
		}
		if len(mEncodes) == 0 {
			return nil, nil
		}
		mEncode := &mEncodes[0]
		dbg.DPrintf(dbg.SOCIAL_NETWORK_MEDIA, "Found encoded media for %v in DB\n", mediaid)
		data, err := hex.DecodeString(mEncode.Mediacontent)
		if err != nil {
			return nil, err
		}
		media.Mediadata, media.Mediaid, media.Mediatype = data, mEncode.Mediaid, mEncode.Mediatype 
		msrv.cachec.Put(key, media)
	} else {
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
