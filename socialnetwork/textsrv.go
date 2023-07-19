package socialnetwork

import (
	"fmt"
	"regexp"
	dbg "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	sp "sigmaos/sigmap"
	"sigmaos/socialnetwork/proto"
	"sync"
)

// YH:
// Text service for social network
// No db or cache connection.

const (
	TEXT_QUERY_OK = "OK"
)

var mentionRegex = regexp.MustCompile("@[a-zA-Z0-9-_]+")
var urlRegex = regexp.MustCompile("(http://|https://)([a-zA-Z0-9_!~*'().&=+$%-/]+)")

type TextSrv struct {
	userc *protdevclnt.ProtDevClnt
	urlc  *protdevclnt.ProtDevClnt
}

func RunTextSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_TEXT, "Creating text service\n")
	tsrv := &TextSrv{}
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_TEXT, tsrv, sp.SOCIAL_NETWORK_TEXT, public)
	if err != nil {
		return err
	}
	fsls := MakeFsLibs(sp.SOCIAL_NETWORK_TEXT)
	pdc, err := protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_USER)
	if err != nil {
		return err
	}
	tsrv.userc = pdc
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_URL)
	if err != nil {
		return err
	}
	tsrv.urlc = pdc
	dbg.DPrintf(dbg.SOCIAL_NETWORK_TEXT, "Starting text service\n")
	return pds.RunServer()
}

func (tsrv *TextSrv) ProcessText(
	ctx fs.CtxI, req proto.ProcessTextRequest, res *proto.ProcessTextResponse) error {
	res.Ok = "No. "
	if req.Text == "" {
		res.Ok = "Cannot process empty text."
		return nil
	}
	// find mentions and urls
	mentions := mentionRegex.FindAllString(req.Text, -1)
	mentionsL := len(mentions)
	usernames := make([]string, mentionsL)
	for idx, mention := range mentions {
		usernames[idx] = mention[1:]
	}
	userArg := proto.CheckUserRequest{Usernames: usernames}
	userRes := proto.CheckUserResponse{}

	urlIndices := urlRegex.FindAllStringIndex(req.Text, -1)
	urlIndicesL := len(urlIndices)
	extendedUrls := make([]string, urlIndicesL)
	for idx, loc := range urlIndices {
		extendedUrls[idx] = req.Text[loc[0]:loc[1]]
	}
	urlArg := proto.ComposeUrlsRequest{Extendedurls: extendedUrls}
	urlRes := proto.ComposeUrlsResponse{}

	// concurrent RPC calls
	var wg sync.WaitGroup
	var userErr, urlErr error
	if mentionsL > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			userErr = tsrv.userc.RPC("User.CheckUser", &userArg, &userRes)
		}()
	}
	if urlIndicesL > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			urlErr = tsrv.urlc.RPC("Url.ComposeUrls", &urlArg, &urlRes)
		}()
	}
	wg.Wait()
	res.Text = req.Text
	if userErr != nil || urlErr != nil {
		return fmt.Errorf("%w; %w", userErr, urlErr)
	}

	// process mentions
	for idx, userid := range userRes.Userids {
		if userid > 0 {
			res.Usermentions = append(res.Usermentions, userid)
		} else {
			dbg.DPrintf("User %v does not exist!", usernames[idx])
		}
	}

	// process urls and text
	if urlIndicesL > 0 {
		if urlRes.Ok != URL_QUERY_OK {
			dbg.DPrintf(dbg.SOCIAL_NETWORK_TEXT, "cannot process urls %v!\n", extendedUrls)
			res.Ok += urlRes.Ok
			return nil
		} else {
			res.Urls = urlRes.Shorturls
			res.Text = ""
			prevLoc := 0
			for idx, loc := range urlIndices {
				res.Text += req.Text[prevLoc:loc[0]] + urlRes.Shorturls[idx]
				prevLoc = loc[1]
			}
			res.Text += req.Text[urlIndices[urlIndicesL-1][1]:]
		}
	}
	res.Ok = TEXT_QUERY_OK
	return nil
}
