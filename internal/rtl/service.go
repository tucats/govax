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

	// noArgs names the services registered with RegisterNoArgs.
	noArgs map[string]bool
}

// NewServiceTable returns an empty ServiceTable.
func NewServiceTable() *ServiceTable {
	return &ServiceTable{entries: map[string]ServiceFunc{}, noArgs: map[string]bool{}}
}

// Register adds fn under name, matching declare_service(name, handler). A
// name not present in p1Vector (p1vector.go) can still be registered here —
// it would simply never be reachable via SystemService's pc-based lookup —
// but every register* call in this package only ever names a real p1Vector
// entry, verified by TestRegisteredServicesExistInP1Vector.
func (t *ServiceTable) Register(name string, fn ServiceFunc) {
	t.entries[name] = fn
}

// RegisterNoArgs adds fn under name, like Register, for a service that
// isn't called with an argument list: SystemService passes it a nil argv
// instead of reading one from AP. The AST exit, SYS$CLRAST, is one: it's
// reached by an AST routine's RET, not by CALLS (ast.go).
func (t *ServiceTable) RegisterNoArgs(name string, fn ServiceFunc) {
	t.Register(name, fn)
	t.noArgs[name] = true
}

// ReadsArgs reports whether the service registered under name takes an
// argument list — true unless it was registered with RegisterNoArgs.
func (t *ServiceTable) ReadsArgs(name string) bool { return !t.noArgs[name] }

// Lookup returns the ServiceFunc registered under name.
func (t *ServiceTable) Lookup(name string) (ServiceFunc, bool) {
	fn, ok := t.entries[name]
	
	return fn, ok
}

// registerServices installs every implemented SYS$ service into t. Each
// register* function lives alongside the routines it registers (service.go
// itself for the service.c ports, devices.go, logicals.go, cli.go, rms.go),
// matching declare_services' own per-service declare_service calls. RMS
// services (SYS$CREATE/SYS$CONNECT/SYS$OPEN/SYS$CLOSE/SYS$GET/SYS$PUT,
// rms.go) are thin wrapper closures over internal/rms's own handlers —
// Phase 10's host-passthrough stopgap that used to implement a subset of
// them directly in this package has been removed outright, not kept as a
// fallback (docs/PHASE-22.md).
func registerServices(t *ServiceTable) {
	registerCoreServices(t)
	registerAddressSpaceServices(t)
	registerPageProtectionServices(t)
	registerProcessServices(t)
	registerPrivilegeServices(t)
	registerEventFlagServices(t)
	registerJPIServices(t)
	registerSYIServices(t)
	registerTimerServices(t)
	registerTimeServices(t)
	registerHibernateServices(t)
	registerASTServices(t)
	registerExitServices(t)
	registerFAOServices(t)
	registerMessageServices(t)
	registerChangeModeServices(t)
	registerConditionServices(t)
	registerDeviceServices(t)
	registerDVIServices(t)
	registerQIOServices(t)
	registerMailboxServices(t)
	registerOperatorServices(t)
	registerLogicalServices(t)
	registerCLIService(t)
	registerRMSServices(t)
}
