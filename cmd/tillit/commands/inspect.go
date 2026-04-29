package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Alge/tillit/models"
)

// Inspect prints the full details of a single cached signature, looked
// up by its content-hash ID (full or unique prefix). Accepts a leading
// `#` so the user can paste an ID straight from `query` output.
func Inspect(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: tillit inspect <signature_id>")
	}
	q := strings.TrimPrefix(args[0], "#")
	if q == "" {
		return fmt.Errorf("signature id is empty")
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	sig, err := s.LookupCachedSignature(q)
	if err != nil {
		return err
	}

	fmt.Printf("ID:          %s\n", sig.ID)
	fmt.Printf("Signer:      %s\n", sig.Signer)
	fmt.Printf("Algorithm:   %s\n", sig.Algorithm)
	fmt.Printf("Uploaded at: %s\n", sig.UploadedAt.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Fetched at:  %s\n", sig.FetchedAt.Format("2006-01-02 15:04:05 MST"))
	if sig.Revoked {
		when := ""
		if sig.RevokedAt != nil {
			when = " at " + sig.RevokedAt.Format("2006-01-02 15:04:05 MST")
		}
		fmt.Printf("Revoked:     yes%s\n", when)
	} else {
		fmt.Println("Revoked:     no")
	}

	fmt.Println("Payload:")
	if pretty, err := prettyJSON(sig.Payload); err == nil {
		fmt.Println(indent(pretty, "  "))
	} else {
		fmt.Println(indent(sig.Payload, "  "))
	}

	fmt.Printf("Sig:         %s\n", sig.Sig)
	return nil
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
