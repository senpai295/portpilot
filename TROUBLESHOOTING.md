# PortPilot Troubleshooting & Advanced Guide 🛠️

This guide covers common issues, platform-specific limitations, permission requirements, and advanced automation patterns for PortPilot.

## 🔍 Common Issues & Solutions

### 1. Process Inspection Shows “Unverified” or Missing PIDs

**Symptoms:** Running `portpilot inspect <port>` or `portpilot list` detects an occupied port, but the PID or process name appears as `unknown` or unverified.

**Possible causes:**

- The process belongs to another user or an elevated system account.
- Operating system permissions restrict process inspection.
- Required utilities such as `ss`, `lsof`, `netstat`, or `tasklist` are unavailable.
- The system utility returns incomplete or unexpected output.

**Solutions:**

**Linux / macOS**

Try running the command with elevated privileges when necessary:

```bash
sudo portpilot inspect 80
sudo portpilot list
```

Use elevated privileges only when you trust the executable and understand the security implications.

**Windows**

Open PowerShell or Command Prompt as **Administrator**, then retry the inspection.

> Process identification depends on the operating system, available utilities, and permissions. Elevated privileges do not guarantee that every process can be identified.

### 2. Command Times Out

**Symptoms:** Commands such as `portpilot check` or `portpilot scan` fail with a timeout or `context deadline exceeded` error.

**Possible causes:**

- The configured timeout is too short.
- The system is under heavy load.
- A network operation or system utility is taking longer than expected.
- The scan range is large or the machine has limited resources.

**Solutions:**

Increase the timeout if your command supports the `--timeout` flag:

```bash
portpilot check 5432 --timeout 5s
```

For range scans:

```bash
portpilot scan --start 3000 --end 4000 --timeout 3s
```

If the problem persists, try a smaller range and verify that the timeout option applies to the operation that is failing.

### 3. Custom Configuration Is Not Applied

**Symptoms:** Custom service mappings in `.portpilot.json` do not appear in scan results.

**Possible causes:**

- The JSON file is malformed.
- The configuration file is not in the expected location.
- The configuration schema does not match the implementation.
- The command does not support the `--config` flag.

**Solution:**

Verify that your configuration follows the expected schema:

```json
{
  "ports": [
    {
      "port": 4000,
      "service": "Team Internal API"
    }
  ]
}
```

If supported by your command, specify the configuration file explicitly:

```bash
portpilot scan --start 3000 --end 6000 --config ./configs/.portpilot.json
```

Validate the JSON syntax with `jq`:

```bash
jq empty ./configs/.portpilot.json
```

If validation fails, check the reported line and column for syntax errors.

### 4. Permission Denied When Terminating a Process

**Symptoms:** `portpilot kill <port>` fails to terminate the identified process.

**Possible causes:**

- The process belongs to another user.
- The current account lacks permission to terminate the process.
- The process has already exited.
- The operating system or security software prevents termination.

**Solutions:**

1. Run `portpilot inspect <port>` to verify the target.
2. Confirm that the PID still belongs to the expected application.
3. Use elevated privileges only if necessary and authorized.
4. Check the application's own logs before terminating a critical service.

The `--force` flag should skip the interactive confirmation prompt only. It should not bypass operating system permissions or target validation.

## 🚀 Advanced Automation & Scripting

PortPilot can integrate with shell scripts, CI/CD pipelines, and local development workflows through structured JSON output, provided the relevant commands support `--format json`.

The following examples assume that the documented commands and JSON fields are available in your installed version.

### 1. Automated Port Conflict Check

Fail a startup script if PostgreSQL's default port (`5432`) is occupied:

```bash
#!/usr/bin/env bash
set -euo pipefail

PORT=5432

if ! RESULT=$(portpilot check "$PORT" --format json); then
    echo "Error: Failed to check port $PORT." >&2
    exit 2
fi

if ! STATUS=$(jq -er '.status' <<< "$RESULT"); then
    echo "Error: Could not read port status from JSON output." >&2
    exit 2
fi

if [[ "$STATUS" == "OCCUPIED" ]]; then
    echo "Error: Port $PORT is already occupied!" >&2
    portpilot inspect "$PORT" || true
    exit 1
fi

echo "Port $PORT is available."
```

This example distinguishes a failed port check from an occupied port and validates the JSON field before using it.

### 2. Clean Up Development Servers

If your implementation supports `portpilot kill` and the `--force` flag, you can terminate processes occupying development ports without an interactive confirmation:

```bash
#!/usr/bin/env bash
set -euo pipefail

for PORT in 3000 5173; do
    echo "Checking development port $PORT..."

    if portpilot inspect "$PORT"; then
        echo "Attempting to terminate the process on port $PORT..."
        portpilot kill "$PORT" --force
    else
        echo "No inspectable process found on port $PORT."
    fi
done
```

**Important:** An unsuccessful inspection does not necessarily mean the port is free. Verify the command's exit-code semantics before relying on this script for automated cleanup. Review the target process before enabling unattended termination.

### 3. Generate Scheduled Markdown Audit Reports

Generate a dated Markdown report for a local development port range:

```bash
#!/usr/bin/env bash
set -euo pipefail

REPORT_DIR="./audit-reports"
REPORT_FILE="$REPORT_DIR/local-ports-$(date +%F).md"

mkdir -p "$REPORT_DIR"

portpilot scan \
    --start 3000 \
    --end 9000 \
    --output "$REPORT_FILE"

echo "Port audit report saved to: $REPORT_FILE"
```

To generate a JSON report instead:

```bash
REPORT_FILE="./audit-reports/local-ports-$(date +%F).json"

mkdir -p ./audit-reports

portpilot scan \
    --start 3000 \
    --end 9000 \
    --output "$REPORT_FILE"
```

Schedule the script using `cron` on Linux/macOS or Task Scheduler on Windows if you need recurring reports.

### 4. Find Node.js Services in JSON Output

If `portpilot list --format json` produces a `.ports` array with a `.service` field, use `jq` to filter entries:

```bash
portpilot list --format json |
    jq '.ports[] | select(.service | strings | contains("Node"))'
```

This is useful for identifying services associated with Node.js by their reported service names. It does not independently verify the executable behind each process.

## 🧪 Diagnostic Commands

Use these commands to gather basic diagnostic information when troubleshooting:

```bash
go version
go test ./...
go vet ./...
```

Check whether common inspection utilities are installed on Linux:

```bash
command -v ss
command -v lsof
```

On macOS:

```bash
command -v lsof
```

On Windows PowerShell:

```powershell
Get-Command netstat, tasklist -ErrorAction SilentlyContinue
```

## 🔐 Security Best Practices

- **Verify process identity:** Confirm the target PID and application before terminating a process.
- **Use elevated privileges sparingly:** Run PortPilot with additional permissions only when required.
- **Treat `--force` carefully:** Skipping confirmation can cause unintended service interruptions.
- **Validate machine-readable output:** Handle malformed JSON and command failures explicitly in scripts.
- **Avoid hard-coded PIDs:** PIDs can change between executions; resolve the target immediately before acting.
- **Test automation safely:** Validate scripts in a development environment before running them on shared or production systems.

## 🤝 Need More Help?

If an issue persists:

1. Reproduce it with the smallest command possible.
2. Record your operating system and Go version.
3. Include the exact command and error output.
4. Remove sensitive information before sharing diagnostic logs.
5. Open an issue in the [PortPilot GitHub repository](https://github.com/senpai295/portpilot/issues).

When reporting a bug, include the expected behavior and actual behavior to help reproduce the problem.
