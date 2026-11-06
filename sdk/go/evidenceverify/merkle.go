package evidenceverify

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"math/bits"
)

const (
	ctxRecord      = "numinance.evidence.v1.Record"
	ctxAttestation = "numinance.evidence.v1.Attestation"
	ctxManifest    = "numinance.evidence.v1.BundleManifest"
	ctxCheckpoint  = "numinance.evidence.v1.Checkpoint"

	leafHead        = 0x48
	leafAttestation = 0x41
)

func leafHash(leaf []byte) []byte {
	sum := sha256.Sum256(append([]byte{0}, leaf...))
	return sum[:]
}

func nodeHash(l, r []byte) []byte {
	buf := append([]byte{1}, l...)
	sum := sha256.Sum256(append(buf, r...))
	return sum[:]
}

// keyID is the digest of the raw 32-byte public key.
func keyID(pub []byte) string { return sha256Digest(pub) }

// verifySigned checks Ed25519(key, context || digest) where digest is the
// sha256 of the canonical form without the signature.
func verifySigned(pub ed25519.PublicKey, context string, digest [32]byte, sig []byte) bool {
	if len(pub) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(pub, append([]byte(context), digest[:]...), sig)
}

// proof is a decoded inclusion proof.
type proof struct {
	leafIndex, treeSize uint64
	path                [][]byte
}

func proofFromNode(n *node) (*proof, error) {
	p := &proof{leafIndex: n.u("leaf_index"), treeSize: n.u("tree_size")}
	for _, e := range n.list("audit_path") {
		raw, ok := rawDigest(e.str)
		if !ok {
			return nil, errors.New("bad audit path entry")
		}
		p.path = append(p.path, raw)
	}
	if p.leafIndex >= p.treeSize {
		return nil, errors.New("leaf index not below tree size")
	}
	inner := bits.Len64(p.leafIndex ^ (p.treeSize - 1))
	want := inner + bits.OnesCount64(p.leafIndex>>inner)
	if len(p.path) != want {
		return nil, errors.New("audit path length does not fit the index and size")
	}
	return p, nil
}

// decodeProof decodes a proof document of at most 8192 bytes.
func decodeProof(data []byte) (*proof, error) {
	n, err := decodeDoc(data, maxProof, proofFields())
	if err != nil {
		return nil, err
	}
	return proofFromNode(n)
}

// root recomputes the tree root from a leaf (RFC 9162 section 2.1.3.2).
func (p *proof) root(leaf []byte) ([]byte, bool) {
	fn, sn := p.leafIndex, p.treeSize-1
	r := leafHash(leaf)
	for _, h := range p.path {
		if sn == 0 {
			return nil, false
		}
		if fn&1 == 1 || fn == sn {
			r = nodeHash(h, r)
			for fn&1 == 0 && fn != 0 {
				fn >>= 1
				sn >>= 1
			}
		} else {
			r = nodeHash(r, h)
		}
		fn >>= 1
		sn >>= 1
	}
	return r, sn == 0
}

// checkpoint is a decoded stored checkpoint.
type checkpoint struct {
	origin   string
	treeSize uint64
	root     []byte
	digest   [32]byte // sha256 of the canonical form without the signature
	sig      []byte
	keyID    string
}

func decodeCheckpoint(data []byte) (*checkpoint, error) {
	n, err := decodeDoc(data, maxCheckpoint, checkpointFields())
	if err != nil {
		return nil, err
	}
	root, _ := rawDigest(n.s("root_hash"))
	return &checkpoint{
		origin: n.s("origin"), treeSize: n.u("tree_size"), root: root,
		digest: digestBytes(n.canonicalWithout("signature")), sig: decodeBase64(n.s("signature")), keyID: n.s("signing_key_id"),
	}, nil
}

// logPin is the transparency log the caller pinned.
type logPin struct {
	key    ed25519.PublicKey
	origin string
}

// verifyLeaf verifies a leaf under a checkpoint with the pinned log
// (log-and-checkpoints.md section 4, in that order).
func (l *logPin) verifyLeaf(cp *checkpoint, p *proof, leaf []byte) error {
	if cp.keyID != keyID(l.key) || !verifySigned(l.key, ctxCheckpoint, cp.digest, cp.sig) {
		return errors.New("the checkpoint signature does not verify under the pinned log key")
	}
	if cp.origin != l.origin {
		return errors.New("the checkpoint origin is not the pinned origin")
	}
	if len(leaf) != 33 || p.treeSize != cp.treeSize {
		return errors.New("the leaf or the tree size does not fit the checkpoint")
	}
	r, ok := p.root(leaf)
	if !ok || !bytes.Equal(r, cp.root) {
		return errors.New("the proof does not reach the checkpoint root")
	}
	return nil
}

func headLeaf(kind byte, hashRaw []byte) []byte { return append([]byte{kind}, hashRaw...) }
