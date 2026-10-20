package vmsdef

import "testing"

// TestBackupMessages checks BACKUP's messages as the probe's log shows
// them (testdata/backup69/vax/backup69.log), and the condition values
// its $STATUS showed (with STS$M_INHIB_MSG, bit 28, taken off).
func TestBackupMessages(t *testing.T) {
	codes := map[uint32]uint32{
		BackupOpenIn:        0x00A3800C,
		BackupOpenIn&^7 | 2: 0x00A3800A,
		BackupOpenOut:       0x00A38012,
		BackupLisInpSav:     0x00A3808C,
		BackupNotSaveSet:    0x00A38104,
		BackupVBNMissing:    0x00A3822A,
		BackupVerifyErr:     0x00A38272,
		BackupNoFiles:       0x00A38400,
	}

	for got, want := range codes {
		if got != want {
			t.Errorf("code %%X%08X, want %%X%08X", got, want)
		}
	}

	cases := []struct {
		code uint32
		args []any
		want string
	}{
		{BackupOpenIn, []any{"DUA1:[SETS]NONE.BCK;"}, "BACKUP-F-OPENIN, error opening DUA1:[SETS]NONE.BCK; as input"},
		{BackupOpenIn&^7 | 2, []any{"DUA1:[CMP]TEXT.TXT;1"}, "BACKUP-E-OPENIN, error opening DUA1:[CMP]TEXT.TXT;1 as input"},
		{BackupOpenOut, []any{"DUA1:[R7]TEXT.TXT;1"}, "BACKUP-E-OPENOUT, error opening DUA1:[R7]TEXT.TXT;1 as output"},
		{BackupLisInpSav, nil, "BACKUP-F-LISINPSAV, /LIST requires save set as input"},
		{BackupNotSaveSet, []any{"DUA1:[B69]TEXT.TXT;2"}, "BACKUP-F-NOTSAVESET, DUA1:[B69]TEXT.TXT;2 is not a BACKUP save set"},
		{BackupNoFiles, []any{"DUA1:[SETS]B69.BCK;"}, "BACKUP-W-NOFILES, no files selected from DUA1:[SETS]B69.BCK;"},
		{BackupVBNMissing, []any{"DUA1:[DB69CUT.B69.SUB1]MEDIUM.DAT;1", 11, 74}, "BACKUP-E-VBNMISSING, DUA1:[DB69CUT.B69.SUB1]MEDIUM.DAT;1 has missing blocks 11 through 74"},
		{BackupVerifyErr, []any{1, "DUA1:[CMP]TEXT.TXT;1"}, "BACKUP-E-VERIFYERR, verification error for block 1 of DUA1:[CMP]TEXT.TXT;1"},
		{BackupCreated, []any{"DUA1:[R1.B69]ACL.TXT;1"}, "BACKUP-S-CREATED, created DUA1:[R1.B69]ACL.TXT;1"},
		{BackupCreDir, []any{"DUA1:[R1.B69]"}, "BACKUP-S-CREDIR, created directory DUA1:[R1.B69]"},
		{BackupCopied, []any{"DUA1:[B69]ACL.TXT;1"}, "BACKUP-S-COPIED, copied DUA1:[B69]ACL.TXT;1"},
		{BackupCompared, []any{"DUA1:[B69]ACL.TXT;1"}, "BACKUP-S-COMPARED, compared DUA1:[B69]ACL.TXT;1"},
		{BackupHeadCopied, []any{"DUA1:[B69]NOBACK.TXT;1"}, "BACKUP-S-HEADCOPIED, copied DUA1:[B69]NOBACK.TXT;1 header"},
		{BackupNoBackup, []any{"DUA1:[B69]NOBACK.TXT;1"}, "BACKUP-I-NOBACKUP, DUA1:[B69]NOBACK.TXT;1 data not copied, file marked NOBACKUP"},
		{BackupXorErrs, []any{1, "DUA1:[SETS]B69HDRD.BCK;1"}, "BACKUP-I-XORERRS, 1 error recovered by redundancy group in DUA1:[SETS]B69HDRD.BCK;1"},
		{BackupXorErrs, []any{2, "X.BCK;1"}, "BACKUP-I-XORERRS, 2 errors recovered by redundancy group in X.BCK;1"},
		{0x00A3F000, nil, "NONAME-W-NOMSG, Message number 00A3F000"},
	}

	for _, c := range cases {
		if got := FormatMessage(c.code, c.args...); got != c.want {
			t.Errorf("FormatMessage(%%X%08X) = %q, want %q", c.code, got, c.want)
		}
	}
}
