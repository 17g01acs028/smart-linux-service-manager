# Linux Service Manager (LSM)

LSM is a lightweight, Go-based service manager for Linux. It monitors your existing systemd services (or any other processes), restarts them if they fail, and supports scheduled restarts (cron).

It solves the problem of: "I need to ensure my service stays up, but also restart it every night at 3 AM, but NOT if I manually stopped it."

## Features
- **Auto-Restart**: Detects crashes and restarts services.
- **Smart Scheduling**: Cron-based scheduled restarts (e.g., `@daily`).
- **Safe Manual Stops**: Respects manual stops (won't restart a service you intentionally stopped).
- **Log Rotation**: Built-in log rotation (Lumberjack).
- **Zero Dependencies**: Single static binary.

## How It Works

LSM runs a background loop (every 10 seconds) that checks the health of your services. It uses three commands you provide:
1.  **Check Command**: "Is the service healthy?" (Exit 0 = Yes, Exit 1 = Failed).
2.  **Status Command**: "Is the service currently active/running?" (Exit 0 = Yes). Used to prevent scheduled restarts if you stopped the service.
3.  **Restart Command**: The command to run if the Check fails.

---

## Detailed Scenarios & Examples

### Scenario 1: The Standard Web Server (Nginx)
*Goal: Restart Nginx if it crashes (error), but NOT if I stop it manually.*

```bash
sudo lsm add \
  --name "nginx" \
  --restart "systemctl restart nginx" \
  --check "! systemctl is-failed nginx" \
  --status "systemctl is-active nginx" \
  --schedule "@daily"
```
**How it works:**
- **Check (`! is-failed`)**: Returns "Failed" (Exit 1) only if systemd marks the service as `failed` (red). If you `stop` nginx, it goes to `inactive` (not failed), so LSM leaves it alone.
- **Schedule**: Every day at midnight, it checks `status`. If Nginx is running, it restarts it. If stopped, it skips.

### Scenario 2: The "Keep-Alive" (Strict Mode)
*Goal: This custom app (`my-worker`) must ALWAYS be running. If it exits (even cleanly with code 0), start it again immediately.*

```bash
sudo lsm add \
  --name "my-worker" \
  --restart "systemctl start my-worker" \
  --check "systemctl is-active my-worker" \
  --status "systemctl is-active my-worker"
```
**How it works:**
- **Check (`is-active`)**: Returns "Failed" (Exit 1) if the service is NOT active.
- **Result**: If the app stops for *any* reason, LSM restarts it.
- **Caveat**: To stop this app manually, you MUST disable LSM monitoring first (`lsm toggle`), otherwise LSM will fight you and restart it.

### Scenario 3: Memory Leak Mitigation (Scheduled Only)
*Goal: Don't monitor for crashes (systemd handles that), just restart it every 4 hours to clear memory.*

```bash
sudo lsm add \
  --name "heavy-app" \
  --restart "systemctl restart heavy-app" \
  --check "true" \
  --status "systemctl is-active heavy-app" \
  --schedule "0 */4 * * *"
```
**How it works:**
- **Check (`true`)**: Always returns "OK". LSM never "repairs" it.
- **Schedule**: Cron runs every 4 hours. It checks `status`. If running, it restarts.

---

## Installation

### 1. Download & Install
1.  Copy the binary `lsm-linux` and `install.sh` to your server.
2.  Run the install script:

```bash
chmod +x install.sh
sudo ./install.sh
```

This will:
- Install `lsm` to `/usr/local/bin/lsm`.
- Create a systemd service `lsm.service` (running as root).
- Create log directory `/var/log/lsm/` (readable by everyone).

### 2. Verify
Check if the daemon is running:
```bash
sudo systemctl status lsm
```

Tail the logs:
```bash
tail -f /var/log/lsm/lsm.log
```

## Usage

All commands must be run as `root` (sudo) because LSM manages system services.

### 1. Add a Service

**Standard Mode (Recommended)**
Restarts ONLY if the service crashes (exit status != 0).
```bash
sudo lsm add \
  --name "nginx" \
  --restart "systemctl restart nginx" \
  --check "! systemctl is-failed nginx" \
  --status "systemctl is-active nginx" \
  --schedule "@daily"
```

**Strict Mode (Always Running)**
Restarts if the service stops for ANY reason (even a clean exit code 0).
*Use this for apps that might exit cleanly but should still be running.*
```bash
sudo lsm add \
  --name "crasher" \
  --restart "systemctl start crasher" \
  --check "systemctl is-active crasher" \
  --status "systemctl is-active crasher" \
  --schedule "@daily"
```

### 2. List Services
View all monitored services and their last status.
```bash
sudo lsm list
```

### 3. Update a Service
Change settings for an existing service.
```bash
# Change schedule to hourly
sudo lsm update --name "nginx" --schedule "@hourly"

# Disable schedule (only monitor crashes)
sudo lsm update --name "nginx" --schedule ""
```

### 4. Toggle Monitoring
Temporarily disable monitoring for a service (useful for maintenance).
```bash
sudo lsm toggle --name "nginx"
```

### 5. Remove a Service
Stop monitoring and remove from LSM database.
```bash
sudo lsm remove --name "nginx"
```

### 7. Configure Logging
Adjust log rotation settings.
```bash
sudo lsm config-log --max-size 50 --max-backups 10 --compress=true
```

### 8. Smart Pause (Maintenance Mode)
Prevent LSM from restarting services while you are working on the server.
If enabled, LSM checks if *any* user is logged in (via SSH or terminal). If yes, it pauses monitoring.
```bash
# Enable Smart Pause
sudo lsm config-pause --enable=true

# Disable (Default)
sudo lsm config-pause --enable=false
```

## Configuration Details

### The Flags

| Flag | Description | Example |
|------|-------------|---------|
| `--name` | Unique name for the service in LSM. | `my-app` |
| `--restart` | Command LSM runs to start/restart the service. | `systemctl start my-app` |
| `--check` | Command to check health. **Exit 0 = OK, Exit 1 = Failed.** | `! systemctl is-failed my-app` |
| `--status` | Command to check if active. Used by scheduler to avoid starting stopped apps. | `systemctl is-active my-app` |
| `--schedule` | Cron expression for periodic restarts. | `@daily`, `0 4 * * *` |

### Database & Logs
- **Database**: `/var/lib/lsm/lsm.db` (SQLite)
- **Logs**: `/var/log/lsm/lsm.log`



