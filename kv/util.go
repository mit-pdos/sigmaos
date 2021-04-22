package kv

func prepareName(kv string) string {
	return KVPREPARED + kv
}

func commitName(kv string) string {
	return KVCOMMITTED + kv
}
