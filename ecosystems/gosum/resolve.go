package gosum

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// Default endpoints, used when GOPROXY / GOSUMDB don't pick something
// else. We deliberately mirror the Go toolchain's defaults.
const (
	defaultGoProxy = "https://proxy.golang.org"
	defaultGoSumDB = "https://sum.golang.org"
)

// ResolveVersion verifies that (packageID, version) exists by
// querying the Go module proxy's `.info` endpoint, and (when sumdb
// access isn't disabled) fetches the canonical `h1:` zip hash from
// the checksum database. Both calls are small (a few hundred bytes)
// and never download the module zip itself.
//
// GOPROXY and GOSUMDB env vars are honoured the same way the Go
// toolchain does (first http(s) entry of GOPROXY; GOSUMDB="off"
// skips hash lookup).
func (GoSum) ResolveVersion(packageID, version string) (*ecosystems.VersionInfo, error) {
	proxy := pickGoProxy()
	if proxy == "" {
		return nil, fmt.Errorf("no usable GOPROXY (set GOPROXY or use --skip-verify)")
	}

	encModule, err := escapeModulePath(packageID)
	if err != nil {
		return nil, err
	}
	encVersion, err := escapeVersion(version)
	if err != nil {
		return nil, err
	}

	// Existence: GET .info — small JSON, 200 if found, 404 if not.
	infoURL := fmt.Sprintf("%s/%s/@v/%s.info",
		strings.TrimRight(proxy, "/"), encModule, encVersion)
	if err := getInfo(infoURL); err != nil {
		return nil, err
	}

	info := &ecosystems.VersionInfo{
		PackageID: packageID,
		Version:   version,
	}

	// Hash: GET sumdb /lookup — body's first 'h1:' line is the zip
	// hash. Optional; failures here don't fail the existence check.
	if sumdb := pickGoSumDB(); sumdb != "" {
		lookupURL := fmt.Sprintf("%s/lookup/%s@%s",
			strings.TrimRight(sumdb, "/"), packageID, version)
		if hash, err := lookupSumDB(lookupURL); err == nil {
			info.Hash = hash
			info.HashAlgo = "h1"
		}
	}
	return info, nil
}

// getInfo fires the proxy `.info` request and reduces the response
// to a single error if anything went wrong. The body itself isn't
// needed beyond confirming the version exists.
func getInfo(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("contact module proxy: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		// Read enough to satisfy the response body; ignore content.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil
	case http.StatusNotFound, http.StatusGone:
		return fmt.Errorf("module proxy: version not found")
	default:
		return fmt.Errorf("module proxy returned %s", resp.Status)
	}
}

// lookupSumDB fetches the checksum database response for the given
// module@version and extracts the zip hash (the line ending with
// the bare version, not "/go.mod"). The returned string includes
// the "h1:" algorithm prefix.
func lookupSumDB(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sumdb returned %s", resp.Status)
	}
	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 1<<16))
	for scanner.Scan() {
		line := scanner.Text()
		// Lines look like:
		//   <module> <version> h1:<base64=>
		//   <module> <version>/go.mod h1:<base64=>
		// We want the first one — bare version, no /go.mod suffix.
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if strings.HasSuffix(fields[1], "/go.mod") {
			continue
		}
		if !strings.HasPrefix(fields[2], "h1:") {
			continue
		}
		return fields[2], nil
	}
	return "", fmt.Errorf("sumdb response had no zip hash")
}

// pickGoProxy returns the first http(s) entry from GOPROXY (Go
// honours a comma- or pipe-separated list and walks them in order).
// "off" anywhere in the list aborts; "direct" entries are skipped
// because they imply a VCS fetch rather than a proxy. An empty
// GOPROXY falls back to the toolchain default.
func pickGoProxy() string {
	raw := os.Getenv("GOPROXY")
	if raw == "" {
		return defaultGoProxy
	}
	for _, item := range splitProxyList(raw) {
		switch item {
		case "off":
			return ""
		case "direct":
			continue
		}
		if strings.HasPrefix(item, "http://") || strings.HasPrefix(item, "https://") {
			return item
		}
	}
	return defaultGoProxy
}

// pickGoSumDB returns the URL of the checksum database to query, or
// "" when sumdb access is disabled (GOSUMDB="off"). The default is
// sum.golang.org, matching the Go toolchain.
func pickGoSumDB() string {
	raw := os.Getenv("GOSUMDB")
	if raw == "off" {
		return ""
	}
	if raw == "" {
		return defaultGoSumDB
	}
	// Go's GOSUMDB allows "<name> <url>" syntax for custom keys —
	// strip the leading name if present and take whatever URL we
	// can find. If no URL part is given (just a name), use the
	// default endpoint root.
	for _, field := range strings.Fields(raw) {
		if strings.HasPrefix(field, "http://") || strings.HasPrefix(field, "https://") {
			return field
		}
	}
	return defaultGoSumDB
}

func splitProxyList(s string) []string {
	// Go accepts both ',' and '|' as separators; treat them
	// equivalently for our simpler purposes.
	out := []string{}
	for _, sep := range []string{",", "|"} {
		if strings.Contains(s, sep) {
			for _, part := range strings.Split(s, sep) {
				if p := strings.TrimSpace(part); p != "" {
					out = append(out, p)
				}
			}
			return out
		}
	}
	return []string{strings.TrimSpace(s)}
}

// escapeModulePath applies Go's module-path case-encoding rules:
// every ASCII uppercase letter [A-Z] is replaced by '!' followed by
// the lowercase form. Other characters pass through unchanged. This
// is what the Go toolchain does so case-only differences in module
// paths can coexist on case-insensitive filesystems / URLs.
func escapeModulePath(p string) (string, error) {
	var b strings.Builder
	for _, r := range p {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteByte('!')
			b.WriteRune(r + ('a' - 'A'))
		case r == ' ' || r == '?' || r == '#':
			return "", fmt.Errorf("invalid character %q in module path", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String(), nil
}

// escapeVersion uses the same rule for version strings — pseudo-
// versions and pre-release identifiers can contain uppercase letters
// in some build metadata.
func escapeVersion(v string) (string, error) {
	return escapeModulePath(v)
}
