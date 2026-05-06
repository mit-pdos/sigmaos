package blink

const (
	BLINK_PORT = 50055

	BLINK_RESULTS = "/home/arielck/blink-ae/results/blink.recent"

	JUNCTION_RUN    = "/home/arielck/blink-ae/junction/build/junction/junction_run"
	JUNCTION_CONFIG = BLINK_RESULTS + "/junction.config"
	JUNCTION_CHROOT = "/home/arielck/blink-ae/chroot"

	CHROOT_MOUNT_SCRIPT  = "/home/arielck/blink-ae/scripts/chroot_mount.sh"
	CALADAN_SETUP_SCRIPT = "/home/arielck/blink-ae/junction/lib/caladan/scripts/setup_machine.sh"
	IOKERNELD_BIN        = "/home/arielck/blink-ae/junction/lib/caladan/iokerneld"

	SNAPSHOT_JIF_SUFFIX = "_itrees_ord_reorder.jif"
)
