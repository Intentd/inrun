package web

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type RuntimeStorage struct {
	URLs []string `json:"urls"`
}

func getStoragePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, ".inrun")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "instances.json"), nil
}

func LoadRuntimeStorage() (*RuntimeStorage, error) {
	path, err := getStoragePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &RuntimeStorage{URLs: []string{}}, nil
		}
		return nil, err
	}
	var store RuntimeStorage
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

func SaveRuntimeStorage(store *RuntimeStorage) error {
	path, err := getStoragePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
