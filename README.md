# necro

necro (Necromancer) is a small CLI to orchestrate operations across multiple AWS accounts using AWS SSO profiles.

![alt text](necro.png)

## Goals (v0)

- Run the same AWS operation across multiple ~/.aws/config SSO profiles from one place
- Default region: ap-northeast-1 (Tokyo), but per-command region override is supported
- Prefer idempotent-style operations (check -> create -> configure)
- Public repo + drop a single binary on another machine and run (requires AWS CLI installed)

## Install

## Windows

Download necro_windows_amd64.exe from GitHub Releases and run it.

## Mac ※arm64 for MacBookPro(M2)

Download necro_darwin_arm64 from GitHub Releases and run it.

```bash
xattr -d com.apple.quarantine necro_darwin_arm64
chmod +x necro_darwin_arm64
```

## Quick start

### 1. Prepare AWS config (SSO)

Use the sample config as a reference:

    conf/sample_aws_config

Create or update:

    ~/.aws/config

Then login:

    aws sso login --profile SAMPLE_PROFILE

### 2. Prepare task config

Use the sample task file:

    conf/sample_task.yml

Copy it and adjust for your environment:

    cp conf/sample_task.yml conf/task1.yml

Edit:

    conf/task1.yml

### 3. Run necro

Check build info:

    necro version

Dry run (see planned commands only):

    necro conf/task1.yml --dry-run

Execute:

    necro conf/task1.yml

## Requirements

- AWS CLI v2 installed and available as aws
- ~/.aws/config exists (SSO profiles live here)
- SSO session cache is valid (otherwise run:
      aws sso login --profile <name>
  )

## Roadmap

- YAML config runner (targets + steps) with --output json enforced
- run/apply: execute AWS CLI steps across targets with region overrides
- report: JSON summary output
