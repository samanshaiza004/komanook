package config

import "testing"

func TestConfigurationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values map[string]string
		valid  bool
	}{
		{"local", map[string]string{"DATABASE_URL": "postgres://localhost/komanook"}, true},
		{"missing database", map[string]string{}, false},
		{"invalid environment", map[string]string{"APP_ENV": "prod", "DATABASE_URL": "postgres://localhost/k"}, false},
		{"origin path", map[string]string{"BASE_URL": "https://read.example/path", "DATABASE_URL": "postgres://localhost/k"}, false},
		{"production TLS", map[string]string{"APP_ENV": "production", "BASE_URL": "https://read.example", "DATABASE_URL": "postgres://db/k?sslmode=verify-full"}, true},
		{"insecure production", map[string]string{"APP_ENV": "production", "BASE_URL": "http://read.example", "DATABASE_URL": "postgres://db/k?sslmode=disable"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(func(key string) string { return tc.values[key] })
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
}
