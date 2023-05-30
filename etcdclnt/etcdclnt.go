package etcdclnt

import (
	"log"
	"time"

	"go.etcd.io/etcd/client/v3"

	db "sigmaos/debug"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

const (
	DialTimeout = 5 * time.Second
	SessionTTL  = 5
)

var (
	Endpoints = []string{"127.0.0.1:2379", "localhost:22379", "localhost:32379"}
)

func SetNamed(cli *clientv3.Client, mnt sp.Tmount) error {
	d, err := mnt.Marshal()
	if err != nil {
		return err
	}
	nf := &NamedFile{Perm: uint32(sp.DMSYMLINK), Data: d}
	if err := PutFile(cli, sessp.Tpath(BOOT), nf); err != nil {
		db.DPrintf(db.NAMEDV1, "SetNamed %v err %v\n", BOOT, err)
		return err
	}
	return nil
}

func GetNamed() (sp.Tmount, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   Endpoints,
		DialTimeout: DialTimeout,
	})
	if err != nil {
		return sp.Tmount{}, err
	}
	defer cli.Close()
	nf, _, sr := GetFile(cli, sessp.Tpath(BOOT))
	if sr != nil {
		db.DPrintf(db.NAMEDV1, "GetFile %v %v err %v\n", BOOT, nf, sr)
		return sp.Tmount{}, sr
	}
	mnt, sr := sp.MkMount(nf.Data)
	if sr != nil {
		db.DPrintf(db.NAMEDV1, "MkMount %v err %v\n", BOOT, err)
		return sp.Tmount{}, sr
	}
	log.Printf("GetNamed mnt %v\n", mnt)
	return mnt, nil
}
