package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL    string
	SleeperUserID  string
	SleeperSport   string
	SleeperBaseURL string
	StartSeason    int
	EndSeason      int
	HTTPTimeout    time.Duration
	DBMaxOpenConns int
	DBMaxIdleConns int
	DBMaxIdleTime  time.Duration
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	cfg := Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		SleeperUserID:  os.Getenv("SLEEPER_USER_ID"),
		SleeperSport:   getEnv("SLEEPER_SPORT", "nfl"),
		SleeperBaseURL: getEnv("SLEEPER_API_BASE_URL", "https://api.sleeper.app/v1"),
		StartSeason:    getEnvInt("START_SEASON", 2022),
		EndSeason:      getEnvInt("END_SEASON", 2025),
		HTTPTimeout:    getEnvDuration("HTTP_TIMEOUT", 30*time.Second),
		DBMaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 25),
		DBMaxIdleTime:  getEnvDuration("DB_MAX_IDLE_TIME", 75*time.Minute),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL must be set")
	}
	if cfg.SleeperUserID == "" {
		return Config{}, fmt.Errorf("SLEEPER_USER_ID must be set")
	}
	if cfg.StartSeason > cfg.EndSeason {
		return Config{}, fmt.Errorf("START_SEASON must be less than or equal to END_SEASON")
	}

	return cfg, nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
