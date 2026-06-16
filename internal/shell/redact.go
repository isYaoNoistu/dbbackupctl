package shell

import (
	"regexp"
	"strings"
)

var envSecretPattern = regexp.MustCompile(`(?i)(MYSQL_PWD|PGPASSWORD|[A-Z0-9_]*PASSWORD|[A-Z0-9_]*SECRET|[A-Z0-9_]*TOKEN|[A-Z0-9_]*KEY)=([^ \t\r\n]+)`)
var passwordArgPattern = regexp.MustCompile(`(?i)(--password=)([^ \t\r\n]+)`)

// RedactString masks common password and token forms in logs.
func RedactString(s string) string {
	out := envSecretPattern.ReplaceAllString(s, `${1}=REDACTED`)
	out = passwordArgPattern.ReplaceAllString(out, `${1}REDACTED`)
	return out
}

// RedactEnv masks sensitive environment variables.
func RedactEnv(env []string) []string {
	redacted := make([]string, len(env))
	for i, item := range env {
		key, _, ok := strings.Cut(item, "=")
		if !ok {
			redacted[i] = item
			continue
		}
		upper := strings.ToUpper(key)
		if strings.Contains(upper, "PASSWORD") ||
			strings.Contains(upper, "SECRET") ||
			strings.Contains(upper, "TOKEN") ||
			strings.HasSuffix(upper, "_KEY") ||
			upper == "MYSQL_PWD" ||
			upper == "PGPASSWORD" {
			redacted[i] = key + "=REDACTED"
			continue
		}
		redacted[i] = item
	}
	return redacted
}

// RedactArgs masks sensitive command-line arguments.
func RedactArgs(args []string) []string {
	redacted := make([]string, len(args))
	copy(redacted, args)
	for i, arg := range redacted {
		if strings.HasPrefix(strings.ToLower(arg), "--password=") {
			redacted[i] = "--password=REDACTED"
			continue
		}
		redacted[i] = RedactString(arg)
	}
	return redacted
}
