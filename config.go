package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DefaultEditor string `json:"default_editor"`
	WorkspacesDir string `json:"workspaces_dir"`
}

func getConfigDir() (string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userConfigDir, "flint"), nil
}

func getWorkspacesDir(configDir string) string {
	return filepath.Join(configDir, "workspaces")
}

func (a *App) loadConfig() error {
	configPath := filepath.Join(a.ConfigDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return a.saveConfig()
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &a.Config)
}

func (a *App) saveConfig() error {
	configPath := filepath.Join(a.ConfigDir, "config.json")
	data, err := json.MarshalIndent(a.Config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
