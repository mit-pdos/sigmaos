package main

import (
	"ulambda/realm"
)

func main() {
	r := realm.MakeRealmMgr()
	r.Work()
}
