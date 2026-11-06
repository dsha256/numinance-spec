package plugin

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// The environment the host passes to a provider subprocess and the constants of the handshake (the plugin protocol). The wire format is that of HashiCorp go-plugin, implemented here in the standard library:
// the provider prints one line on stdout,
//
//	CORE-PROTOCOL-VERSION|APP-PROTOCOL-VERSION|NETWORK-TYPE|NETWORK-ADDR|PROTOCOL
//
// and the host dials the address. A line with a sixth field (the mutual TLS certificate of go-plugin) is rejected:
// v1 carries no certificate because the socket is a Unix socket in a directory only the host and the provider reach.
const (
	// CookieEnv is the environment variable that carries MagicCookie.
	CookieEnv = "NUMINANCE_PLUGIN_COOKIE"
	// ProtocolsEnv carries the app protocol versions the host serves, comma separated decimal numbers.
	ProtocolsEnv = "NUMINANCE_PLUGIN_PROTOCOLS"
	// HostAddrEnv carries the gRPC target of the HostService the provider calls back, "unix:" plus an absolute path.
	HostAddrEnv = "NUMINANCE_HOST_ADDR"
	// MagicCookie is the value of CookieEnv. Like the go-plugin magic cookie it is not a secret: it stops a user from
	// running a provider binary by hand and being puzzled by a handshake line, nothing more.
	MagicCookie = "numinance.plugin.v1/6d0b2f94-3c1e-4a58-9f7d-1e5b8a2c4d63"
	// CoreProtocolVersion is the go-plugin core protocol version, always 1.
	CoreProtocolVersion = 1
	// AppProtocolVersion is the Numinance plugin protocol major this package speaks.
	AppProtocolVersion = 1
	// NetworkUnix is the only network type of v1.
	NetworkUnix = "unix"
	// ProtocolGRPC is the only protocol of v1.
	ProtocolGRPC = "grpc"
	// MaxLineBytes bounds the handshake line a host reads, newline included.
	MaxLineBytes = 1024
	// maxSocketPath is the shortest sun_path limit of the platforms the host runs on, minus the terminating NUL.
	maxSocketPath = 103
)

// ErrHandshakeLine is wrapped by every rejection of a handshake line. The message names the field, never its value:
// the line comes from a process the host does not trust.
var ErrHandshakeLine = errors.New("handshake line rejected")

// Line is the parsed handshake line.
type Line struct {
	// Core is the go-plugin core protocol version.
	Core uint
	// App is the Numinance plugin protocol major the provider chose from ProtocolsEnv.
	App uint
	// Network is NetworkUnix.
	Network string
	// Address is the absolute path of the provider's socket.
	Address string
	// Protocol is ProtocolGRPC.
	Protocol string
}

// String returns the line without the trailing newline.
func (l Line) String() string {
	return fmt.Sprintf("%d|%d|%s|%s|%s", l.Core, l.App, l.Network, l.Address, l.Protocol)
}

func badLine(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrHandshakeLine, fmt.Sprintf(format, args...))
}

// ParseLine parses and validates one handshake line (without its newline). It accepts exactly five fields, core
// protocol 1, an app protocol of at least 1, network "unix", an absolute path without control characters or "..",
// and protocol "grpc".
func ParseLine(s string) (Line, error) {
	if len(s) >= MaxLineBytes {
		return Line{}, badLine("line is longer than %d bytes", MaxLineBytes)
	}
	f := strings.Split(s, "|")
	if len(f) != 5 {
		return Line{}, badLine("want 5 fields, got %d", len(f))
	}
	core, err := strconv.ParseUint(f[0], 10, 32)
	if err != nil || core != CoreProtocolVersion {
		return Line{}, badLine("core protocol version is not %d", CoreProtocolVersion)
	}
	app, err := strconv.ParseUint(f[1], 10, 32)
	if err != nil || app < 1 {
		return Line{}, badLine("app protocol version is not a positive number")
	}
	if f[2] != NetworkUnix {
		return Line{}, badLine("network is not %s", NetworkUnix)
	}
	if err := checkSocketPath(f[3]); err != nil {
		return Line{}, err
	}
	if f[4] != ProtocolGRPC {
		return Line{}, badLine("protocol is not %s", ProtocolGRPC)
	}
	return Line{Core: uint(core), App: uint(app), Network: f[2], Address: f[3], Protocol: f[4]}, nil
}

func checkSocketPath(p string) error {
	if p == "" || p[0] != '/' || len(p) > maxSocketPath {
		return badLine("address is not an absolute path of at most %d bytes", maxSocketPath)
	}
	for _, r := range p {
		if r == unicode.ReplacementChar || !unicode.IsPrint(r) || r == '|' {
			return badLine("address has a character outside printable text")
		}
	}
	for seg := range strings.SplitSeq(p[1:], "/") {
		if seg == ".." || seg == "." || seg == "" {
			return badLine("address has an empty, . or .. segment")
		}
	}
	return nil
}

// ParseProtocols parses the value of ProtocolsEnv: decimal numbers of at least 1, comma separated, no duplicates, at
// most 16 entries.
func ParseProtocols(s string) ([]uint, error) {
	parts := strings.Split(s, ",")
	if len(parts) > 16 {
		return nil, badLine("more than 16 protocol versions")
	}
	out := make([]uint, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseUint(p, 10, 32)
		if err != nil || v < 1 {
			return nil, badLine("protocol version is not a positive number")
		}
		for _, o := range out {
			if o == uint(v) {
				return nil, badLine("duplicate protocol version")
			}
		}
		out = append(out, uint(v))
	}
	return out, nil
}

// FormatProtocols is the inverse of ParseProtocols.
func FormatProtocols(v []uint) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = strconv.FormatUint(uint64(n), 10)
	}
	return strings.Join(s, ",")
}
