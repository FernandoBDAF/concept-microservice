package config

import "testing"

// baseValidConfig returns a Config that satisfies every non-MinIO validation
// rule, so tests can vary only the MinIO credential fields.
func baseValidConfig() *Config {
	return &Config{
		Server:   ServerConfig{HTTPPort: 8080},
		Postgres: PostgresConfig{DSN: "postgres://localhost/db"},
		Redis:    RedisConfig{Host: "localhost", Port: 6379},
		Auth:     AuthConfig{URL: "http://auth"},
	}
}

func TestValidate_MinIOCredentialModes(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  string
		accessKey string
		secretKey string
		wantErr   bool
	}{
		{
			name:      "static creds: both keys set",
			endpoint:  "minio:9000",
			accessKey: "minioadmin",
			secretKey: "minioadmin",
			wantErr:   false,
		},
		{
			name:      "ambient/IRSA: both keys empty",
			endpoint:  "s3.us-east-1.amazonaws.com",
			accessKey: "",
			secretKey: "",
			wantErr:   false,
		},
		{
			name:      "partial config: access set, secret empty",
			endpoint:  "minio:9000",
			accessKey: "minioadmin",
			secretKey: "",
			wantErr:   true,
		},
		{
			name:      "partial config: secret set, access empty",
			endpoint:  "minio:9000",
			accessKey: "",
			secretKey: "minioadmin",
			wantErr:   true,
		},
		{
			name:      "no endpoint: keys irrelevant",
			endpoint:  "",
			accessKey: "",
			secretKey: "",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseValidConfig()
			cfg.MinIO = MinIOConfig{
				Endpoint:        tt.endpoint,
				AccessKeyID:     tt.accessKey,
				SecretAccessKey: tt.secretKey,
			}
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}
