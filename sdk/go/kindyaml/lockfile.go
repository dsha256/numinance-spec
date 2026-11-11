package kindyaml

import (
	"encoding/base64"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// MaxExtensionIDBytes bounds a LockedExtension id (the plugin_id bound of plugin/v1 Manifest).
const MaxExtensionIDBytes = 253

// MaxSignerBytes bounds a LockedExtension signer (the string bound of plugin/v1 Manifest).
const MaxSignerBytes = 1024

// isExtensionID reports whether s is a plugin_id: at least two dot-joined labels, each matching
// ^[a-z]([a-z0-9-]*[a-z0-9])?$, at most MaxExtensionIDBytes bytes.
func isExtensionID(s string) bool {
	if s == "" || len(s) > MaxExtensionIDBytes {
		return false
	}
	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return false
	}
	for _, l := range labels {
		if l == "" || l[0] < 'a' || l[0] > 'z' || l[len(l)-1] == '-' {
			return false
		}
		for i := range len(l) {
			c := l[i]
			if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' {
				continue
			}
			return false
		}
	}
	return true
}

// isH1 reports whether s is "h1:" plus the standard base64 of 32 bytes (44 characters).
func isH1(s string) bool {
	b, ok := strings.CutPrefix(s, "h1:")
	if !ok || len(b) != 44 {
		return false
	}
	raw, err := base64.StdEncoding.DecodeString(b)
	return err == nil && len(raw) == 32 && base64.StdEncoding.EncodeToString(raw) == b
}

// isZH reports whether s is "zh:" plus 64 lowercase hex characters.
func isZH(s string) bool {
	h, ok := strings.CutPrefix(s, "zh:")
	return ok && isDigest("sha256:"+h)
}

// isSigner reports whether s is one printable line of at most MaxSignerBytes bytes with no whitespace: the OIDC issuer
// and subject pair of a signer identity carries none.
func isSigner(s string) bool {
	return isText(s, MaxSignerBytes) && !strings.ContainsAny(s, " \t")
}

// pairKey keys a list element by two or more string fields, so "a"+"bc" and "ab"+"c" differ.
func pairKey(names ...protoreflect.Name) func(protoreflect.Value) string {
	return func(v protoreflect.Value) string {
		m := v.Message()
		fs := m.Descriptor().Fields()
		parts := make([]string, len(names))
		for i, n := range names {
			parts[i] = m.Get(fs.ByName(n)).String()
		}
		return strings.Join(parts, "\x00")
	}
}
