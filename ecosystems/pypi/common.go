package pypi

// pypiCommon carries the methods shared by every PyPI-hosted lockfile
// adapter in this package: identity, version comparison, version
// validation, and PyPI-side existence/hash resolution. Per-format
// adapters embed it so they only need to implement Name, CanParse,
// and Parse.
type pypiCommon struct{}

func (pypiCommon) Ecosystem() string { return "pypi" }

func (pypiCommon) CompareVersions(a, b string) int { return comparePEP440(a, b) }
