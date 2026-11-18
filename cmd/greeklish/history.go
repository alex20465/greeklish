package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// entry is one line of the history file. The file is JSON lines so it can
// be replayed later as a validation set or mined for corrections.
type entry struct {
	Time    time.Time `json:"time"`
	Version string    `json:"version"`
	Input   string    `json:"input"`
	Output  string    `json:"output"`
}

// historyPath follows the XDG base directory spec on Unix, where history
// is state, and uses the user config directory elsewhere.
func historyPath() (string, error) {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" && runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "state")
	}
	if dir == "" {
		var err error
		if dir, err = os.UserConfigDir(); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "greeklish", "history.jsonl"), nil
}

type history struct {
	enc *json.Encoder
	f   *os.File
}

func openHistory() (*history, error) {
	path, err := historyPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &history{enc: json.NewEncoder(f), f: f}, nil
}

func (h *history) add(input, output string) error {
	return h.enc.Encode(entry{Time: time.Now().UTC(), Version: version, Input: input, Output: output})
}

func (h *history) Close() error { return h.f.Close() }
