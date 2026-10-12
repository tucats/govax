package console

// STOP and the cleanup of processes (docs/PHASE-45.md, subtask 12).

// endOtherProcesses deletes every process but process 1, closing their
// files, before the machine they ran on is replaced or ends (INIT,
// VMINIT, ZERO, and govax's exit). It must run while the old memory is
// still the machine's, since a process's rundown reads its tables.
func (m *Machine) endOtherProcesses() {
	if m.processOne == nil || m.Mem == nil {
		return
	}

	m.processOne.DeleteOtherProcesses()
}
