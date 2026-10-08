package rms

import (
	"fmt"
	"strings"

	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// VerifyVolume checks the volume mounted on device for the damage files
// shared by processes could do (Phase 47's stress tests): it writes the
// cached bitmaps back, then reports every block the storage bitmap and
// the file headers disagree about (ods2's ANALYZE/DISK pass), and every
// directory entry whose file ID leads to no file. It returns one line
// per problem; none means the volume is consistent.
func (t *MountTable) VerifyVolume(device string) ([]string, error) {
	vol, ok := t.Lookup(device)
	if !ok {
		return nil, fmt.Errorf("rms: %s: not mounted", device)
	}

	var problems []string

	for _, dev := range vol.Devices {
		bm, ib, err := deviceBitmaps(dev)
		if err != nil {
			return nil, err
		}

		if err := flushBitmaps(bm, ib); err != nil {
			return nil, err
		}

		report, err := volume.AnalyzeDisk(dev)
		if err != nil {
			return nil, err
		}

		for _, d := range report.Discrepancies {
			problems = append(problems, d.String())
		}
	}

	var walk func(fid ondisk.Fid, path string)

	walk = func(fid ondisk.Fid, path string) {
		dir, err := vol.OpenDirectory(fid)
		if err != nil {
			problems = append(problems, fmt.Sprintf("directory %s: %v", path, err))

			return
		}

		entries, err := dir.List()
		if err != nil {
			problems = append(problems, fmt.Sprintf("directory %s: %v", path, err))

			return
		}

		for _, e := range entries {
			f, err := vol.OpenFID(e.Fid)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s%s;%d names no file: %v", path, e.Name, e.Version, err))

				continue
			}

			if f.Header.IsDirectory() && !e.Fid.Equal(fid) && !e.Fid.Equal(ondisk.MasterFileDirectoryFid) {
				walk(e.Fid, path+strings.TrimSuffix(e.Name, ".DIR")+".")
			}
		}
	}

	walk(ondisk.MasterFileDirectoryFid, "")

	return problems, nil
}
