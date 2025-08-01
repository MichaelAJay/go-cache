package redis

import (
	"os"
	"strconv"

	"github.com/MichaelAJay/go-cache/interfaces"
)

// Default configuration values
const (
	defaultRedisAddr     = "127.0.0.1:6379"
	defaultRedisPassword = ""
	defaultRedisDB       = 0
	defaultRedisPoolSize = 10
)

// Environment variable names
const (
	envRedisAddr     = "REDIS_ADDR"
	envRedisPassword = "REDIS_PASSWORD"
	envRedisDB       = "REDIS_DB"
	envRedisPoolSize = "REDIS_POOL_SIZE"
)

// LoadRedisOptionsFromEnv creates a RedisOptions struct with values from environment variables
// If environment variables are not set, default values are used
func LoadRedisOptionsFromEnv() *interfaces.RedisOptions {
	options := &interfaces.RedisOptions{
		Address:  getEnvString(envRedisAddr, defaultRedisAddr),
		Password: getEnvString(envRedisPassword, defaultRedisPassword),
		DB:       getEnvInt(envRedisDB, defaultRedisDB),
		PoolSize: getEnvInt(envRedisPoolSize, defaultRedisPoolSize),
	}

	return options
}

// getEnvString retrieves a string value from environment or returns the default
func getEnvString(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an integer value from environment or returns the default
func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
