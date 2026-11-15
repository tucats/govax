package corevms

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/security"
)

// The objects other than files whose profiles $GET_SECURITY and
// $SET_SECURITY reach (docs/planning/PHASE-93 - file protection.md,
// subtask 13b): devices, volumes, logical name tables, and security
// classes. Only a device's profile may be changed; the others are read.

// deviceProfile is a device's owner, protection, and ACL.
type deviceProfile struct {
	owner      security.UIC
	protection security.Protection
	acl        []byte
}

// defaultDeviceProfile is a device's profile until $SET_SECURITY changes
// it: VMS 7.1's disks', [1,4] and (S:RWPL,O:RWPL,G:R,W), with no ACL
// (cases 411 to 416; other devices' unconfirmed).
var defaultDeviceProfile = deviceProfile{owner: security.SystemUIC, protection: 0xFE00}

// deviceObject is a device as a security object.
type deviceObject struct {
	env  *Environment
	name string // the device's name, DUA1
}

// profile is the device's profile, the default if it has none of its own.
func (d deviceObject) profile() *deviceProfile {
	if p := d.env.deviceProfiles[d.name]; p != nil {
		return p
	}

	p := defaultDeviceProfile

	return &p
}

// DeviceProfile is the profile of the device called name (DUA1), for
// $CHECK_ACCESS.
func (env *Environment) DeviceProfile(name string) security.Profile {
	return deviceObject{env: env, name: strings.TrimSuffix(strings.TrimPrefix(name, "_"), ":")}.Profile()
}

func (d deviceObject) Profile() security.Profile {
	p := d.profile()
	acl, _ := security.DecodeACL(p.acl)

	return security.Profile{Owner: p.owner, Protection: p.protection, ACL: acl, Name: d.name}
}

func (d deviceObject) ACL() []byte { return d.profile().acl }

func (d deviceObject) MayRead() bool {
	return mayAny(d.env.Credentials(), d.Profile(), security.Read, security.Control)
}

func (d deviceObject) MayChange() bool {
	return mayAny(d.env.Credentials(), d.Profile(), security.Control)
}

// set keeps the device's changed profile.
func (d deviceObject) set(change func(*deviceProfile)) {
	p := *d.profile()
	change(&p)

	if d.env.deviceProfiles == nil {
		d.env.deviceProfiles = map[string]*deviceProfile{}
	}

	d.env.deviceProfiles[d.name] = &p
}

func (d deviceObject) SetACL(acl []byte) error {
	d.set(func(p *deviceProfile) { p.acl = append([]byte(nil), acl...) })

	return nil
}

func (d deviceObject) SetOwnerProtection(owner security.UIC, protection security.Protection) error {
	if owner != d.profile().owner && !security.CanChangeOwner(d.env.Credentials(), d.Profile(), owner) {
		return errNoChange
	}

	d.set(func(p *deviceProfile) { p.owner, p.protection = owner, protection })

	return nil
}

// errNoChange is a change the process may not make.
var errNoChange = errors.New("corevms: no privilege for the change")

// mayAny reports whether c has any of wants to an object with profile p.
func mayAny(c security.Credentials, p security.Profile, wants ...security.Access) bool {
	for _, w := range wants {
		if security.Check(c, p, w).Granted {
			return true
		}
	}

	return false
}

// findDeviceObject is the device name names: its device part, through
// its logical names (PROT93$DISK:[SEC93.GET]G1.DAT is DUA1's; so is
// PROT93$DISK with no colon, as VMS 7.1 took it). Its OBJECT_NAME is the
// device's full name, _node$DUA1:.
func findDeviceObject(env *Environment, name string) (securityObject, string, uint32) {
	device := strings.TrimSpace(name)
	if i := strings.IndexByte(device, ':'); i >= 0 {
		device = device[:i]
	}

	physical, st := env.deviceName(device + ":")
	if st != 0 {
		return nil, "", st
	}

	d, found := env.Devices.Find(strings.TrimSuffix(strings.TrimPrefix(physical, "_"), ":"))
	if !found {
		return nil, "", ssNoSuchDev
	}

	return deviceObject{env: env, name: d.Name}, nodeDeviceName(env, d), ssNormal
}

// readOnlyObject is an object whose profile is read and never changed
// here: a volume (whose protection govax shows and doesn't enforce, the
// review's answer 4), a logical name table, or a security class.
type readOnlyObject struct{ profile security.Profile }

func (o readOnlyObject) Profile() security.Profile { return o.profile }
func (o readOnlyObject) ACL() []byte               { return nil }
func (o readOnlyObject) MayRead() bool             { return true }
func (o readOnlyObject) MayChange() bool           { return false }
func (o readOnlyObject) SetACL([]byte) error       { return errNoChange }

func (o readOnlyObject) SetOwnerProtection(security.UIC, security.Protection) error {
	return errNoChange
}

// findVolumeObject is the volume on name's device: its label (its
// OBJECT_NAME), owner, and protection.
func findVolumeObject(env *Environment, name string) (securityObject, string, uint32) {
	if env.Session == nil {
		return nil, "", ssNoSuchObj
	}

	device := strings.TrimSpace(name)
	if i := strings.IndexByte(device, ':'); i >= 0 {
		device = device[:i]
	}

	label, owner, protection, err := env.Session.VolumeProfile(device + ":")
	if err != nil {
		return nil, "", ssNoSuchDev
	}

	return readOnlyObject{security.Profile{Owner: owner, Protection: protection}}, label, ssNormal
}

// sharedTableProfile is a shareable logical name table's profile: VMS
// 7.1's LNM$SYSTEM_TABLE, [1,4] and (S:RWC,O:RWC,G:R,W:R) (case 417); a
// process table's is the process's, with every access (unconfirmed).
const sharedTableProtection = security.Protection(0xEE88)

// findTableObject is the logical name table called name.
func findTableObject(env *Environment, name string) (securityObject, string, uint32) {
	if env.Logicals == nil {
		return nil, "", ssNoSuchObj
	}

	name = strings.ToUpper(strings.TrimSpace(name))

	for _, t := range env.Logicals.Tables() {
		if !strings.EqualFold(t.Name, name) {
			continue
		}

		p := security.Profile{Owner: security.UIC(env.Process.UIC)}
		if t.Shareable {
			p = security.Profile{Owner: security.SystemUIC, Protection: sharedTableProtection}
		}

		return readOnlyObject{p}, t.Name, ssNormal
	}

	return nil, "", ssNoSuchObj
}

// findClassObject is the security class called name, its OBJECT_NAME its
// whole name; a name that isn't a class's is SS$_NOCLASS (VMS 7.1's for a
// file's name). Its profile is [1,4]'s with every access (unconfirmed).
func findClassObject(_ *Environment, name string) (securityObject, string, uint32) {
	c := classNamed(name)
	if c == nil {
		return nil, "", ssNoClass
	}

	return readOnlyObject{security.Profile{Owner: security.SystemUIC}}, c.name, ssNormal
}
