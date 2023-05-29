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
	endpoints = []string{"127.0.0.1:2379", "localhost:22379", "localhost:32379"}
)

func SetNamed(cli *clientv3.Client, mnt sp.Tmount) error {
	d, err := mnt.Marshal()
	if err != nil {
		return err
	}
	nf := &NamedFile{Perm: uint32(sp.DMSYMLINK), Data: d}
	sr := PutFile(cli, sessp.Tpath(BOOT), nf)
	if sr != nil {
		return sr
	}
	return nil
}

func GetNamed() error {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: DialTimeout,
	})
	if err != nil {
		db.DFatalf("clientv3.New err %v\n", err)
	}
	defer cli.Close()
	nf, _, sr := GetFile(cli, sessp.Tpath(BOOT))
	if sr != nil {
		db.DFatalf("ReadFile %v err %v\n", BOOT, sr)
	}
	mnt, sr := sp.MkMount(nf.Data)
	if sr != nil {
		db.DFatalf("MkMount %v err %v\n", BOOT, sr)
	}
	log.Printf("mnt %v\n", mnt)
	return nil
}
