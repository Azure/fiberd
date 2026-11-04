package hyperlight

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/fiberd/pkg/artifact"
)

var ErrHelper = errors.New("hyperlight: helper error")

// facts is what a helper's snapshots depend on. The helper reports it on
// `--version` and on READY as one token each, in this order.
//
//	helper      the helper and its version ("fiberd-hyperlight-helper/0.3.0")
//	hyperlight  the hyperlight_host crate the helper was built with
//	hypervisor  the one in use on this host (kvm, mshv)
//	cpu         the CPU vendor string (GenuineIntel, AuthenticAMD)
//
// Hyperlight refuses to load a snapshot from another version, hypervisor
// or CPU vendor, and the park layout is the helper's own. Recording the
// facts as the platform refuses a mismatch before the resume instead of
// failing inside it.
type facts struct {
	helper, hyperlight, hypervisor, cpu string
}

// parseFacts reads the four tokens after READY or from `--version`.
func parseFacts(tokens []string) (facts, error) {
	if len(tokens) != 4 {
		return facts{}, fmt.Errorf("%w: want 4 facts (helper hyperlight hypervisor cpu), got %q", ErrHelper, strings.Join(tokens, " "))
	}
	for _, t := range tokens {
		if t == "" {
			return facts{}, fmt.Errorf("%w: empty fact in %q", ErrHelper, strings.Join(tokens, " "))
		}
	}
	return facts{helper: tokens[0], hyperlight: tokens[1], hypervisor: tokens[2], cpu: tokens[3]}, nil
}

// platform maps the facts onto the parity fields. The Hyperlight version
// goes where a kernel release goes, so kernel=series relaxes it to a
// minor series. Everything else goes, exact, where libc goes. The host
// keeps the architecture.
func (f facts) platform() artifact.Platform {
	return artifact.Platform{Kernel: "hyperlight-" + f.hyperlight, Libc: f.helper + "+" + f.hypervisor + "+" + f.cpu}
}

// PlatformOf is the platform a Hyperlight home records for snapshots
// made through the helper whose `--version` printed version: its facts,
// mapped as the backend's Platform maps them. A build of a Hyperlight
// snapshot artifact records it (artifact.BuildOptions.HyperlightSnapshot),
// so the parity a home checks is against the helper it runs, not the host
// the artifact was built on.
func PlatformOf(version string) (artifact.Platform, error) {
	f, err := parseFacts(strings.Fields(strings.SplitN(version, "\n", 2)[0]))
	if err != nil {
		return artifact.Platform{}, err
	}
	return f.platform(), nil
}

// HelperFlag reports whether arg, a template argument, names one of the
// flags fiberd itself gives the helper (--guest, --engine, --template-dir)
// in any spelling a flag parser takes: with one dash or more, with or
// without =value, and, after two dashes, abbreviated as getopt_long and
// clap let a long flag be. A template's arguments come from its author,
// and a helper whose last flag wins would take the guest, engine or
// snapshot the author named over fiberd's.
func HelperFlag(arg string) bool {
	if !strings.HasPrefix(arg, "-") {
		return false
	}
	f, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
	for _, name := range []string{"guest", "engine", "template-dir"} {
		if f == name || strings.HasPrefix(arg, "--") && f != "" && strings.HasPrefix(name, f) {
			return true
		}
	}
	return false
}
