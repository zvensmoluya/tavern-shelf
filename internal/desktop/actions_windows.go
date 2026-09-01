//go:build windows

package desktop

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue   = "Tavern Shelf"
)

type Actions struct {
	ChooseInboxFunc func() (string, error)
}

func (a Actions) OpenInbox(path string) error {
	if err := exec.Command("explorer.exe", path).Start(); err != nil {
		return fmt.Errorf("open Inbox in Explorer: %w", err)
	}
	return nil
}

func (a Actions) ChooseInbox() (string, error) {
	if a.ChooseInboxFunc == nil {
		return "", errors.New("Inbox directory picker is not ready")
	}
	return a.ChooseInboxFunc()
}

func (a Actions) AutoStartEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("open Windows startup settings: %w", err)
	}
	defer key.Close()
	value, _, err := key.GetStringValue(runValue)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("read Windows startup setting: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("locate Tavern Shelf executable: %w", err)
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(executable)), nil
}

func (a Actions) SetAutoStart(enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("open Windows startup settings: %w", err)
	}
	defer key.Close()
	if !enabled {
		if err := key.DeleteValue(runValue); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("disable Tavern Shelf auto-start: %w", err)
		}
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate Tavern Shelf executable: %w", err)
	}
	command := fmt.Sprintf("\"%s\" --background", executable)
	if err := key.SetStringValue(runValue, command); err != nil {
		return fmt.Errorf("enable Tavern Shelf auto-start: %w", err)
	}
	return nil
}
