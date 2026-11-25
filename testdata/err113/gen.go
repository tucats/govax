//go:build ignore

// gen.go writes round 2's LINK sweep (testdata/err113's README, "Round 2"):
// LNK113D00.MAR to LNK113D15.MAR, sixteen shareable images whose debug
// symbol tables' records (the traceback records, then the fixup records VMS's
// LINK starts on a block of their own) total sixteen successive byte counts
// around a multiple of 512. Whether VMS's LINK reserves for them the blocks
// the sum fills, rounded up, and one more, or rounded down and two more, shows
// only where the sum is a multiple of 512 (docs/planning/PHASE-113 - Fortran
// errata.md, subtask 13g). Each image has the same routines but one, whose
// name grows by a character from image to image, and so its DST by a byte.
//
// It builds each image with govax's MACRO and LINK to find the sums, so that
// the sixteen straddle a multiple of 512 (VMS's records have been govax's in
// every round), and writes LNK113D.COM, which builds them on VMS. Run from
// the repository root:
//
//	go run testdata/err113/gen.go
//
// The VAX's images come back whole, and the test reads the sums from them, so
// nothing rests on govax's sums being VMS's.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/vmsimage"
)

const (
	dir    = "testdata/err113"
	images = 16
)

// routines is how many routines every image has, chosen by main so that
// the sweep straddles a multiple of 512.
var routines int

func main() {
	// Find the base pad: the one whose image's sum is just below a
	// multiple of 512, so the sweep's sums are 512m-8 to 512m+7.
	for routines = 20; routines < 80; routines++ {
		for pad := 1; pad <= 31-images; pad++ {
			sum, err := sumFor(pad)
			if err != nil {
				log.Fatal(err)
			}

			if (sum+8)%512 == 0 {
				write(pad)

				return
			}
		}
	}

	log.Fatal("no routine count and pad put the sweep around a multiple of 512")
}

// source is image i's MACRO source for a growing name of pad+i characters.
func source(name string, grow int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\t.TITLE\t%s\tone of round 2's DST sweep (testdata/err113's gen.go)\n", name)
	b.WriteString("\t.PSECT\t$CODE, PIC,USR,CON,REL,LCL,SHR,EXE,RD,NOWRT,LONG\n")
	fmt.Fprintf(&b, "\t.ENTRY\tG%s,^M<>\n\tRET\n", strings.Repeat("X", grow))

	for r := 1; r < routines; r++ {
		fmt.Fprintf(&b, "\t.ENTRY\tROUTINE%02d,^M<>\n\tRET\n", r)
	}

	b.WriteString("\t.END\n")

	return b.String()
}

// sumFor is the DST's record bytes for a growing name of pad characters, as
// govax's MACRO and LINK lay it out.
func sumFor(pad int) (int, error) {
	a := asm.New(false)
	a.SetDialect(asm.DialectMACRO)

	if _, err := a.Assemble(source("LNK113DX", pad)); err != nil {
		var list *asm.Errors
		if errors.As(err, &list) {
			err = errors.Join(list.List...)
		}

		return 0, err
	}

	m, err := a.Object(asm.ObjectOptions{Created: time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		return 0, err
	}

	img, err := link.Link([]link.Input{{File: "LNK113DX.OBJ", Module: m}}, link.Options{
		ImageName: "LNK113DX",
		Traceback: true,
		Shareable: true,
		Sources:   []link.SymbolSource{dclcmd.GovaxSymbols(nil)},
	})
	if err != nil {
		return 0, err
	}

	return DSTSum(img.Bytes)
}

// DSTSum is an image's debug symbol table's record bytes: the records up to
// the first of length 0, and the fixup records from the next block on.
func DSTSum(data []byte) (int, error) {
	img, err := vmsimage.ReadImage(data)
	if err != nil {
		return 0, err
	}

	start := int(img.DSTVBN-1) * vmsimage.BlockSize
	dst := data[start : start+int(img.DSTBlockCount())*vmsimage.BlockSize]

	walk := func(at int) int {
		for at < len(dst) && dst[at] != 0 {
			at += int(dst[at]) + 1
		}

		return at
	}

	first := walk(0)
	next := (first + vmsimage.BlockSize - 1) / vmsimage.BlockSize * vmsimage.BlockSize

	return first + walk(next) - next, nil
}

// write writes the sweep's sources and LNK113D.COM.
func write(pad int) {
	var com strings.Builder

	com.WriteString("$ ! LNK113D.COM - round 2's DST sweep (testdata/err113's README), written by gen.go.\n")
	com.WriteString("$ ! Sixteen shareable images whose DST records total successive byte counts.\n")
	com.WriteString("$ SET NOON\n")

	for i := range images {
		name := fmt.Sprintf("LNK113D%02d", i)
		text := source(name, pad+i)

		if err := os.WriteFile(filepath.Join(dir, strings.ToLower(name)+".mar"), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}

		sum, err := sumFor(pad + i)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("%s: %d bytes of records (%d mod 512)\n", name, sum, sum%512)
		fmt.Fprintf(&com, "$ MACRO/NOLIST %s\n$ LINK/NOMAP/SHAREABLE %s\n", name, name)
	}

	com.WriteString("$ DIRECTORY/SIZE=ALL LNK113D*.EXE\n")

	if err := os.WriteFile(filepath.Join(dir, "lnk113d.com"), []byte(com.String()), 0o644); err != nil {
		log.Fatal(err)
	}
}
