package keymanager

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joshjon/kit/log"
)

const (
	configDirName  = "verve"
	configFileName = "config.json"
)

type config struct {
	EncryptionKey string `json:"encryption_key"`
}

// ResolveEncryptionKey determines the encryption key to use.
//
// Resolution logic:
//  1. No env var + no stored key → generate, store, and return
//  2. No env var + stored key    → load and return
//  3. Env var + no stored key    → store env value and return
//  4. Env var matches stored     → return
//  5. Env var mismatches stored  → return error with instructions
func ResolveEncryptionKey(envKey string, logger log.Logger) (string, error) {
	stored, err := loadKey()
	if err != nil {
		return "", fmt.Errorf("load stored encryption key: %w", err)
	}

	switch {
	case envKey == "" && stored == "":
		// Generate a new key.
		key, err := generateKey()
		if err != nil {
			return "", fmt.Errorf("generate encryption key: %w", err)
		}
		if err := storeKey(key); err != nil {
			return "", fmt.Errorf("store encryption key: %w", err)
		}
		logger.Info("generated and stored new encryption key", "config.path", configPath())
		return key, nil

	case envKey == "" && stored != "":
		return stored, nil

	case envKey != "" && stored == "":
		if err := storeKey(envKey); err != nil {
			return "", fmt.Errorf("store encryption key: %w", err)
		}
		logger.Info("stored encryption key from environment", "config.path", configPath())
		return envKey, nil

	case envKey == stored:
		return envKey, nil

	default:
		return "", fmt.Errorf(
			"ENCRYPTION_KEY does not match stored key in %s — to use the new key, remove the config file (warning: existing encrypted data will be irrecoverable)",
			configPath(),
		)
	}
}

func generateKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config", configDirName)
}

func configPath() string {
	return filepath.Join(configDir(), configFileName)
}

func loadKey() (string, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse config %s: %w", configPath(), err)
	}
	return cfg.EncryptionKey, nil
}

func storeKey(key string) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config{EncryptionKey: key}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o600)
}
