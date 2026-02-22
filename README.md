# necro

necro is a small CLI to orchestrate operations across multiple AWS accounts using AWS SSO profiles.

## Goals (v0)

- Run the same AWS operation across multiple ~/.aws/config SSO profiles from one place
- Default region: ap-northeast-1 (Tokyo), but per-command region override is supported
- Prefer idempotent-style operations (check -> create -> configure)
- Public repo + drop a single binary on another machine and run (requires AWS CLI installed)

## Install

## Windows

Download necro_windows_amd64.exe from GitHub Releases and run it.

## Quick start

    necro version
    necro hello

## Requirements

- AWS CLI v2 installed and available as aws
- ~/.aws/config exists (SSO profiles live here)
- SSO session cache is valid (otherwise run:
      aws sso login --profile <name>
  )

## Roadmap

- YAML config runner (targets + steps) with --output json enforced
- doctor: check SSO cache with sts get-caller-identity across targets
- run/apply: execute AWS CLI steps across targets with region overrides
- report: JSON summary output
