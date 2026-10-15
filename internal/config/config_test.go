package config

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDBConfigPoolSize(t *testing.T) {
	defaultMax := int32(max(4, runtime.NumCPU()))

	tests := []struct {
		name    string
		env     map[string]string
		wantMax int32
		wantMin int32
		wantErr []string // every substring must be in the error; empty means no error
	}{
		{name: "defaults match pgxpool", wantMax: defaultMax, wantMin: 0},
		{name: "both set", env: map[string]string{"DB_MAX_CONNS": "10", "DB_MIN_CONNS": "2"}, wantMax: 10, wantMin: 2},
		{name: "min equal to max", env: map[string]string{"DB_MAX_CONNS": "3", "DB_MIN_CONNS": "3"}, wantMax: 3, wantMin: 3},
		{name: "max zero", env: map[string]string{"DB_MAX_CONNS": "0"}, wantErr: []string{"DB_MAX_CONNS=0 must be greater than 0"}},
		{name: "min greater than max", env: map[string]string{"DB_MAX_CONNS": "2", "DB_MIN_CONNS": "3"}, wantErr: []string{"DB_MIN_CONNS=3 must be <= DB_MAX_CONNS=2"}},
		{name: "negative min", env: map[string]string{"DB_MIN_CONNS": "-1"}, wantErr: []string{"DB_MIN_CONNS must not be negative"}},
		{name: "not a number", env: map[string]string{"DB_MAX_CONNS": "abc"}, wantErr: []string{"DB_MAX_CONNS has wrong format"}},
		{name: "out of int32 range", env: map[string]string{"DB_MAX_CONNS": "3000000000"}, wantErr: []string{"DB_MAX_CONNS has wrong format"}},
		{name: "both wrong reported together", env: map[string]string{"DB_MAX_CONNS": "abc", "DB_MIN_CONNS": "-1"}, wantErr: []string{"DB_MAX_CONNS", "DB_MIN_CONNS"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetEnv(t, "DB_MAX_CONNS", "DB_MIN_CONNS")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			got, err := LoadDBConfig()

			if len(tt.wantErr) > 0 {
				for _, want := range tt.wantErr {
					assert.ErrorContains(t, err, want)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantMax, got.MaxConns)
			assert.Equal(t, tt.wantMin, got.MinConns)
		})
	}
}

func TestLoadReportsAllErrors(t *testing.T) {
	unsetEnv(t, "JWT_SECRET", "HEALTH_TIMEOUT", "LOGREG_TIMEOUT", "REQUEST_TIMEOUT")
	t.Setenv("DB_MAX_CONNS", "0")

	_, err := Load()

	for _, want := range []string{"DB_MAX_CONNS", "JWT_SECRET", "HEALTH_TIMEOUT", "LOGREG_TIMEOUT", "REQUEST_TIMEOUT"} {
		assert.ErrorContains(t, err, want)
	}
}

// unsetEnv removes keys for one test, so values from the developer's shell or CI can't leak in.
// t.Setenv first registers the restore of the original value.
func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		t.Setenv(k, "")
		require.NoError(t, os.Unsetenv(k))
	}
}
