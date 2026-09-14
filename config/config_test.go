package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigDefaults(t *testing.T) {
	os.Setenv("DB_HOST", "testhost")
	defer os.Unsetenv("DB_HOST")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, ":8080", cfg.Server.ServerAddr)
	assert.Equal(t, "testhost", cfg.Database.Host)
	assert.Equal(t, "postgres", cfg.Database.Name)
	assert.Equal(t, 3, cfg.Redis.QueueDB)
	assert.Equal(t, 2, cfg.Worker.Concurrency)
	assert.Equal(t, "", cfg.JWT.AccessPublicKeyURL)
	assert.Equal(t, []string{"http://localhost:5173", "https://example.com", "https://events.example.com"}, cfg.Server.AllowedOrigins)
}

func TestLoadConfigReadsEnv(t *testing.T) {
	t.Setenv("DB_HOST", "pg")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_NAME", "goevents")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("REDIS_QUEUE_DB", "5")
	t.Setenv("WORKER_CONCURRENCY", "8")
	t.Setenv("SERVER_ADDR", ":9090")
	t.Setenv("JWT_ACCESS_PUBLIC_KEY_URL", "http://identity-service:80/auth/public-key")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, "pg", cfg.Database.Host)
	assert.Equal(t, "5433", cfg.Database.Port)
	assert.Equal(t, "goevents", cfg.Database.Name)
	assert.Equal(t, "redis:6379", cfg.Redis.Addr)
	assert.Equal(t, 5, cfg.Redis.QueueDB)
	assert.Equal(t, 8, cfg.Worker.Concurrency)
	assert.Equal(t, ":9090", cfg.Server.ServerAddr)
	assert.Equal(t, "http://identity-service:80/auth/public-key", cfg.JWT.AccessPublicKeyURL)
}

func TestLoadConfigInvalidQueueDB(t *testing.T) {
	t.Setenv("REDIS_QUEUE_DB", "not-a-number")
	_, err := LoadConfig()
	assert.Error(t, err)
}

func TestGetDSN(t *testing.T) {
	cfg := &Config{
		Database: Database{
			Host:     "pg",
			Port:     "5432",
			User:     "u",
			Password: "p",
			Name:     "n",
			SslMode:  "disable",
			TimeZone: "UTC",
		},
	}
	assert.Equal(t, "host=pg port=5432 user=u password=p dbname=n sslmode=disable TimeZone=UTC", cfg.GetDSN())
}