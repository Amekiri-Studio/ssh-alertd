package monitor

import (
	"testing"
	"time"
)

func TestParseAccepted(t *testing.T) {
	m := New(nil, "myhost", nil)
	m.now = func() time.Time { return time.Unix(0, 0).UTC() }

	cases := []struct {
		name                   string
		line                   string
		ok                     bool
		user, ip, port, method string
	}{
		{
			name:   "journald cat publickey",
			line:   "Accepted publickey for alice from 203.0.113.5 port 50568 ssh2: ED25519 SHA256:abc",
			ok:     true,
			user:   "alice",
			ip:     "203.0.113.5",
			port:   "50568",
			method: "publickey",
		},
		{
			name:   "syslog prefixed password",
			line:   "Jun 26 09:49:01 host sshd[123]: Accepted password for bob from 10.0.0.2 port 41122 ssh2",
			ok:     true,
			user:   "bob",
			ip:     "10.0.0.2",
			port:   "41122",
			method: "password",
		},
		{
			name: "failed login is ignored",
			line: "Failed password for bob from 10.0.0.2 port 41122 ssh2",
			ok:   false,
		},
		{
			name: "unrelated line",
			line: "Jun 26 09:49:01 host sshd[123]: Server listening on 0.0.0.0 port 22.",
			ok:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, ok := m.parse(tc.line)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if !tc.ok {
				return
			}
			if e.Username != tc.user || e.IP != tc.ip || e.Port != tc.port || e.Method != tc.method {
				t.Errorf("got user=%q ip=%q port=%q method=%q", e.Username, e.IP, e.Port, e.Method)
			}
			if e.Hostname != "myhost" {
				t.Errorf("hostname = %q, want myhost", e.Hostname)
			}
		})
	}
}
