// Package hexpm parses Elixir/Erlang lockfile formats whose packages
// live on hex.pm into the canonical PackageRef shape consumed by the
// trust graph resolver. Each lockfile format gets its own adapter
// type inside this package; they all share Ecosystem() == "hexpm" so
// a vetting of (hexpm, phoenix, 1.7.10) is portable regardless of
// whether the project was driven by Mix or rebar3.
package hexpm

import "github.com/Alge/tillit/ecosystems/internal/semver"

// hexpmCommon carries the methods shared by every hex.pm lockfile
// adapter in this package: identity, version comparison, version
// validation, and registry-side existence/hash resolution. Per-format
// adapters embed it so they only need to implement Name, CanParse,
// and Parse.
type hexpmCommon struct{}

func (hexpmCommon) Ecosystem() string { return "hexpm" }

func (hexpmCommon) CompareVersions(a, b string) int { return semver.Compare(a, b) }

func (hexpmCommon) ValidateVersion(v string) error { return semver.Validate(v) }
