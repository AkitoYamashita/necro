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
	Vars struct {
		EnvMap map[string]string `yaml:"envMap"`
	} `yaml:"vars"`
	Cmd []struct {
		Name string   `yaml:"name"`
		Run  []string `yaml:"run"`
	} `yaml:"cmd"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: necro <config-file> [--dry-run]")
		os.Exit(1)
	}

	cfgPath := os.Args[1]
	dryRun := false
	if len(os.Args) > 2 && os.Args[2] == "--dry-run" {
		dryRun = true
	}

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
		profiles = loadProfilesFromAWSConfig()
	}

	profiles = applyExclude(profiles, cfg.Targets.Exclude)

	if len(profiles) == 0 {
		fmt.Println("No profiles to run.")
		os.Exit(1)
	}

	fmt.Println("==== TARGET PROFILES ====")
	for _, p := range profiles {
		fmt.Println("-", p)
	}

	fmt.Println("\n==== COMMANDS ====")
	for _, c := range cfg.Cmd {
		fmt.Println("-", c.Name)
	}

	if dryRun {
		fmt.Println("\n==== DRY RUN PLAN ====")
	}

	for _, profile := range profiles {
		accountID := getAccountID(profile, region)
		env := cfg.Vars.EnvMap[profile]

		for _, c := range cfg.Cmd {
			finalArgs := buildCommand(profile, region, accountID, env, c.Run)

			if dryRun {
				fmt.Println(strings.Join(finalArgs, " "))
				continue
			}

			runAWS(finalArgs)
		}
	}

	if dryRun {
		os.Exit(0)
	}
}

func buildCommand(profile, region, accountID, env string, args []string) []string {
	full := []string{
		"aws",
		"--profile", profile,
		"--region", region,
		"--output", "json",
	}

	for _, a := range args {
		a = strings.ReplaceAll(a, "${PROFILE}", profile)
		a = strings.ReplaceAll(a, "${ACCOUNT_ID}", accountID)
		a = strings.ReplaceAll(a, "${REGION}", region)
		a = strings.ReplaceAll(a, "${ENV}", env)
		full = append(full, a)
	}

	return full
}

func getAccountID(profile, region string) string {
	cmd := exec.Command("aws",
		"--profile", profile,
		"--region", region,
		"--output", "json",
		"sts", "get-caller-identity",
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return ""
	}

	var data struct {
		Account string `json:"Account"`
	}
	_ = json.Unmarshal(out.Bytes(), &data)
	return data.Account
}

func runAWS(full []string) {
	cmd := exec.Command(full[0], full[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func applyExclude(profiles, exclude []string) []string {
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
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".aws", "config")

	f, err := os.Open(path)
	if err != nil {
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
