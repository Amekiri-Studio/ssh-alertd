// Package event defines the domain model shared between the log monitor
// (producer) and the notifiers (consumers).
package event

import (
	"fmt"
	"time"
)

// LoginEvent describes a successful SSH login detected by the monitor.
type LoginEvent struct {
	// Username is the account that was logged into.
	Username string
	// IP is the remote source address of the connection.
	IP string
	// Port is the client's source (ephemeral) port of the connection, as logged
	// by sshd in "... from <IP> port <Port>". It is NOT the server's listening
	// port (e.g. 22).
	Port string
	// Method is the authentication method, e.g. "password" or "publickey".
	Method string
	// Hostname is the host that received the login (the machine we run on).
	Hostname string
	// Time is when the login was observed.
	Time time.Time
}

// String renders a compact, human-readable one-line summary. Notifiers that
// support rich formatting build their own representation; this is the plain
// fallback and is handy for logging.
func (e LoginEvent) String() string {
	return fmt.Sprintf("SSH login: user=%s ip=%s client_port=%s method=%s host=%s time=%s",
		e.Username, e.IP, e.Port, e.Method, e.Hostname, e.Time.Format(time.RFC3339))
}
