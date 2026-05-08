package systemd

import "regexp"

// unitNameRE matches valid systemd unit identifiers we accept from URLs.
//
// The character class covers everything systemd itself allows in unit names
// (letters, digits, "@" for templates, ":", "_", ".", "-"); the trailing
// extension whitelist limits us to the unit types ControlRoom manages.
var unitNameRE = regexp.MustCompile(
	`^[a-zA-Z0-9@_.\-:\\]+\.(service|socket|target|timer|path|mount|slice|scope)$`,
)

func ValidUnitName(name string) bool {
	if name == "" || len(name) > 256 {
		return false
	}
	return unitNameRE.MatchString(name)
}
