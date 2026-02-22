package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const defaultRegion = "ap-northeast-1"

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "hello":
		fmt.Println("necro: hello")

	case "version":
		fmt.Printf("necro %s (commit=%s, date=%s)\n", version, commit, date)

	case "doctor":
		profile := getProfile()
		if profile == "" {
			exit("doctor requires --profile")
		}
		runAWS(profile, defaultRegion, []string{"sts", "get-caller-identity"})

	case "s3":
		profile := getProfile()
		if profile == "" {
			exit("s3 requires --profile")
		}
		runAWS(profile, defaultRegion, []string{"s3api", "list-buckets"})

	case "help", "-h", "--help":
		usage()

	default:
		exit("unknown command")
	}
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

	// pretty print JSON
	var pretty bytes.Buffer
	if json.Indent(&pretty, out.Bytes(), "", "  ") == nil {
		fmt.Println(pretty.String())
	} else {
		fmt.Println(out.String())
	}
}

func getProfile() string {
	for i, a := range os.Args {
		if a == "--profile" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func usage() {
	fmt.Println("necro - multi-account AWS helper CLI")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  necro doctor --profile <name>")
	fmt.Println("  necro s3     --profile <name>")
	fmt.Println("  necro version")
	fmt.Println("")
}

func exit(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}
