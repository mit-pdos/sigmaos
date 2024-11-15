package crash

type Tselector string

const (
	NAMED_PARTITION = "NAMED_PARTITION"

	SPAWNER_CRASH     = "SPAWNER_CRASH"
	SPAWNER_PARTITION = "SPAWNER_PARTITION"

	MRCOORD_CRASH     = "MRCOORD_CRASH"
	MRCOORD_PARTITION = "MRCOORD_PARTITION"
	MRTASK_CRASH      = "MRTASK_CRASH"
	MRTASK_PARTITION  = "MRTASK_PARTITION"

	KVBALANCER_CRASH     = "KVBALANCER_CRASH"
	KVBALANCER_PARTITION = "KVBALANCER_PARTITION"
	KVMOVER_CRASH        = "KVMOVER_CRASH"
	KVMOVER_PARTITION    = "KVMOVER_PARTITION"
)
