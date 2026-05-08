package setup

import "regexp"

var usernameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

func validUsername(s string) bool {
	return usernameRE.MatchString(s)
}
