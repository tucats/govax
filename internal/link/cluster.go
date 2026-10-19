package link

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/obj"
)

// This file is the linker's clusters (the Linker manual, 6.2.2 and 6.3):
// the groups of input files the linker lays out together, each its own
// run of pages in the image.
//
// The cluster list is, in order: a cluster for each CLUSTER= option and
// each shareable image an options file names, in the order the options
// files give them; then the default cluster, which holds every other input
// file; then a cluster for each shareable image the link found in a
// library (IMAGELIB.OLB), in the order their symbols were first referred
// to. Pass 1 reads the files cluster by cluster in that order, so a
// CLUSTER= option's files come before the command line's (testdata/
// link68's EXE1: EXE1B's module before EXE1's).
//
// A psect belongs to a cluster. One that isn't global (LCL) is the
// cluster's own: modules in two clusters that contribute to a psect of
// the same name make two psects (EXE1's two ". ABS ." psects). A global
// one (GBL) is looked for in every cluster up to the current one, and
// defined in the current one if none has it. COLLECT= moves psects into a
// cluster and makes them global; PSECT_ATTR= changes a psect's attributes
// as each module defines it.
//
// Allocation gives each cluster its addresses in two passes through the
// list (6.3.4): first the clusters with a base of their own (CLUSTER='s
// base address, and the default cluster's BASE=), each at its base, then
// the others, each at the lowest free addresses that hold it. A shareable
// image cluster normally gets its addresses from the image activator, at
// run time; but with BASE=, real LINK placed LIBRTL at link time, in the
// second pass, in the gap after the default cluster (EXE1), and resolved
// the references to it then, leaving it no fixup cells. Whether a based
// CLUSTER= without BASE= does the same is unconfirmed; govax doesn't.

// defaultClusterName is the default cluster's, as a map names it.
const defaultClusterName = "DEFAULT_CLUSTER"

// p0Limit is the end of P0 space, the most a cluster can reach.
const p0Limit = 0x40000000

// cluster is one cluster of the image.
type cluster struct {
	name string
	// base is its address, once allocated, and based says CLUSTER= or
	// BASE= gave it one.
	base  uint32
	based bool
	// pfc is its image sections' page fault cluster (CLUSTER='s), and
	// protect says PROTECT=YES protects them.
	pfc     uint32
	protect bool
	// image names a shareable image cluster's image; "" for a user
	// cluster. shared is what the sources say of the image, and placed
	// that the link gave it its addresses.
	image  string
	shared SharedImage
	placed bool
	// psects are the cluster's psects, by name, and collected the names
	// COLLECT= put in it, which it defines when a module does.
	psects    map[string]*psect
	collected map[string]bool
	// list are its psects in the order defined, and sections its image
	// sections, by address.
	list     []*psect
	sections []*section
	// size is its bytes, a whole number of pages.
	size uint32
	// last says it is the cluster at the highest address, whose image
	// sections are marked ISD$V_LASTCLU.
	last bool
}

// user says c is a user cluster, not a shareable image's.
func (c *cluster) user() bool { return c.image == "" }

// newCluster adds a cluster to the end of the list.
func (l *linker) newCluster(name string) *cluster {
	c := &cluster{name: name, psects: map[string]*psect{}, collected: map[string]bool{}}
	l.clusters = append(l.clusters, c)

	return c
}

// clusterNamed returns the user cluster name, or nil.
func (l *linker) clusterNamed(name string) *cluster {
	for _, c := range l.clusters {
		if c.user() && strings.EqualFold(c.name, name) {
			return c
		}
	}

	return nil
}

// imageCluster returns the shareable image name's cluster, or nil.
func (l *linker) imageCluster(name string) *cluster {
	for _, c := range l.clusters {
		if c.image == name {
			return c
		}
	}

	return nil
}

