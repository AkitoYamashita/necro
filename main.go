package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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
		Defaults map[string]string            `yaml:"defaults"`
		Profiles map[string]map[string]string `yaml:"profiles"`
	} `yaml:"vars"`
	Cmd []struct {
		Name string   `yaml:"name"`
		Run  []string `yaml:"run"`
	} `yaml:"cmd"`
}

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var reVar = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)

func main() {
	cfgPath, dryRun := parseArgs(os.Args)
	if cfgPath == "" {
		usage()
		os.Exit(1)
	}

	cfgData, err := os.ReadFile(cfgPath)
	dieIf(err)

	var cfg Config
	dieIf(yaml.Unmarshal(cfgData, &cfg))

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
	if len(cfg.Cmd) == 0 {
		fmt.Println("No cmd to run.")
		os.Exit(1)
	}

	// ---- Preview ----
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
	} else {
		if !confirmProceed() {
			fmt.Println("Cancelled.")
			os.Exit(0)
		}
	}

	// ---- Execute/Plan ----
	for _, profile := range profiles {
		accountID := mustGetAccountID(profile, region)

		// Build built-in context (highest priority)
		ctx := map[string]string{
			"PROFILE":    profile,
			"REGION":     region,
			"ACCOUNT_ID": accountID,
		}

		// Merge vars.defaults (cannot override built-in)
		mergeVarsNoOverride(ctx, cfg.Vars.Defaults)

		// Merge vars.profiles[PROFILE] (cannot override built-in)
		if pv, ok := cfg.Vars.Profiles[profile]; ok {
			mergeVarsNoOverride(ctx, pv)
		}

		// Resolve template references inside ctx values (after merge)
		ctx, err = resolveContext(ctx)
		if err != nil {
			die(fmt.Errorf("profile %s: %w", profile, err))
		}

		for _, c := range cfg.Cmd {
			finalArgs, err := renderAWSArgs(profile, region, c.Run, ctx)
			if err != nil {
				die(fmt.Errorf("profile %s cmd %s: %w", profile, c.Name, err))
			}

			if dryRun {
				fmt.Println(strings.Join(finalArgs, " "))
				continue
			}

			fmt.Println("\n==== PROFILE:", profile, "====")
			fmt.Println("->", c.Name)
			runAWS(finalArgs)
		}
	}
}

func parseArgs(args []string) (cfgPath string, dryRun bool) {
	// usage: necro <config-file> [--dry-run]
	// accept --dry-run anywhere after program name
	for i := 1; i < len(args); i++ {
		if args[i] == "--dry-run" {
			dryRun = true
			continue
		}
		if cfgPath == "" && !strings.HasPrefix(args[i], "-") {
			cfgPath = args[i]
		}
	}
	return cfgPath, dryRun
}

func usage() {
	fmt.Printf("necro %s (commit=%s, date=%s)\n", version, commit, date)
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  necro <config-file> [--dry-run]")
}

func confirmProceed() bool {
	fmt.Print("\nProceed? (y/N): ")
	in := bufio.NewScanner(os.Stdin)
	if !in.Scan() {
		return false
	}
	return strings.ToLower(strings.TrimSpace(in.Text())) == "y"
}

func mergeVarsNoOverride(dst map[string]string, add map[string]string) {
	if add == nil {
		return
	}
	for k, v := range add {
		// built-in keys must win
		if isBuiltInKey(k) {
			continue
		}
		dst[k] = v
	}
}

func isBuiltInKey(k string) bool {
	return k == "PROFILE" || k == "REGION" || k == "ACCOUNT_ID"
}

func resolveContext(ctx map[string]string) (map[string]string, error) {
	// resolve ${VAR} inside ctx values using ctx itself
	// error if referenced VAR is not defined
	// loop until stable, with a small cap to avoid infinite recursion
	out := copyMap(ctx)

	for step := 0; step < 20; step++ {
		changed := false

		// stable iteration order (debuggability)
		keys := make([]string, 0, len(out))
		for k := range out {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := out[k]
			nv, didChange, err := expandStrict(v, out)
			if err != nil {
				return nil, err
			}
			if didChange {
				out[k] = nv
				changed = true
			}
		}

		if !changed {
			break
		}
	}

	// final check: no unresolved placeholders
	for k, v := range out {
		if reVar.MatchString(v) {
			return nil, fmt.Errorf("unresolved variable remains in %s: %s", k, v)
		}
	}

	return out, nil
}

func renderAWSArgs(profile, region string, run []string, ctx map[string]string) ([]string, error) {
	// build final: aws --profile ... --region ... --output json + rendered run args
	full := []string{
		"aws",
		"--profile", profile,
		"--region", region,
		"--output", "json",
	}

	for _, a := range run {
		na, _, err := expandStrict(a, ctx)
		if err != nil {
			return nil, err
		}
		if reVar.MatchString(na) {
			return nil, fmt.Errorf("unresolved variable remains in cmd arg: %s", na)
		}
		full = append(full, na)
	}

	return full, nil
}

func expandStrict(s string, ctx map[string]string) (string, bool, error) {
	changed := false

	out := reVar.ReplaceAllStringFunc(s, func(m string) string {
		sub := reVar.FindStringSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		key := sub[1]
		val, ok := ctx[key]
		if !ok {
			// signal error by returning a special marker
			return "\x00MISSING:" + key + "\x00"
		}
		changed = true
		return val
	})

	if strings.Contains(out, "\x00MISSING:") {
		// extract first missing key
		i := strings.Index(out, "\x00MISSING:")
		j := strings.Index(out[i:], "\x00")
		miss := out[i : i+j]
		miss = strings.TrimPrefix(miss, "\x00MISSING:")
		return "", false, fmt.Errorf("undefined variable: %s", miss)
	}

	return out, changed, nil
}

func mustGetAccountID(profile, region string) string {
	// We keep region + output json for consistency.
	cmd := exec.Command("aws",
		"--profile", profile,
		"--region", region,
		"--output", "json",
		"sts", "get-caller-identity",
	)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		die(fmt.Errorf("sts failed for %s: %s", profile, strings.TrimSpace(stderr.String())))
	}

	var data struct {
		Account string `json:"Account"`
	}
	if err := json.Unmarshal(out.Bytes(), &data); err != nil {
		die(fmt.Errorf("sts json parse failed for %s: %v", profile, err))
	}
	if strings.TrimSpace(data.Account) == "" {
		die(fmt.Errorf("sts returned empty Account for %s", profile))
	}
	return data.Account
}

func runAWS(full []string) {
	cmd := exec.Command(full[0], full[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
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

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func dieIf(err error) {
	if err != nil {
		die(err)
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
