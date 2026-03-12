//go:build ignore

package userpb

import "strings"

// FullName returns the user's full name.
func (m *User) FullName() string {
	return strings.TrimSpace(m.FirstName + " " + m.LastName)
}

// PrimaryEmail returns the user's first email, if any.
func (m *User) PrimaryEmail() (string, bool) {
	if len(m.Emails) == 0 {
		return "", false
	}
	return m.Emails[0], true
}

// MemberCount returns the number of members on the team.
func (t *Team) MemberCount() int {
	return len(t.Members)
}
