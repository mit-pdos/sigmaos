package socialnetwork

import (
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/cacheclnt"
	"sigmaos/dbclnt"
	"sigmaos/fs"
	"sigmaos/socialnetwork/proto"
	"fmt"
	"strings"
	"math/rand"
	"time"
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
	dbc    *dbclnt.DbClnt
}

func RunUrlSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Creating url service\n")
	urlsrv := &UrlSrv{}
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_URL, urlsrv, public)
	if err != nil {
		return err
	}
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.SigmaClnt().FsLib, sp.DBD)
	if err != nil {
		return err
	}
	urlsrv.dbc = dbc
	fsls := MakeFsLibs(sp.SOCIAL_NETWORK_URL, pds.MemFs.SigmaClnt().FsLib)
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
	res.Urls = make([]*proto.Url, nUrls)
	q := "INSERT INTO socialnetwork_url (shorturl, extendedurl) VALUES"
	for idx, extendedurl := range req.Extendedurls {
		shorturl := RandStringRunes(URL_LENGTH)
		q += fmt.Sprintf(" ('%v', '%v'),", shorturl, extendedurl)
		res.Urls[idx] = &proto.Url{Extendedurl: extendedurl, Shorturl: URL_HOSTNAME + shorturl}
	} 
	// remove the last ","
	q = q[0:len(q)-1]
	if err := urlsrv.dbc.Exec(q); err != nil {
		res.Ok = "DB Failure."
		return nil
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
	url := proto.Url{}
	if err := urlsrv.cachec.Get(key, &url); err != nil {
		if !urlsrv.cachec.IsMiss(err) {
			return "", err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Url %v cache miss\n", key)
		q := fmt.Sprintf("SELECT * from socialnetwork_url where shorturl='%v';", urlKey)
		var urls []proto.Url
		if err := urlsrv.dbc.Query(q, &urls); err != nil {
			return "", err
		}
		if len(urls) == 0 {
			return "", nil
		}
		url = urls[0]
		dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Found %v for %v in DB.", url, urlKey)
		urlsrv.cachec.Put(key, &url)
	} else {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_URL, "Found %v in cache!\n", url)
	}
	return url.Extendedurl, nil
}

