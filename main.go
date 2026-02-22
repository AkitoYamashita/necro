package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version  int `yaml:"version"`
	Defaults struct {
		Region string `yaml:"region"`
	} `yaml:"defaults"`
	Targets struct {
		Profiles []string `yaml:"profiles"`
		Exclude  []string `yaml:"exclude"`
	} `yaml:"targets"`
	Cmd []struct {
		Name string   `yaml:"name"`
		Run  []string `yaml:"run"`
	} `yaml:"cmd"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: necro <config-file>")
		os.Exit(1)
	}

	cfgPath := os.Args[1]

	cfgData, err := os.ReadFile(cfgPath)
	if err != nil {
		panic(err)
	}

	var cfg Config
	if err := yaml.Unmarshal(cfgData, &cfg); err != nil {
		panic(err)
	}

	region := cfg.Defaults.Region
	if region == "" {
		region = "ap-northeast-1"
	}

	profiles := cfg.Targets.Profiles
	if len(profiles) == 0 {
		fmt.Println("No profiles specified. Loading from ~/.aws/config ...")
		profiles = loadProfilesFromAWSConfig()
	}

	profiles = applyExclude(profiles, cfg.Targets.Exclude)

	if len(profiles) == 0 {
		fmt.Println("No profiles to run.")
		os.Exit(1)
	}

	// ---- Preview ----
	fmt.Println("==== TARGET PROFILES ====")
	for _, p := range profiles {
		fmt.Println("-", p)
	}

	fmt.Println("")
	fmt.Println("==== COMMANDS ====")
	for _, c := range cfg.Cmd {
		fmt.Println("-", c.Name)
	}

	fmt.Print("\nProceed? (y/N): ")
	var input string
	fmt.Scanln(&input)
	if strings.ToLower(input) != "y" {
		fmt.Println("Cancelled.")
		os.Exit(0)
	}

	// ---- Execute ----
	for _, profile := range profiles {
		fmt.Println("\n==== PROFILE:", profile, "====")

		for _, c := range cfg.Cmd {
			fmt.Println("->", c.Name)
			runAWS(profile, region, c.Run)
		}
	}
}

func applyExclude(profiles, exclude []string) []string {
	if len(exclude) == 0 {
		return profiles
	}

	exSet := make(map[string]struct{})
	for _, e := range exclude {
		exSet[e] = struct{}{}
	}

	var filtered []string
	for _, p := range profiles {
		if _, found := exSet[p]; !found {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func loadProfilesFromAWSConfig() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	path := filepath.Join(home, ".aws", "config")

	f, err := os.Open(path)
	if err != nil {
		fmt.Println("cannot open ~/.aws/config:", err)
		return nil
	}
	defer f.Close()

	var profiles []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[profile ") && strings.HasSuffix(line, "]") {
			name := strings.TrimPrefix(line, "[profile ")
			name = strings.TrimSuffix(name, "]")
			name = strings.TrimSpace(name)
			profiles = append(profiles, name)
		}
	}
	return profiles
}

func runAWS(profile, region string, args []string) {
	full := []string{
		"--profile", profile,
		"--region", region,
		"--output", "json",
	}
	full = append(full, args...)

	cmd := exec.Command("aws", full...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		fmt.Println(stderr.String())
		os.Exit(1)
	}

	var pretty bytes.Buffer
	if json.Indent(&pretty, out.Bytes(), "", "  ") == nil {
		fmt.Println(pretty.String())
	} else {
		fmt.Println(out.String())
	}
}
