package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"
)

// The grammar of ticketing.proto ("Formats"), applied before anything is sent: a request that breaks it ends INVALID_ARGUMENT
// with a fixed message and never reaches the tracker, so a malformed value cannot be told apart from another by timing.
var (
	segmentRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)
	issueKeyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	// idRe is the form of every identifier read back from the tracker and reused (a comment id, a transition id): one opaque
	// token, so it can be a path segment, an evidence value and a property value.
	idRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

const (
	maxSearchText = 256
	maxCommentLen = 8192
	maxBodyBound  = 65536
	maxResults    = 50
)

// validProject reports whether p is in the project grammar: one or two segments separated by one '/', no '..' anywhere.
func validProject(p string) bool {
	if len(p) > 201 || strings.Contains(p, "..") {
		return false
	}
	segs := strings.Split(p, "/")
	if len(segs) > 2 {
		return false
	}
	for _, s := range segs {
		if !segmentRe.MatchString(s) {
			return false
		}
	}
	return true
}

func validIssueKey(k string) bool { return issueKeyRe.MatchString(k) }

// isPrintable is the port's printable: Unicode categories L, M, N, P, S and U+0020.
func isPrintable(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if !printableRune(r) {
			return false
		}
	}
	return true
}

func printableRune(r rune) bool {
	return r == ' ' || unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
}

// validSearchText: at most 256 printable bytes (empty matches every issue).
func validSearchText(t string) bool { return len(t) <= maxSearchText && isPrintable(t) }

// validCommentBody: 1 to 8192 bytes of valid UTF-8 text. A line feed and a tab are layout; every other control, bidirectional
// and zero-width character is refused, because a comment is read by people and such characters can disguise what it says.
func validCommentBody(b string) bool {
	if len(b) < 1 || len(b) > maxCommentLen || !utf8.ValidString(b) {
		return false
	}
	for _, r := range b {
		if r != '\n' && r != '\t' && !printableRune(r) {
			return false
		}
	}
	return true
}

func knownState(s ticketingv1.IssueStateCategory) bool {
	return s >= ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN && s <= ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_CANCELED
}

// keyDigest names the operation of a host idempotency key on the tracker: the first 16 bytes of its SHA-256, in hex. The host
// key itself is never written to the tracker.
func keyDigest(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:16])
}

// CommentProperty is the name of the issue property that records the comment a host idempotency key made. It is exported so
// the replay cassette and its tests name the same property.
func CommentProperty(idempotencyKey string) string {
	return commentPropertyPrefix + keyDigest(idempotencyKey)
}

const commentPropertyPrefix = "numinance.comment."

// operationIDsProperty holds the operation ids recorded on an issue by earlier writes. Anyone who can edit the issue can set it.
const operationIDsProperty = "numinance.operation_ids"
