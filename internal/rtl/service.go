package rtl

// ServiceFunc implements one SYS$ system service (a service.c/devices.c/
// logical_names.c/rms.c function), matching p1_vector.c's own CALLV
// signature: given the argument list already resolved to native longwords,
// return the value call_service() leaves in R0.
type ServiceFunc func(env *Environment, argv []uint32) (uint32, error)

// ServiceTable is a name-keyed registry of ServiceFuncs, the Go equivalent
// of p1_vector.c's declare_service/declare_services — see doc.go's design
// note on why this is a registry rather than a switch. Keyed by name (not
// address) since the name is the stable, human-meaningful key; p1vector.go's
// lookupP1Vector resolves a calling address to a name first.
type ServiceTable struct {
	entries map[string]ServiceFunc
}

// NewServiceTable returns an empty ServiceTable.
func NewServiceTable() *ServiceTable {
	return &ServiceTable{entries: map[string]ServiceFunc{}}
}

// Register adds fn under name, matching declare_service(name, handler). A
// name not present in p1Vector (p1vector.go) can still be registered here —
// it would simply never be reachable via SystemService's pc-based lookup —
// but every register* call in this package only ever names a real p1Vector
// entry, verified by TestRegisteredServicesExistInP1Vector.
func (t *ServiceTable) Register(name string, fn ServiceFunc) {
	t.entries[name] = fn
}

// Lookup returns the ServiceFunc registered under name.
func (t *ServiceTable) Lookup(name string) (ServiceFunc, bool) {
	fn, ok := t.entries[name]
	return fn, ok
}

// registerServices installs every implemented SYS$ service into t. Each
// register* function lives alongside the routines it registers (service.go
// itself for the service.c ports, devices.go, logicals.go, rms.go, cli.go),
// matching declare_services' own per-service declare_service calls.
func registerServices(t *ServiceTable) {
	registerCoreServices(t)
	registerDeviceServices(t)
	registerLogicalServices(t)
	registerRMSServices(t)
	registerCLIService(t)
}
