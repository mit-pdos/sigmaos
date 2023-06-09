package graph

import "os"

func ReadGraph(fn string) (*[]byte, error) {
	b, err := os.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// XXX Make sure these are accessible in sigmaOS

var DATA_TINY_FN = "data/tiny.txt"

// I run into problems using a graph this big, so for now I'm not testing with it.
// FACEBOOK is from https://snap.stanford.edu/data/ego-Facebook.html
var DATA_FACEBOOK_FN = "data/facebook.txt"

// TWITCH is from https://snap.stanford.edu/data/twitch_gamers.html
var DATA_TWITCH_FN = "data/twitch.txt"
