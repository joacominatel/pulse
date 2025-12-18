package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application.
// loaded from environment variables, no magic defaults for required fields.
type Config struct {
	Database DatabaseConfig
	Auth     AuthConfig
}

// DatabaseConfig contains database connection parameters.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
	Schema   string
}

// AuthConfig contains authentication configuration.
type AuthConfig struct {
	// JWTSecret is the supabase jwt secret for token validation
	JWTSecret string
}

// ConnectionString returns the postgres connection string.
func (c DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&search_path=%s",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.Name,
		c.SSLMode,
		c.Schema,
	)
}

// Load reads configuration from environment variables.
// loads .env file if present, but doesn't fail if it's missing.
func Load() (*Config, error) {
	// try to load .env file, ignore error if it doesn't exist
	_ = godotenv.Load()

	dbConfig, err := loadDatabaseConfig()
	if err != nil {
		return nil, fmt.Errorf("database config: %w", err)
	}

	authConfig, err := loadAuthConfig()
	if err != nil {
		return nil, fmt.Errorf("auth config: %w", err)
	}

	return &Config{
		Database: dbConfig,
		Auth:     authConfig,
	}, nil
}

func loadAuthConfig() (AuthConfig, error) {
	config := AuthConfig{
		JWTSecret: os.Getenv("SUPABASE_JWT_SECRET"),
	}

	if config.JWTSecret == "" {
		return config, errors.New("SUPABASE_JWT_SECRET is required")
	}

	return config, nil
}

func loadDatabaseConfig() (DatabaseConfig, error) {
	config := DatabaseConfig{
		Host:     getEnvOrDefault("DB_HOST", "localhost"),
		Port:     getEnvOrDefault("DB_PORT", "5432"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Name:     os.Getenv("DB_NAME"),
		SSLMode:  getEnvOrDefault("DB_SSL_MODE", "require"),
		Schema:   getEnvOrDefault("DB_SCHEMA", "pulse"),
	}

	// required fields must be set
	if config.User == "" {
		return config, errors.New("DB_USER is required")
	}
	if config.Password == "" {
		return config, errors.New("DB_PASSWORD is required")
	}
	if config.Name == "" {
		return config, errors.New("DB_NAME is required")
	}

	return config, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