// buildClusters makes the cluster list the options give: their clusters
// and shareable images, then the default cluster, then any cluster a
// COLLECT= names that nothing else made (after the default cluster, as
// the manual has COLLECT= read after the command; unconfirmed).
func (l *linker) buildClusters() {
	for _, o := range l.opts.Clusters {
		if o.Shareable {
			if o.Image != "" && l.imageCluster(o.Image) == nil {
				l.newCluster(o.Image).image = o.Image
			}

			continue
		}

		c := l.clusterNamed(o.Name)
		if c == nil {
			c = l.newCluster(o.Name)
		}

		if o.Base != nil {
			c.base, c.based = pageUp(*o.Base), true
		}

		c.pfc = o.PFC
		c.protect = c.protect || o.Protect
	}

	l.defaultCluster = l.newCluster(defaultClusterName)
	if l.opts.Base != nil {
		l.defaultCluster.base, l.defaultCluster.based = pageUp(*l.opts.Base), true
	}

	for _, o := range l.opts.Collects {
		c := l.clusterNamed(o.Cluster)
		if c == nil {
			c = l.newCluster(o.Cluster)
		}

		c.protect = c.protect || o.Protect

		for _, name := range o.Psects {
			c.collected[strings.ToUpper(name)] = true
		}
	}
}

// clusterOf returns the cluster an input file goes in.
func (l *linker) clusterOf(in *Input) (*cluster, error) {
	if in.Cluster == "" {
		return l.defaultCluster, nil
	}

	if c := l.clusterNamed(in.Cluster); c != nil {
		return c, nil
	}

	return nil, fmt.Errorf("link: %s is in cluster %s, which no CLUSTER= option makes", in.File, in.Cluster)
}

// byCluster returns the inputs in the order pass 1 reads them: cluster by
// cluster, in the list's order, and in their own order within a cluster.
func (l *linker) byCluster(inputs []Input) ([]*Input, error) {
	position := map[*cluster]int{}
	for i, c := range l.clusters {
		position[c] = i
	}

	ordered := make([]*Input, len(inputs))
	where := make(map[*Input]int, len(inputs))

	for i := range inputs {
		c, err := l.clusterOf(&inputs[i])
		if err != nil {
			return nil, err
		}

		ordered[i] = &inputs[i]
		where[&inputs[i]] = position[c]
	}

	sort.SliceStable(ordered, func(i, j int) bool { return where[ordered[i]] < where[ordered[j]] })

	return ordered, nil
}

// psectAttributes returns a psect's attributes and alignment as the link
// takes them: a module's, changed by each PSECT_ATTR= option for the
// psect, and global if a COLLECT= names it (the Linker manual, 6.3.2 and
// 6.3.3.1).
func (l *linker) psectAttributes(name string, flags uint16, align byte) (uint16, byte) {
	for _, a := range l.opts.PsectAttrs {
		if !strings.EqualFold(a.Name, name) {
			continue
		}

		for _, k := range a.Attributes {
			bit, set := psectAttributeBit(k)
			if set {
				flags |= bit
			} else {
				flags &^= bit
			}
		}

		if a.Align != nil {
			align = byte(*a.Align)
		}
	}

	for _, c := range l.clusters {
		if c.collected[strings.ToUpper(name)] {
			flags |= obj.PsectGBL
		}
	}

	return flags, align
}

// psectAttributeBit returns the flag a PSECT_ATTR= keyword sets or (for
// its NO form, or LCL, CON, ABS, USR) clears.
func psectAttributeBit(keyword string) (uint16, bool) {
	switch keyword {
	case "PIC", "NOPIC":
		return obj.PsectPIC, keyword == "PIC"
	case "SHR", "NOSHR":
		return obj.PsectSHR, keyword == "SHR"
	case "WRT", "NOWRT":
		return obj.PsectWRT, keyword == "WRT"
	case "EXE", "NOEXE":
		return obj.PsectEXE, keyword == "EXE"
	case "RD", "NORD":
		return obj.PsectRD, keyword == "RD"
	case "GBL", "LCL":
		return obj.PsectGBL, keyword == "GBL"
	case "OVR", "CON":
		return obj.PsectOVR, keyword == "OVR"
	case "REL", "ABS":
		return obj.PsectREL, keyword == "REL"
	case "VEC", "NOVEC":
		return obj.PsectVEC, keyword == "VEC"
	case "LIB", "USR":
		return obj.PsectLIB, keyword == "LIB"
	}

	return 0, false
}

