package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DBURL           string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

func (c *Config) SetUser(username string) error {
	c.CurrentUserName = username
	return Write(*c)
}

const fileName = ".gatorconfig.json"

func getConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, fileName), nil
}

func Read() (Config, error) {
	filePath, err := getConfigFilePath()
	if err != nil {
		return Config{}, err
	}
	data, dataErr := os.ReadFile(filePath)
	if dataErr != nil {
		return Config{}, dataErr
	}
	var cfg Config
	rspErr := json.Unmarshal(data, &cfg)
	if rspErr != nil {
		return Config{}, rspErr
	}
	return cfg, nil
}

func Write(cfg Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	filepath, getErr := getConfigFilePath()
	if getErr != nil {
		return getErr
	}
	return os.WriteFile(filepath, data, 0644)
}
