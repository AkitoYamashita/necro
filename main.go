package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

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
	if handleSubcommand(os.Args) {
		return
	}

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

	// ---- Log file (single file per run) ----
	runID := newRunID()
	dieIf(os.MkdirAll("log", 0755))
	logPath := filepath.Join("log", runID+".txt")

	logFile, err := os.Create(logPath)
	dieIf(err)
	defer logFile.Close()

	// write to console + log file
	mw := io.MultiWriter(os.Stdout, logFile)

	fmt.Fprintf(mw, "🧾 LOG FILE | %s\n", logPath)
	fmt.Fprintf(mw, "🆔 RUN ID   | %s\n", runID)

	// ---- Preview ----
	fmt.Fprintln(mw, "\n==== TARGET PROFILES ====")
	for _, p := range profiles {
		fmt.Fprintln(mw, "-", p)
	}

	fmt.Fprintln(mw, "\n==== COMMANDS ====")
	for _, c := range cfg.Cmd {
		fmt.Fprintln(mw, "-", c.Name)
	}

	if dryRun {
		fmt.Fprintln(mw, "\n==== DRY RUN PLAN ====")
	} else {
		// Proceed prompt should be console-only (avoid polluting log with user input)
		if !confirmProceed() {
			fmt.Fprintln(mw, "Cancelled.")
			os.Exit(0)
		}
	}

	// ---- Execute/Plan ----
	// 1) Build ctx cache per profile (STS check happens here once per profile)
	ctxByProfile := make(map[string]map[string]string, len(profiles))

	for _, profile := range profiles {
		accountID, arn, errText, e := getCallerIdentity(profile, region)
		if e != nil {
			fmt.Fprintf(mw, "❌ STS NG   | profile=%s\n", profile)
			if errText != "" {
				fmt.Fprintf(mw, "   stderr  | %s\n", errText)
			}
			die(e)
		}
		_ = arn // keep for future verbose use

		fmt.Fprintf(mw, "🔐 STS OK   | profile=%s | account=%s\n", profile, accountID)

		// Built-in context (highest priority)
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
		resolved, e := resolveContext(ctx)
		if e != nil {
			die(fmt.Errorf("profile %s: %w", profile, e))
		}
		ctxByProfile[profile] = resolved
	}

	// 2) Run cmd-by-cmd (gate). cmd1 across all profiles -> cmd2 across all profiles ...
	for _, c := range cfg.Cmd {
		if dryRun {
			fmt.Fprintf(mw, "\n🧪 CMD PLAN  | %s\n", c.Name)
		} else {
			fmt.Fprintf(mw, "\n🚀 CMD START | %s\n", c.Name)
		}

		for _, profile := range profiles {
			ctx := ctxByProfile[profile]

			finalArgs, e := renderAWSArgs(profile, region, c.Run, ctx)
			if e != nil {
				fmt.Fprintf(mw, "❌ CMD NG    | %s | profile=%s (render)\n", c.Name, profile)
				die(fmt.Errorf("profile %s cmd %s: %w", profile, c.Name, e))
			}

			if dryRun {
				fmt.Fprintf(mw, "🧪 RUN PLAN  | %s | profile=%s\n", c.Name, profile)
				fmt.Fprintln(mw, strings.Join(finalArgs, " "))
				continue
			}

			fmt.Fprintf(mw, "▶️  RUN       | %s | profile=%s\n", c.Name, profile)
			if e := runAWSWithError(finalArgs, mw); e != nil {
				fmt.Fprintf(mw, "❌ RUN NG    | %s | profile=%s\n", c.Name, profile)
				die(e) // stop immediately
			}
			fmt.Fprintf(mw, "✅ RUN OK    | %s | profile=%s\n", c.Name, profile)
		}

		if dryRun {
			fmt.Fprintf(mw, "🧪 CMD DONE  | %s\n", c.Name)
		} else {
			fmt.Fprintf(mw, "🚀 CMD OK    | %s\n", c.Name)
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
	fmt.Println("  necro version")
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
		"--no-cli-pager",
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
func getCallerIdentity(profile, region string) (accountID string, arn string, errText string, err error) {
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

	// suppress interactive behaviors (pager / auto prompt)
	cmd.Env = append(os.Environ(),
		"AWS_PAGER=",
		"AWS_CLI_AUTO_PROMPT=off",
	)

	if e := cmd.Run(); e != nil {
		return "", "", strings.TrimSpace(stderr.String()), fmt.Errorf("sts failed for %s", profile)
	}

	var data struct {
		Account string `json:"Account"`
		Arn     string `json:"Arn"`
	}
	if e := json.Unmarshal(out.Bytes(), &data); e != nil {
		return "", "", "", fmt.Errorf("sts json parse failed for %s: %v", profile, e)
	}
	if strings.TrimSpace(data.Account) == "" {
		return "", "", "", fmt.Errorf("sts returned empty Account for %s", profile)
	}
	if strings.TrimSpace(data.Arn) == "" {
		return "", "", "", fmt.Errorf("sts returned empty Arn for %s", profile)
	}

	return data.Account, data.Arn, "", nil
}

func runAWSWithError(full []string, w io.Writer) error {
	cmd := exec.Command(full[0], full[1:]...)
	cmd.Stdout = w
	cmd.Stderr = w

	// suppress interactive behaviors (pager / auto prompt)
	cmd.Env = append(os.Environ(),
		"AWS_PAGER=",
		"AWS_CLI_AUTO_PROMPT=off",
	)

	return cmd.Run()
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

func handleSubcommand(args []string) bool {
	// subcommands: version, help
	if len(args) < 2 {
		return false
	}

	switch args[1] {
	case "version":
		fmt.Printf("necro %s (commit=%s, date=%s)\n", version, commit, date)
		return true
	case "help", "-h", "--help":
		usage()
		return true
	default:
		return false
	}
}
func newRunID() string {
	// e.g. 20260223-170501-123-abc123
	now := time.Now()
	ts := now.Format("20060102-150405")
	ms := fmt.Sprintf("%03d", now.Nanosecond()/1e6)

	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return ts + "-" + ms + "-000000"
	}

	return ts + "-" + ms + "-" + hex.EncodeToString(b)
}
