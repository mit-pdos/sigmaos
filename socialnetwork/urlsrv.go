package socialnetwork

import (
	"fmt"
	"gopkg.in/mgo.v2/bson"
	"math/rand"
	"sigmaos/cache"
	"sigmaos/cachedsvcclnt"
	dbg "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/mongoclnt"
	"sigmaos/proc"
	"sigmaos/sigmasrv"
	"sigmaos/socialnetwork/proto"
	"strings"
	"time"
)

// YH:
// Url service for social network

const (
	URL_CACHE_PREFIX = "url_"
	URL_QUERY_OK     = "OK"
	URL_HOSTNAME     = "http://short-url/"
	URL_LENGTH       = 10
)

var urlPrefixL = len(URL_HOSTNAME)

type UrlSrv struct {
	cachec *cachedsvcclnt.CachedSvcClnt
	mongoc *mongoclnt.MongoClnt
	random *rand.Rand
}

func RunUrlSrv(jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Creating url service\n")
	urlsrv := &UrlSrv{}
	ssrv, err := sigmasrv.NewSigmaSrv(SOCIAL_NETWORK_URL, urlsrv, proc.GetProcEnv())
	if err != nil {
		return err
	}
	mongoc, err := mongoclnt.NewMongoClnt(ssrv.MemFs.SigmaClnt().FsLib)
	if err != nil {
		return err
	}
	mongoc.EnsureIndex(SN_DB, URL_COL, []string{"shorturl"})
	urlsrv.mongoc = mongoc
	fsls, err := NewFsLibs(SOCIAL_NETWORK_URL, ssrv.MemFs.SigmaClnt().GetNetProxyClnt())
	if err != nil {
		return err
	}
	cachec, err := cachedsvcclnt.NewCachedSvcClnt(fsls, jobname)
	if err != nil {
		return err
	}
	urlsrv.cachec = cachec
	urlsrv.random = rand.New(rand.NewSource(time.Now().UnixNano()))
	dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Starting url service\n")
	return ssrv.RunServer()
}

func (urlsrv *UrlSrv) ComposeUrls(
	ctx fs.CtxI, req proto.ComposeUrlsRequest, res *proto.ComposeUrlsResponse) error {
	nUrls := len(req.Extendedurls)
	if nUrls == 0 {
		res.Ok = "Empty input"
		return nil
	}
	res.Shorturls = make([]string, nUrls)
	for idx, extendedurl := range req.Extendedurls {
		shorturl := RandString(URL_LENGTH, urlsrv.random)
		url := &Url{Extendedurl: extendedurl, Shorturl: shorturl}
		if err := urlsrv.mongoc.Insert(SN_DB, URL_COL, url); err != nil {
			dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Mongo error: %v", err)
			return err
		}
		res.Shorturls[idx] = URL_HOSTNAME + shorturl
	}
	res.Ok = URL_QUERY_OK
	return nil
}

func (urlsrv *UrlSrv) GetUrls(
	ctx fs.CtxI, req proto.GetUrlsRequest, res *proto.GetUrlsResponse) error {
	res.Ok = "No."
	extendedurls := make([]string, len(req.Shorturls))
	missing := false
	for idx, shorturl := range req.Shorturls {
		extendedurl, err := urlsrv.getExtendedUrl(shorturl)
		if err != nil {
			return err
		}
		if extendedurl == "" {
			missing = true
			res.Ok = res.Ok + fmt.Sprintf(" Missing %v.", shorturl)
		} else {
			extendedurls[idx] = extendedurl
		}
	}
	res.Extendedurls = extendedurls
	if !missing {
		res.Ok = URL_QUERY_OK
	}
	return nil
}

// The following function is not optimized. But it's not used yet.
func (urlsrv *UrlSrv) getExtendedUrl(shortUrl string) (string, error) {
	if !strings.HasPrefix(shortUrl, URL_HOSTNAME) {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Url %v does not start with %v!", shortUrl, URL_HOSTNAME)
		return "", nil
	}
	urlKey := shortUrl[urlPrefixL:]
	key := URL_CACHE_PREFIX + urlKey
	cacheItem := &proto.CacheItem{}
	url := &Url{}
	if err := urlsrv.cachec.Get(key, cacheItem); err != nil {
		if !cache.IsMiss(err) {
			return "", err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Url %v cache miss\n", key)
		found, err := urlsrv.mongoc.FindOne(SN_DB, URL_COL, bson.M{"shorturl": urlKey}, url)
		if err != nil {
			return "", err
		}
		if !found {
			return "", nil
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Found %v for %v in DB.", url, urlKey)
		encoded, _ := bson.Marshal(url)
		urlsrv.cachec.Put(key, &proto.CacheItem{Key: key, Val: encoded})
	} else {
		bson.Unmarshal(cacheItem.Val, url)
		dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Found %v in cache!\n", cacheItem)
	}
	return url.Extendedurl, nil
}

type Url struct {
	Shorturl    string `bson:shorturl`
	Extendedurl string `bson:extendedurl`
}
