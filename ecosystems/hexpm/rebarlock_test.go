package hexpm_test

import (
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/hexpm"
	"github.com/Alge/tillit/ecosystems/internal/testutil"
)

type rebarPackage struct {
	ID, Version, Hash string
}

func parseRebar(t *testing.T, content string) (pkgs []rebarPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{
		"rebar.lock": &fstest.MapFile{Data: []byte(content)},
	}
	res, err := (hexpm.RebarLock{}).Parse(fsys, "rebar.lock")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, p := range res.Packages {
		if p.Ecosystem != "hexpm" {
			t.Errorf("Ecosystem = %q, want hexpm", p.Ecosystem)
		}
		pkgs = append(pkgs, rebarPackage{p.PackageID, p.Version, p.Hash})
	}
	return pkgs, res.Warnings
}

func TestRebarLock_Identity(t *testing.T) {
	a := hexpm.RebarLock{}
	if a.Ecosystem() != "hexpm" {
		t.Errorf("Ecosystem() = %q, want hexpm", a.Ecosystem())
	}
	if a.Name() != "rebar.lock" {
		t.Errorf("Name() = %q, want rebar.lock", a.Name())
	}
	cases := []struct {
		path string
		want bool
	}{
		{"rebar.lock", true},
		{"./rebar.lock", true},
		{"/some/path/rebar.lock", true},
		{"rebar.config", false}, // project file, not lockfile
		{"mix.lock", false},     // sibling adapter's territory
		{"rebar.lock.bak", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := a.CanParse(tc.path); got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestRebarLock_Parse_BasicHexPackages(t *testing.T) {
	pkgs, warnings := parseRebar(t, `{"1.2.0",[{<<"cowboy">>,{pkg,<<"cowboy">>,<<"2.10.0">>},0},
{<<"cowlib">>,{pkg,<<"cowlib">>,<<"2.12.0">>},1}]}.
[
{pkg_hash,[
  {<<"cowboy">>, <<"3DAFBBC0BFAC23BD9DAF3D7C9D1F3F4FC2BE38311A14B17BF3DF6CFFD9CC1B3D">>},
  {<<"cowlib">>, <<"AABBCCDDEEFF0011223344556677889900112233445566778899AABBCCDDEEFF">>}]},
{pkg_hash_ext,[
  {<<"cowboy">>, <<"OUTER_HASH_COWBOY">>},
  {<<"cowlib">>, <<"OUTER_HASH_COWLIB">>}]}
].
`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
	got := map[string]rebarPackage{}
	for _, p := range pkgs {
		got[p.ID] = p
	}
	if got["cowboy"].Version != "2.10.0" {
		t.Errorf("cowboy version: %+v", got["cowboy"])
	}
	if got["cowboy"].Hash != "3DAFBBC0BFAC23BD9DAF3D7C9D1F3F4FC2BE38311A14B17BF3DF6CFFD9CC1B3D" {
		t.Errorf("cowboy hash: %+v", got["cowboy"])
	}
	if got["cowlib"].Version != "2.12.0" {
		t.Errorf("cowlib version: %+v", got["cowlib"])
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestRebarLock_Parse_MultiLineEntries(t *testing.T) {
	// Real rebar.lock files often wrap entries across lines.
	pkgs, _ := parseRebar(t, `{"1.2.0",
 [{<<"cowboy">>,
   {pkg,<<"cowboy">>,<<"2.10.0">>},
   0}]}.
[].
`)
	if len(pkgs) != 1 || pkgs[0].ID != "cowboy" || pkgs[0].Version != "2.10.0" {
		t.Errorf("expected cowboy 2.10.0, got: %+v", pkgs)
	}
}

func TestRebarLock_Parse_AliasName(t *testing.T) {
	// rebar3 lets a project depend on a hex package under a different
	// local atom. The OUTER name is the project alias; the INNER
	// {pkg, <<"actual_name">>, <<"version">>} carries the registry name.
	// We key signing on the registry name (inner), not the alias.
	pkgs, _ := parseRebar(t, `{"1.2.0",[{<<"my_alias">>,{pkg,<<"actual_pkg">>,<<"1.0.0">>},0}]}.
[{pkg_hash,[{<<"my_alias">>, <<"abc">>}]}].
`)
	if len(pkgs) != 1 || pkgs[0].ID != "actual_pkg" {
		t.Errorf("expected registry name as id, got: %+v", pkgs)
	}
}

func TestRebarLock_Parse_SkipsGitSource(t *testing.T) {
	pkgs, warnings := parseRebar(t, `{"1.2.0",[{<<"ok">>,{pkg,<<"ok">>,<<"1.0.0">>},0},
{<<"from_git">>,{git,"https://github.com/foo/bar.git",{ref,"abc123"}},0}]}.
[].
`)
	if len(pkgs) != 1 || pkgs[0].ID != "ok" {
		t.Errorf("expected only hex pkg, got: %+v", pkgs)
	}
	if !testutil.WarningContains(warnings, "from_git") {
		t.Errorf("expected git warning, got: %v", warnings)
	}
}

func TestRebarLock_Parse_SkipsRawSource(t *testing.T) {
	// rebar3 also has `{raw, ...}` for arbitrary non-hex deps.
	_, warnings := parseRebar(t, `{"1.2.0",[{<<"raw_dep">>,{raw,"path/to/something"},0}]}.
[].
`)
	if !testutil.WarningContains(warnings, "raw_dep") {
		t.Errorf("expected warning, got: %v", warnings)
	}
}

func TestRebarLock_Parse_HashOptional(t *testing.T) {
	// Older rebar.lock files don't have the pkg_hash second term.
	pkgs, _ := parseRebar(t, `{"1.1.0",[{<<"cowboy">>,{pkg,<<"cowboy">>,<<"2.10.0">>},0}]}.
`)
	if len(pkgs) != 1 || pkgs[0].Hash != "" {
		t.Errorf("expected pkg with empty hash, got: %+v", pkgs)
	}
}

func TestRebarLock_Parse_DedupsRepeatEntries(t *testing.T) {
	pkgs, _ := parseRebar(t, `{"1.2.0",[{<<"cowboy">>,{pkg,<<"cowboy">>,<<"2.10.0">>},0},
{<<"cowboy">>,{pkg,<<"cowboy">>,<<"2.10.0">>},0}]}.
[].
`)
	if len(pkgs) != 1 {
		t.Errorf("expected dedup, got: %+v", pkgs)
	}
}

func TestRebarLock_Parse_KeepsDifferentVersions(t *testing.T) {
	pkgs, _ := parseRebar(t, `{"1.2.0",[{<<"cowboy_a">>,{pkg,<<"cowboy">>,<<"2.10.0">>},0},
{<<"cowboy_b">>,{pkg,<<"cowboy">>,<<"2.11.0">>},0}]}.
[].
`)
	if len(pkgs) != 2 {
		t.Errorf("expected 2 versions, got: %+v", pkgs)
	}
}

func TestRebarLock_Parse_MissingFileErrors(t *testing.T) {
	_, err := (hexpm.RebarLock{}).Parse(fstest.MapFS{}, "rebar.lock")
	if err == nil {
		t.Error("expected error when file missing")
	}
}

func TestRebarLock_Parse_EmptyLockfile(t *testing.T) {
	pkgs, _ := parseRebar(t, `{"1.2.0",[]}.
[].
`)
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
}

func TestRebarLock_Parse_TotallyEmpty(t *testing.T) {
	// Defensive — completely empty input shouldn't crash.
	pkgs, _ := parseRebar(t, ``)
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
}
