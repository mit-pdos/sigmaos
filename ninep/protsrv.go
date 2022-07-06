package ninep

type Isrvconn interface {
}

type Conn interface {
	IsClosed() bool
	Close()
	GetReplyC() chan *Fcall
}

type Fsrvfcall func(*Fcall)

type SessServer interface {
	Register(Tsession, Conn) *Err
	Unregister(Tsession, Conn)
	SrvFcall(*Fcall)
	Snapshot() []byte
	Restore([]byte)
}

type Protsrv interface {
	Version(Tversion, *Rversion) *Rerror
	Auth(Tauth, *Rauth) *Rerror
	Flush(Tflush, *Rflush) *Rerror
	Attach(Tattach, *Rattach) *Rerror
	Walk(Twalk, *Rwalk) *Rerror
	Create(Tcreate, *Rcreate) *Rerror
	Open(Topen, *Ropen) *Rerror
	Watch(Twatch, *Ropen) *Rerror
	Clunk(Tclunk, *Rclunk) *Rerror
	Read(Tread, *Rread) *Rerror
	ReadV(TreadV, *Rread) *Rerror
	Write(Twrite, *Rwrite) *Rerror
	WriteV(TwriteV, *Rwrite) *Rerror
	Remove(Tremove, *Rremove) *Rerror
	RemoveFile(Tremovefile, *Rremove) *Rerror
	Stat(Tstat, *Rstat) *Rerror
	Wstat(Twstat, *Rwstat) *Rerror
	Renameat(Trenameat, *Rrenameat) *Rerror
	GetFile(Tgetfile, *Rgetfile) *Rerror
	SetFile(Tsetfile, *Rwrite) *Rerror
	PutFile(Tputfile, *Rwrite) *Rerror
	Detach(*Rdetach) *Rerror
	Snapshot() []byte
}

type MkProtServer func(SessServer, Tsession) Protsrv
type RestoreProtServer func(SessServer, []byte) Protsrv