// findPsect returns the psect name that a module in cluster c, defining
// it with flags, contributes to, defining it if it's new. A global psect
// is looked for in each cluster up to c, where a COLLECT= of it counts as
// its definition; a local one in c only.
func (l *linker) findPsect(c *cluster, name string, flags uint16) *psect {
	key := strings.ToUpper(name)

	for _, k := range l.clusters {
		if flags&obj.PsectGBL == 0 && k != c {
			continue
		}

		if p := k.psects[key]; p != nil {
			return p
		}

		if k.collected[key] {
			return l.definePsect(k, name, flags)
		}

		if k == c {
			break
		}
	}

	return l.definePsect(c, name, flags)
}

// definePsect defines the psect name in cluster c.
func (l *linker) definePsect(c *cluster, name string, flags uint16) *psect {
	p := &psect{name: name, flags: flags, cluster: c}
	c.psects[strings.ToUpper(name)] = p
	c.list = append(c.list, p)
	l.order = append(l.order, p)

	return p
}

// addImageClusters gives each shareable image the modules' symbols are
// in a cluster, after the default cluster, in the order the symbols were
// first referred to, and drops the cluster of an image an options file
// named that nothing uses (unconfirmed: real LINK may keep it).
func (l *linker) addImageClusters() {
	used := map[string]bool{}

	for _, g := range l.bySequence() {
		if !g.defined || g.image == "" || !g.strongRef {
			continue
		}

		used[g.image] = true

		if l.imageCluster(g.image) == nil {
			l.newCluster(g.image).image = g.image
		}
	}

	kept := l.clusters[:0]

	for _, c := range l.clusters {
		if c.user() || used[c.image] {
			kept = append(kept, c)
		}
	}

	l.clusters = kept
}

// layoutCluster lays out a user cluster's image sections from its start:
// its relocatable psects grouped by their significant attributes, each
// group an image section on a page of its own, in Table 6-1's order, and
// the psects in a section in alphabetical order (the Linker manual,
// 6.3.4.1).
func (l *linker) layoutCluster(c *cluster) {
	groups := map[int]*section{}

	for _, p := range c.list {
		if p.flags&obj.PsectREL == 0 {
			continue // an absolute psect's contributions are only offsets
		}

		key := sectionKey(p.flags)
		if groups[key] == nil {
			groups[key] = &section{key: key, cluster: c}
		}

		groups[key].psects = append(groups[key].psects, p)
	}

	keys := make([]int, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}

	sort.Ints(keys)

	var va uint32

	for _, k := range keys {
		s := groups[k]

		sort.SliceStable(s.psects, func(i, j int) bool { return s.psects[i].name < s.psects[j].name })

		va = pageUp(va)
		s.base = va

		for _, p := range s.psects {
			va = alignUp(va, p.align)
			p.base, p.section = va, s
			va += p.length
		}

		if va == s.base {
			continue // nothing in it
		}

		s.pages = (pageUp(va) - s.base) / blockSize
		s.data = make([]byte, s.pages*blockSize)
		s.stored = make([]bool, s.pages)
		c.sections = append(c.sections, s)
	}

	c.size = pageUp(va)
}

// freeList is the virtual addresses no cluster has yet, as page-aligned
// runs in address order (the Linker manual, 6.3.4.2).
type freeList struct{ runs [][2]uint32 }

// take removes [at, at+size) from the list, which must hold it all.
func (f *freeList) take(at, size uint32) bool {
	for i, r := range f.runs {
		if at < r[0] || at+size > r[1] {
			continue
		}

		var keep [][2]uint32

		if at > r[0] {
			keep = append(keep, [2]uint32{r[0], at})
		}

		if at+size < r[1] {
			keep = append(keep, [2]uint32{at + size, r[1]})
		}

		f.runs = append(f.runs[:i], append(keep, f.runs[i+1:]...)...)

		return true
	}

	return false
}

// first returns the lowest address with size bytes free.
func (f *freeList) first(size uint32) (uint32, bool) {
	for _, r := range f.runs {
		if r[1]-r[0] >= size {
			return r[0], true
		}
	}

	return 0, false
}

// FatalError is a link real LINK ends without making an image: its
// messages, as real LINK reports them, the last NOIMGFIL.
type FatalError struct {
	Messages []Message
}

func (e *FatalError) Error() string {
	lines := make([]string, len(e.Messages))
	for i, m := range e.Messages {
		lines[i] = m.String()
	}

	return strings.Join(lines, "\n")
}

// statusNoImage is LINK's condition value for a link that made no image
// (NOIMGFIL), less STS$M_INHIBIT_MSG (testdata/link68's E5: %X106482B2).
const statusNoImage = 0x006482B2

