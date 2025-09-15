package cachegrp

const (
	SRVDIR = "servers/"
	BACKUP = "backup/"
)

func Server(n string) string {
	return SRVDIR + n
}

func BackupServer(n string) string {
	return BACKUP + n
}
