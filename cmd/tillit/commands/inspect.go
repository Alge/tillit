package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/models"
)

// Inspect prints the full details of a single cached signature, looked
// up by its content-hash ID (full or unique prefix). Accepts a leading
// `#` so the user can paste an ID straight from `query` output.
func Inspect(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: tillit inspect <signature_id>")
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	return runInspect(s, os.Stdout, args[0])
}

func runInspect(s *localstore.Store, w io.Writer, id string) error {
	if id == "" {
		return fmt.Errorf("signature id is empty")
	}

	sig, err := s.LookupCachedSignature(id)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "ID:          %s\n", sig.ID)
	fmt.Fprintf(w, "Signer:      %s\n", sig.Signer)
	fmt.Fprintf(w, "Algorithm:   %s\n", sig.Algorithm)
	fmt.Fprintf(w, "Uploaded at: %s\n", sig.UploadedAt.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(w, "Fetched at:  %s\n", sig.FetchedAt.Format("2006-01-02 15:04:05 MST"))

	// Revocation is derived: a revocation sig with target_id == sig.ID
	// signed by the same signer is the source of truth, not the cache
	// row's mutable revoked column.
	rev, _ := findRevocationFor(s, sig)
	if rev != nil {
		fmt.Fprintf(w, "Revoked:     yes at %s\n",
			rev.UploadedAt.Format("2006-01-02 15:04:05 MST"))
		fmt.Fprintln(w, "Revocation:")
		fmt.Fprintf(w, "  ID:          %s\n", rev.ID)
		fmt.Fprintf(w, "  Uploaded at: %s\n", rev.UploadedAt.Format("2006-01-02 15:04:05 MST"))
		fmt.Fprintln(w, "  Payload:")
		if pretty, err := prettyJSON(rev.Payload); err == nil {
			fmt.Fprintln(w, indent(pretty, "    "))
		} else {
			fmt.Fprintln(w, indent(rev.Payload, "    "))
		}
		fmt.Fprintf(w, "  Sig:         %s\n", rev.Sig)
	} else {
		fmt.Fprintln(w, "Revoked:     no")
	}

	fmt.Fprintln(w, "Payload:")
	if pretty, err := prettyJSON(sig.Payload); err == nil {
		fmt.Fprintln(w, indent(pretty, "  "))
	} else {
		fmt.Fprintln(w, indent(sig.Payload, "  "))
	}

	fmt.Fprintf(w, "Sig:         %s\n", sig.Sig)
	return nil
}

// findRevocationFor looks for the revocation signature that targets the
// given signature. Only the original signer can revoke their own
// signatures (enforced server-side), so we only have to scan that
// signer's cached signatures. Returns (nil, nil) when no revocation is
// cached.
func findRevocationFor(s *localstore.Store, target *localstore.CachedSignature) (*localstore.CachedSignature, error) {
	sigs, err := s.GetCachedSignaturesBySigner(target.Signer)
	if err != nil {
		return nil, err
	}
	for _, candidate := range sigs {
		if candidate.ID == target.ID {
			continue
		}
		var p models.Payload
		if err := json.Unmarshal([]byte(candidate.Payload), &p); err != nil {
			continue
		}
		if p.Type == models.PayloadTypeRevocation && p.TargetID == target.ID {
			return candidate, nil
		}
	}
	return nil, nil
}

func prettyJSON(s string) (string, error) {
	var p models.Payload
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return "", err
	}
	out, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
