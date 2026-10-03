package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
)

type watchlistConfig struct {
	Cities  []city   `json:"cities"`
	Sort    sortMode `json:"sort"`
	ShowUTC bool     `json:"show_utc,omitempty"`
}

func loadWatchlistConfig(configPath, legacyPath string, defaults []city) (watchlistConfig, error) {
	config, err := readWatchlist(configPath)
	if err == nil {
		return config, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return watchlistConfig{}, fmt.Errorf("load user city config: %w", err)
	}

	config, err = readWatchlist(legacyPath)
	if err == nil {
		if err := saveWatchlist(configPath, config); err != nil {
			return watchlistConfig{}, fmt.Errorf("migrate legacy city config: %w", err)
		}
		return config, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return watchlistConfig{}, fmt.Errorf("load legacy city config: %w", err)
	}

	return watchlistConfig{Cities: append([]city(nil), defaults...), Sort: sortNameAscending}, nil
}

func loadWatchlist(configPath, legacyPath string, defaults []city) ([]city, error) {
	config, err := loadWatchlistConfig(configPath, legacyPath, defaults)
	return config.Cities, err
}

func readWatchlist(filename string) (watchlistConfig, error) {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return watchlistConfig{}, err
	}
	var legacy []city
	if err := json.Unmarshal(contents, &legacy); err == nil && legacy != nil {
		return watchlistConfig{Cities: legacy, Sort: sortNameAscending}, nil
	}
	var config watchlistConfig
	if err := json.Unmarshal(contents, &config); err != nil {
		return watchlistConfig{}, err
	}
	if config.Cities == nil {
		return watchlistConfig{}, fmt.Errorf("city config %q must contain a JSON array", filename)
	}
	if !config.Sort.valid() {
		config.Sort = sortNameAscending
	}
	return config, nil
}

func readCities(filename string) ([]city, error) {
	config, err := readWatchlist(filename)
	return config.Cities, err
}

func saveWatchlist(filename string, config watchlistConfig) error {
	if !config.Sort.valid() {
		config.Sort = sortNameAscending
	}
	directory := filepath.Dir(filename)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".cities-*.json")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		return err
	}
	return nil
}

func saveCities(filename string, cities []city) error {
	return saveWatchlist(filename, watchlistConfig{Cities: cities, Sort: sortNameAscending})
}

type citySaveResultMsg struct {
	err error
}

func saveCitiesCmd(filename string, cities []city, sortMode sortMode, showUTC bool) tea.Cmd {
	snapshot := append([]city(nil), cities...)
	return func() tea.Msg {
		return citySaveResultMsg{err: saveWatchlist(filename, watchlistConfig{Cities: snapshot, Sort: sortMode, ShowUTC: showUTC})}
	}
}
