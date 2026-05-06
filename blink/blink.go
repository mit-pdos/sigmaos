package blink

const (
	BLINK_PORT = 50055

	BLINK_RESULTS = "/home/arielck/blink-ae/results/blink.recent"

	JUNCTION_RUN    = "/home/arielck/blink-ae/junction/build/junction/junction_run"
	JUNCTION_CONFIG = BLINK_RESULTS + "/junction.config"
	JUNCTION_CHROOT = "/home/arielck/blink-ae/chroot"

	CALADAN_SETUP_SCRIPT = "/home/arielck/blink-ae/junction/lib/caladan/scripts/setup_machine.sh"
	IOKERNELD_BIN        = "/home/arielck/blink-ae/junction/lib/caladan/iokerneld"

	BLINK_PYTHON        = "/home/arielck/blink-ae/bin/venv/bin/python3"
	BLINK_PYTHON_RUNNER = "/home/arielck/blink-ae/functions/python/run.py"
)
