package socialnetwork

import (
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/cacheclnt"
	"sigmaos/mongoclnt"
	"sigmaos/fs"
	"sigmaos/socialnetwork/proto"
	"fmt"
	"strings"
	"math/rand"
	"time"
	"gopkg.in/mgo.v2/bson"
)

// YH:
// Url service for social network

const (
	URL_CACHE_PREFIX = "url_"
	URL_QUERY_OK = "OK"
	URL_HOSTNAME = "http://short-url/"
	URL_LENGTH = 10
)

var urlPrefixL = len(URL_HOSTNAME)
	
var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func init() {
    rand.Seed(time.Now().UnixNano())
}

func RandStringRunes(n int) string {
    b := make([]rune, n)
    for i := range b {
        b[i] = letterRunes[rand.Intn(len(letterRunes))]
    }
    return string(b)
}

type UrlSrv struct {
	cachec *cacheclnt.CacheClnt
	mongoc *mongoclnt.MongoClnt
}

func RunUrlSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Creating url service\n")
	urlsrv := &UrlSrv{}
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_URL, urlsrv, public)
	if err != nil {
		return err
	}
	mongoc, err := mongoclnt.MkMongoClnt(pds.MemFs.SigmaClnt().FsLib)
	if err != nil {
		return err
	}
	mongoc.EnsureIndex(SN_DB, URL_COL, []string{"shorturl"})
	urlsrv.mongoc = mongoc
	fsls := MakeFsLibs(sp.SOCIAL_NETWORK_URL)
	cachec, err := cacheclnt.MkCacheClnt(fsls, jobname)
	if err != nil {
		return err
	}
	urlsrv.cachec = cachec
	dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Starting url service\n")
	return pds.RunServer()
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
		shorturl := RandStringRunes(URL_LENGTH)
		url := &Url{Extendedurl: extendedurl, Shorturl: shorturl}
		if err := urlsrv.mongoc.Insert(SN_DB, URL_COL, url); err != nil {
			dbg.DFatalf("Mongo error: %v", err)
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
		if !urlsrv.cachec.IsMiss(err) {
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
	Shorturl string    `bson:shorturl`
	Extendedurl string `bson:extendedurl`
}
