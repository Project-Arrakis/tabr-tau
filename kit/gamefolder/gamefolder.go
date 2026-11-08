// Package gamefolder holds the names and path rules of the folder where the game keeps its files on Windows:
//
//	%LOCALAPPDATA%\DuneSandbox\Saved
//	    Cloud\PlayerClientStorage\<service>\<id>\game.db     the saves (the service of a retail install is FLS_retail)
//	    Config\Windows                                       the settings files
//	    Logs\DuneSandbox.log                                  the log of the current run
//
// Every function is a rule over the strings it is given. None reads the environment, opens a process or a registry key, or looks at a
// file other than the directory listing and the modification times that [Saves] and [NewestSave] need. The caller passes the folder
// that %LOCALAPPDATA% names, so a test can use any folder.
package gamefolder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Names of the folders and files of the game's tree.
const (
	// GameDir is the game's folder under %LOCALAPPDATA%.
	GameDir = "DuneSandbox"
	// SavedName is the name of its Saved folder.
	SavedName = "Saved"
	// RetailService is the folder of the retail install's saves under PlayerClientStorage.
	RetailService = "FLS_retail"
	// SaveFile is the name of a save in the folder of its id.
	SaveFile = "game.db"
	// LogFile is the name of the log of the current run in the Logs folder.
	LogFile = "DuneSandbox.log"
)

// SavedDir is the game's Saved folder under the folder that %LOCALAPPDATA% names. With an empty localAppData the result is relative
// (DuneSandbox\Saved): a caller that must not guess checks for the empty value first.
func SavedDir(localAppData string) string {
	return filepath.Join(localAppData, GameDir, SavedName)
}

// ClientStorageDir is the tree of the saves under the Saved folder: Saved\Cloud\PlayerClientStorage.
func ClientStorageDir(saved string) string {
	return filepath.Join(saved, "Cloud", "PlayerClientStorage")
}

// ConfigDir is the folder of the game's settings files under the Saved folder: Saved\Config\Windows.
func ConfigDir(saved string) string {
	return filepath.Join(saved, "Config", "Windows")
}

// LogsDir is the folder of the game's logs under the Saved folder.
func LogsDir(saved string) string {
	return filepath.Join(saved, "Logs")
}

// LogPath is the log of the current run under the Saved folder: Saved\Logs\DuneSandbox.log.
func LogPath(saved string) string {
	return filepath.Join(LogsDir(saved), LogFile)
}

// LogPathFor is the log that belongs to a save kept in the standard tree (Saved\Cloud\PlayerClientStorage\<service>\<id>\game.db): the
// name of the Saved folder is found four folders above the file, whatever the case it is written in. A save kept anywhere else has no
// known log, and the error says so.
func LogPathFor(savePath string) (string, error) {
	d := filepath.Dir(savePath)
	for i := 0; i < 4; i++ {
		d = filepath.Dir(d)
	}
	if !strings.EqualFold(filepath.Base(d), SavedName) {
		return "", fmt.Errorf("the save is not in the game's standard folder, so its log cannot be found")
	}
	return LogPath(d), nil
}

// Saves lists the game.db files of the tree, in name order: Saved\Cloud\PlayerClientStorage\<service>\<id>\game.db. An empty service
// matches every service folder. Nothing is opened: a name that the listing returns is returned, whether it is a file or not.
func Saves(saved, service string) []string {
	if service == "" {
		service = "*"
	}
	m, _ := filepath.Glob(filepath.Join(ClientStorageDir(saved), service, "*", SaveFile))
	return m
}

// NewestSave is the save of the retail service (FLS_retail) that was written last by its modification time. Of two saves with the same
// time the first in name order is the answer. The error says that none was found, with the Saved folder.
func NewestSave(saved string) (string, error) {
	var newest string
	var nt time.Time
	for _, m := range Saves(saved, RetailService) {
		if st, err := os.Stat(m); err == nil && st.ModTime().After(nt) {
			newest, nt = m, st.ModTime()
		}
	}
	if newest == "" {
		return "", fmt.Errorf("no game.db found under %s", saved)
	}
	return newest, nil
}
