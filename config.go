package main

import (
	"io"
	"os"
	"os/exec"
	"log"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BackgroundCommand      string                  `yaml:"background_command"`
	DefaultExtendDirection string                  `yaml:"default_extend_direction"`
	Wait                   string		       `yaml:"wait"`
	Setups                 map[string]*SetupConfig `yaml:"setups"`
}

type SetupConfig struct {
	Outputs  []OutputConfig `yaml:"outputs"`
}

type OutputConfig struct {
	Monitor  string `yaml:"monitor"`
	Position struct {
		X int `yaml:"x"`
		Y int `yaml:"y"`
	} `yaml:"position"`
	Primary           bool `yaml:"primary"`
	DisableOnLidClose bool `yaml:"disable_on_lid_close"`
}

func setupScore(outputs []*Output, setup *SetupConfig) int {
	var score int

	for _, outputConfig := range setup.Outputs {
		found := false
		for _, output := range outputs {
			if outputConfig.Monitor == output.monitorId {
				found = true
				break
			}
		}

		if !found {
			return 0
		}

		score += 1
	}

	return score
}

func (c *Config) getOutputConfig(setup string, monitorId string) *OutputConfig {
	if setup == "" {
		return nil
	}

	for _, outputConfig := range c.Setups[setup].Outputs {
		if outputConfig.Monitor == monitorId {
			return &outputConfig
		}
	}
	return nil
}

func (c *Config) executeBackgroundCommand() {
	cmdline := c.BackgroundCommand
	if cmdline == "" {
		return
	}

	log.Printf("Executing command: %s\n", cmdline)
	cmd := exec.Command("sh", "-c", cmdline)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("Command error: %v\n", err)
	}
}

func (c *Config) findBestSetup(outputs []*Output) string {
	var bestScore int
	var bestSetup string

	bestScore = 0
	for name, setup := range c.Setups {
		score := setupScore(outputs, setup)
		if score > bestScore {
			bestScore = score
			bestSetup = name
		}
	}

	return bestSetup
}

func parseConfigStream(r io.Reader) (*Config, error) {
	var conf Config

	decoder := yaml.NewDecoder(r)

	err := decoder.Decode(&conf)
	if err != nil {
		return nil, err
	}

	return &conf, nil
}

func parseConfigFile(name string) (*Config, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	conf, err := parseConfigStream(f)
	if err != nil {
		return nil, err
	}
	return conf, nil
}