// Status is the error's condition value: its $STATUS, but for the
// inhibit bit.
func (e *FatalError) Status() uint32 { return statusNoImage }

// noImage is the FatalError of a link that stopped with msg.
func noImage(msg Message) *FatalError {
	return &FatalError{Messages: []Message{msg, {'E', "NOIMGFIL", "image file not created"}}}
}

// conflictingMemory is real LINK's error for a cluster whose addresses
// another has (E5).
func conflictingMemory(c *cluster, at uint32) *FatalError {
	return noImage(Message{'E', "CONFMEM", fmt.Sprintf(
		"conflicting virtual memory requirement\n\tat %%X%08X for %d. pages in cluster %s", at, c.size/blockSize, c.name)})
}

// placeClusters gives each cluster its addresses in the manual's two
// passes, then every psect, image section, and symbol its address. The
// cluster at the highest address is the last cluster.
func (l *linker) placeClusters() error {
	free := &freeList{runs: [][2]uint32{{imageBase, p0Limit}}}

	for _, c := range l.clusters {
		switch {
		case !c.user():
			c.shared = l.sharedImage(c.image)

			if c.shared.Based {
				return fmt.Errorf("link: based shareable image %s isn't supported yet", c.image)
			}

		case c.based && c.size > 0:
			if !free.take(c.base, c.size) {
				return conflictingMemory(c, c.base)
			}

			c.placed = true
		}
	}

	for _, c := range l.clusters {
		switch {
		case c.placed:
			continue

		case c.user():
			if c.size == 0 {
				continue
			}

		case l.opts.Base == nil:
			continue // the image activator places it

		default:
			if c.shared.Layout == nil {
				return fmt.Errorf("link: shareable image %s can't be placed at link time (BASE=): its image sections aren't known", c.image)
			}

			c.size = c.shared.size()
		}

		at, ok := free.first(c.size)
		if !ok || !free.take(at, c.size) {
			return conflictingMemory(c, at)
		}

		c.base, c.placed = at, true
	}

	var last *cluster

	for _, c := range l.clusters {
		if c.placed && (last == nil || c.base > last.base) {
			last = c
		}
	}

	if last != nil {
		last.last = true
	}

	// A shareable image the link placed is still in the shareable image
	// list (EXE1's LIBRTL), though nothing refers to it through a cell.
	for _, c := range l.clusters {
		if !c.user() && c.placed {
			l.shared = append(l.shared, &sharedRef{image: c.shared, names: map[uint32]string{}})
		}
	}

	for _, c := range l.clusters {
		if !c.user() {
			continue
		}

		for _, p := range c.list {
			if p.flags&obj.PsectREL != 0 {
				p.base += c.base
			}
		}

		for _, s := range c.sections {
			s.base += c.base
			l.sections = append(l.sections, s)
		}
	}

	sort.SliceStable(l.sections, func(i, j int) bool { return l.sections[i].base < l.sections[j].base })

	l.imageBase = imageBase
	if len(l.sections) > 0 {
		l.imageBase = l.sections[0].base
	}

	return nil
}

// based says some cluster has a base of its own, so the image isn't
// position independent.
func (l *linker) based() bool {
	for _, c := range l.clusters {
		if c.based || c.shared.Based {
			return true
		}
	}

	return false
}

// placedImage returns the cluster of the shareable image name if the link
// placed it, or nil.
func (l *linker) placedImage(name string) *cluster {
	if c := l.imageCluster(name); c != nil && c.placed {
		return c
	}

	return nil
}

// imageOrder puts the shareable images the image refers to in the order
// of their clusters: those an options file named (whose clusters come
// before the default cluster), in its order, before those found in
// libraries, which keep the order they were first referred to in (the
// probe's MAIN1: SHR1 before LIBRTL).
func (l *linker) imageOrder() {
	position := map[string]int{}

	for i, c := range l.clusters {
		if c == l.defaultCluster {
			break
		}

		if c.image != "" {
			position[c.image] = i + 1
		}
	}

	sort.SliceStable(l.shared, func(i, j int) bool {
		a, b := position[l.shared[i].image.Name], position[l.shared[j].image.Name]

		if a == 0 || b == 0 {
			return a != 0 && b == 0
		}

		return a < b
	})
}
