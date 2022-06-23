package realm

/*
 * Diagram of the realm fs structure:
 * /
 * |- realmmgr // Realm manager fs.
 * |  |- free-machineds // Control file for free machineds to ask for a realm assignment.
 * |  |- realm-create // Control file to create a realm.
 * |  |- realm-destroy // Control file to destroy a realm.
 * |
 * |- realm-fences
 * |  |- realm-1-fence // Fence/lock used to ensure mutual exclusion when adding/removing machineds to/from a realm.
 * |  |- ...
 * |
 * |- realm-config // Stores a config file for each realm.
 * |  |- realm-1-config
 * |  |- ...
 * |
 * |- machined-config // Stores a config file for each machined.
 * |  |- machined-5-config
 * |  |- ...
 * |
 * |- realm-nameds // Stores symlinks to each realm's named replicas.
 * |  |- realm-1 -> [127.0.0.1:1234,127.0.0.1:4567,...] (named replicas)
 * |  |- ...
 * |  |- ...
 */
