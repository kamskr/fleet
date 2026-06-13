package config

import (
	"errors"
	"os"
	"path/filepath"
)

const DefaultSocket = "fleet"

type Config struct {
	StatePath string
	Socket    string
	Debug     bool
}

func Load() (Config, error) {
	statePath, err := StatePath()
	if err != nil {
		return Config{}, err
	}
	socket := os.Getenv("FLEET_TMUX_SOCKET")
	if socket == "" {
		socket = DefaultSocket
	}
	if socket == "" {
		return Config{}, errors.New("fleet tmux socket cannot be empty")
	}
	return Config{StatePath: statePath, Socket: socket, Debug: os.Getenv("FLEET_DEBUG") != ""}, nil
}

func StatePath() (string, error) {
	if p := os.Getenv("FLEET_STATE_PATH"); p != "" {
		return filepath.Clean(p), nil
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "fleet", "state.json"), nil
}
