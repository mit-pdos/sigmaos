package snapshot

type ObjSnapshot struct {
	Type Tsnapshot
	Data []byte
}

func MakeObjSnapshot(t Tsnapshot, b []byte) ObjSnapshot {
	return ObjSnapshot{t, b}
}
