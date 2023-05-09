package socialnetwork

import (
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/protdevclnt"
	"sigmaos/fs"
	"sigmaos/socialnetwork/proto"
	"regexp"
)

// YH:
// Text service for social network
// No db or cache connection. 

const (
	TEXT_CACHE_PREFIX = "text_"
	TEXT_QUERY_OK = "OK"
)


var mentionRegex = regexp.MustCompile("@[a-zA-Z0-9-_]+") 
var urlRegex = regexp.MustCompile("(http://|https://)([a-zA-Z0-9_!~*'().&=+$%-/]+)")

type TextSrv struct {
	userc  *protdevclnt.ProtDevClnt
	urlc   *protdevclnt.ProtDevClnt
}

func RunTextSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_TEXT, "Creating text service\n")
	tsrv := &TextSrv{}
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_TEXT, tsrv, public)
	if err != nil {
		return err
	}
	pdc, err := protdevclnt.MkProtDevClnt(pds.SigmaClnt().FsLib, sp.SOCIAL_NETWORK_USER)
	if err != nil {
		return err
	}
	tsrv.userc = pdc
	pdc, err = protdevclnt.MkProtDevClnt(pds.SigmaClnt().FsLib, sp.SOCIAL_NETWORK_URL)
	if err != nil {
		return err
	}
	tsrv.urlc = pdc
	dbg.DPrintf(dbg.SOCIAL_NETWORK_TEXT, "Starting text service\n")
	return pds.RunServer()
}

func (tsrv *TextSrv) ProcessText(
		ctx fs.CtxI, req proto.ProcessTextRequest, res *proto.ProcessTextResponse) error {
	if req.Text == "" {
		res.Ok = "Cannot process empty text." 
		return nil
	}
	// process mentions
	mentions := mentionRegex.FindAllString(req.Text, -1)
	for _, mention := range mentions {
		username := mention[1:]
		userArg := proto.CheckUserRequest{Username: username}
		userRes := proto.UserResponse{}
		if err := tsrv.userc.RPC("User.CheckUser", &userArg, &userRes); err != nil {
			return err
		}
		if userRes.Ok != USER_QUERY_OK {
			dbg.DPrintf(dbg.SOCIAL_NETWORK_TEXT, "cannot find mentioned user %v!\n", username)
			continue
		}
		res.Usermentions = append(
			res.Usermentions, &proto.UserRef{Userid: userRes.Userid, Username: username})
	}
	// process urls and text
	urlIndices := urlRegex.FindAllStringIndex(req.Text, -1)
	urlIndicesL := len(urlIndices)
	if urlIndicesL > 0 {
		extendedUrls := make([]string, urlIndicesL)
		for idx, loc := range urlIndices {
			extendedUrls[idx] = req.Text[loc[0]:loc[1]]
		}
		urlArg := proto.ComposeUrlsRequest{Extendedurls: extendedUrls}
		urlRes := proto.ComposeUrlsResponse{}
		if err := tsrv.urlc.RPC("Url.ComposeUrls", &urlArg, &urlRes); err != nil {
			res.Text = req.Text
			return err
		}
		if urlRes.Ok != URL_QUERY_OK {
			dbg.DPrintf(dbg.SOCIAL_NETWORK_TEXT, "cannot process urls %v!\n", extendedUrls)
		} else {
			res.Urls = urlRes.Urls
			res.Text = ""
			prevLoc := 0
			for idx, loc := range urlIndices {
				res.Text += req.Text[prevLoc : loc[0]] + urlRes.Urls[idx].Shorturl
				prevLoc = loc[1]
			}
			res.Text += req.Text[urlIndices[urlIndicesL-1][1]:]
		}
	}
	res.Ok = TEXT_QUERY_OK
	return nil
}
