package console

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements docs/PHASE-30.md's LINK command:
//
//	LINK object[,object...][/HOST] [/EXECUTABLE[=image] | /NOEXECUTABLE] [/[NO]TRACEBACK] [/[NO]SYSLIB]
//
// It reads each object module, links them with internal/link, and writes
// the executable image. The objects and the image can each be a host file
// or a file on a mounted ODS-2 volume, by the same rules as MACRO's source
// and object (rms.Session.Locate). An object named without a type is
// .OBJ, and a later object's bare name is found beside the one before it.
// The image is named after the first object, with the type EXE, unless
// /EXECUTABLE names it.

// LinkOptions is one LINK command.
type LinkOptions struct {
	// Objects are the object file names, and Host an explicit /HOST on
	// them.
	Objects []string
	Host    bool

	// Executable is the image file name from /EXECUTABLE=; "" means the
	// first object's name with the type EXE. NoExecutable is
	// /NOEXECUTABLE: link, and report errors, but write nothing.
	Executable   string
	NoExecutable bool

	// NoTraceback is /NOTRACEBACK: the image doesn't start through
	// SYS$IMGSTA.
	NoTraceback bool

	// NoSysLib is /NOSYSLIB: don't search IMAGELIB.OLB and STARLET.OLB
	// (linksource.go).
	NoSysLib bool
}

// Link links object modules into an executable image.
func (c *Console) Link(opts LinkOptions) error {
	s := c.ContainerSession

	if len(opts.Objects) == 0 {
		return vmserrors.New(vmserrors.CLI_NEEDFILENAME, "LINK")
	}

	var (
		inputs []link.Input
		first  rms.FileLocation
		prev   rms.FileLocation
	)

	for i, name := range opts.Objects {
		var (
			loc rms.FileLocation
			err error
		)

		if i == 0 {
			loc, err = s.Locate(name, opts.Host)
		} else {
			loc, err = s.LocateRelated(name, opts.Host, prev)
		}

		if err != nil {
			return fileFailure(err, name)
		}

		loc = withDefaultType(loc, "OBJ")

		records, found, err := s.ReadRecordFile(loc, rms.VariableRecords)
		if err != nil {
			return fileFailure(err, loc.Name)
		}

		m, err := obj.Decode(records)
		if err != nil {
			return vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s isn't an object module: %w", found.Name, err), found.Name)
		}

		if problems := obj.Check(m); len(problems) > 0 {
			return vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s: %s", found.Name, problems[0]), found.Name)
		}

		inputs = append(inputs, link.Input{File: found.Name, Module: m})

		if i == 0 {
			first = found
		}

		prev = found
	}

	exe, err := outputLocation(s, opts.Executable, first, "EXE")
	if err != nil {
		return fileFailure(err, opts.Executable)
	}

	sources, err := c.linkSources(!opts.NoSysLib)
	if err != nil {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, err, exe.Name)
	}

	img, err := link.Link(inputs, link.Options{
		ImageName: imageName(exe),
		LinkerID:  linkerID(),
		Traceback: !opts.NoTraceback,
		Sources:   sources,
	})
	if err != nil {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, err, exe.Name)
	}

	if opts.NoExecutable {
		return nil
	}

	blocks := make([][]byte, 0, len(img.Bytes)/512)
	for i := 0; i < len(img.Bytes); i += 512 {
		blocks = append(blocks, img.Bytes[i:i+512])
	}

	if _, err := s.CreateRecordFile(exe, rms.ImageBlocks, blocks); err != nil {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, err, exe.Name)
	}

	return nil
}

// imageName is the name an image's header records: its file's name,
// without directory, type, or version, in uppercase, as VMS names are.
func imageName(exe rms.FileLocation) string {
	name := exe.Name
	if exe.Host {
		name = filepath.Base(name)
	} else {
		name, _ = splitVersion(name)
		name = vmsNameType(name)
	}

	if i := strings.IndexByte(name, '.'); i >= 0 {
		name = name[:i]
	}

	return strings.ToUpper(name)
}

// linkerID names govax's LINK in an image header, as real LINK records
// its version ("V11-39"): at most 15 characters.
func linkerID() string {
	id := "govax"
	if BuildVersion != "" {
		id += " V" + BuildVersion
	}

	return id[:min(len(id), 15)]
}
