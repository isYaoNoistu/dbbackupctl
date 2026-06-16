package configenv

// SecretConfig holds secret configuration (passwords, keys)
type SecretConfig struct {
	// Passwords map: key is env var name, value is password
	Passwords map[string]string
}

// GetPassword gets a password by environment variable name
func (s *SecretConfig) GetPassword(envVarName string) (string, bool) {
	if s.Passwords == nil {
		return "", false
	}
	val, ok := s.Passwords[envVarName]
	return val, ok
}