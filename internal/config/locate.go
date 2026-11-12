package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// PathSource says why a configuration file path applies.
type PathSource string

// The sources of the path, in the sequence of their priority.
const (
	// SourceFlag: the flag --config of the command.
	SourceFlag PathSource = "flag"
	// SourceEnv: the environment variable A38_CONFIG.
	SourceEnv PathSource = "env"
	// SourceXDG: the default location below $XDG_CONFIG_HOME.
	SourceXDG PathSource = "xdg"
	// SourceHome: the default location ~/.config/a38/config.yaml.
	SourceHome PathSource = "default"
)

// Location is the configuration file that applies, and the reason.
type Location struct {
	// Path is the absolute path of the file. The file may not exist.
	Path string
	// Source is the reason why Path applies.
	Source PathSource
	// Why is the reason in one sentence.
	Why string
}

// Locate returns the configuration file that applies: the flag --config
// (flagPath, empty if the flag is not set), else A38_CONFIG, else
// $XDG_CONFIG_HOME/a38/config.yaml, else ~/.config/a38/config.yaml. It
// reads no file.
func Locate(flagPath string) (Location, error) {
	env := os.Getenv(EnvConfig)
	if flagPath != "" {
		path, err := cliPath(flagPath)
		if err != nil {
			return Location{}, fmt.Errorf("--config: the path %w", err)
		}
		why := "the flag --config"
		if env != "" {
			why += " (it has priority over the environment variable " + EnvConfig + ")"
		}
		return Location{Path: path, Source: SourceFlag, Why: why}, nil
	}
	if env != "" {
		path, err := absPath(env)
		if err != nil {
			return Location{}, fmt.Errorf("%s: the path %w", EnvConfig, err)
		}
		return Location{Path: path, Source: SourceEnv, Why: "the environment variable " + EnvConfig}, nil
	}
	path, err := DefaultPath()
	if err != nil {
		return Location{}, err
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return Location{Path: path, Source: SourceXDG, Why: "the default location below XDG_CONFIG_HOME (a38/config.yaml)"}, nil
	}
	return Location{Path: path, Source: SourceHome, Why: "the default location ~/.config/a38/config.yaml"}, nil
}

// cliPath makes the value of --config absolute. A leading "~/" is the
// home directory; a relative path is relative to the working directory.
func cliPath(p string) (string, error) {
	if len(p) >= 2 && p[:2] == "~/" {
		return absPath(p)
	}
	return filepath.Abs(p)
}
