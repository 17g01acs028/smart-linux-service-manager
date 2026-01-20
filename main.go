package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"linux_service_manager/internal/db"
	"linux_service_manager/internal/logger"
	"linux_service_manager/internal/monitor"
	"linux_service_manager/internal/scheduler"
	"text/tabwriter"
)

const dbPath = "/var/lib/lsm/lsm.db"
const logPath = "/var/log/lsm/lsm.log"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Commands that require root
	if requiresRoot(command) {
		if os.Geteuid() != 0 {
			fmt.Println("Error: This command requires root privileges. Please run with sudo.")
			os.Exit(1)
		}
	}

	// Init DB for all commands
	if err := db.InitDB(dbPath); err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}

	switch command {
	case "daemon":
		runDaemon()
	case "add":
		runAdd(os.Args[2:])
	case "remove":
		runRemove(os.Args[2:])
	case "update":
		runUpdate(os.Args[2:])
	case "list":
		runList()
	case "toggle":
		runToggle(os.Args[2:])
	case "config-log":
		runConfigLog(os.Args[2:])
	case "config-pause":
		runConfigPause(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: lsm <command> [args]")
	fmt.Println("Commands:")
	fmt.Println("  daemon                    Start the monitoring and scheduling daemon")
	fmt.Println("  add [flags]               Add a new service")
	fmt.Println("  remove --name <name>      Remove a service")
	fmt.Println("  update [flags]            Update an existing service")
	fmt.Println("  list                      List all services")
	fmt.Println("  toggle --name <service>   Toggle service monitoring (enable/disable)")
	fmt.Println("  config-log [flags]        Configure logging settings")
	fmt.Println("  config-pause [flags]      Configure Smart Pause (active user detection)")
	fmt.Println("\nAdd/Update Flags:")
	fmt.Println("  --name      Service name (unique)")
	fmt.Println("  --restart   Command to restart the service")
	fmt.Println("  --check     Command to check health (exit != 0 means failed)")
	fmt.Println("  --status    Command to check status (exit 0 means running). Used for safe scheduling.")
	fmt.Println("  --schedule  Cron schedule (e.g. '@daily', '0 0 * * *'). Leave empty for none.")
}

func runDaemon() {
	// Init Logger
	logger.Init(logPath)

	// Start Scheduler
	scheduler.Start()
	defer scheduler.Stop()

	// Start Monitor Loop
	// Run in goroutine? RunLoop blocks.
	// But we need to handle signals.

	go monitor.RunLoop(10 * time.Second) // Check every 10s

	log.Println("LSM Daemon started. Press Ctrl+C to exit.")

	// Wait for signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	monitor.Stop()
	log.Println("Shutting down...")
}

func runAdd(args []string) {
	addCmd := flag.NewFlagSet("add", flag.ExitOnError)
	name := addCmd.String("name", "", "Service name")
	restart := addCmd.String("restart", "", "Restart command")
	check := addCmd.String("check", "", "Check command")
	status := addCmd.String("status", "", "Status command")
	schedule := addCmd.String("schedule", "", "Cron schedule")
	// enabled by default

	addCmd.Parse(args)

	if *name == "" || *restart == "" || *check == "" {
		fmt.Println("Error: name, restart, and check are required.")
		addCmd.PrintDefaults()
		os.Exit(1)
	}

	svc := db.Service{
		Name:           *name,
		RestartCommand: *restart,
		CheckCommand:   *check,
		StatusCommand:  *status,
		CronSchedule:   *schedule,
		Enabled:        true,
	}

	if err := db.AddService(svc); err != nil {
		log.Fatalf("Failed to add service: %v", err)
	}
	fmt.Printf("Service '%s' added successfully.\n", *name)
}

func runList() {
	services, err := db.ListServices()
	if err != nil {
		log.Fatalf("Failed to list services: %v", err)
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprintln(w, "ID\tName\tSchedule\tEnabled\tLast Checked\tLast Restarted")

	for _, s := range services {
		lastChecked := "-"
		if s.LastChecked != nil {
			lastChecked = s.LastChecked.Format(time.RFC3339)
		}
		lastRestarted := "-"
		if s.LastRestarted != nil {
			lastRestarted = s.LastRestarted.Format(time.RFC3339)
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%t\t%s\t%s\n",
			s.ID, s.Name, s.CronSchedule, s.Enabled, lastChecked, lastRestarted,
		)
	}
	w.Flush()
}

func runToggle(args []string) {
	toggleCmd := flag.NewFlagSet("toggle", flag.ExitOnError)
	name := toggleCmd.String("name", "", "Service name")

	toggleCmd.Parse(args)

	if *name == "" {
		fmt.Println("Error: --name is required.")
		os.Exit(1)
	}

	svc, err := db.GetService(*name)
	if err != nil {
		log.Fatalf("Failed to get service: %v", err)
	}

	newState := !svc.Enabled
	if err := db.ToggleService(*name, newState); err != nil {
		log.Fatalf("Failed to toggle service: %v", err)
	}

	fmt.Printf("Service '%s' enabled set to %t.\n", *name, newState)
}

func runRemove(args []string) {
	cmd := flag.NewFlagSet("remove", flag.ExitOnError)
	name := cmd.String("name", "", "Service name")
	cmd.Parse(args)

	if *name == "" {
		fmt.Println("Error: --name is required.")
		os.Exit(1)
	}

	if err := db.RemoveService(*name); err != nil {
		log.Fatalf("Failed to remove service: %v", err)
	}
	fmt.Printf("Service '%s' removed. (Restart daemon to fully apply changes if running)\n", *name)
}

func runUpdate(args []string) {
	cmd := flag.NewFlagSet("update", flag.ExitOnError)
	name := cmd.String("name", "", "Service name (required)")
	restart := cmd.String("restart", "", "Restart command")
	check := cmd.String("check", "", "Check command")
	status := cmd.String("status", "", "Status command")
	schedule := cmd.String("schedule", "", "Cron schedule")
	// enabled := cmd.Bool("enabled", true, "Enabled") // Hard to handle optional bool with flags, skipping for now. Use toggle.

	cmd.Parse(args)

	if *name == "" {
		fmt.Println("Error: --name is required.")
		os.Exit(1)
	}

	// Fetch existing to mix/match
	existing, err := db.GetService(*name)
	if err != nil {
		log.Fatalf("Failed to get service '%s' (does it exist?): %v", *name, err)
	}

	if *restart != "" {
		existing.RestartCommand = *restart
	}
	if *check != "" {
		existing.CheckCommand = *check
	}
	if *status != "" {
		existing.StatusCommand = *status
	}
	// Schedule can be empty string, so we need a way to know if user passed it.
	// flag.Visit is a good way but for simplicity let's assume if it is explicitly passed as "" it clears it.
	// But default is "".
	// Let's assume user must pass -schedule="" to clear it? No, default is empty.
	// We'll trust user passed what they want if they passed the flag.
	// Actually, checking if flag was set is safer.
	isScheduleSet := false
	cmd.Visit(func(f *flag.Flag) {
		if f.Name == "schedule" {
			isScheduleSet = true
		}
	})
	if isScheduleSet {
		existing.CronSchedule = *schedule
	}

	if err := db.UpdateService(*existing); err != nil {
		log.Fatalf("Failed to update service: %v", err)
	}
	fmt.Printf("Service '%s' updated. (Restart daemon to apply changes)\n", *name)
}

func runConfigLog(args []string) {
	cmd := flag.NewFlagSet("config-log", flag.ExitOnError)
	maxSize := cmd.Int("max-size", 0, "Max size in MB")
	maxBackups := cmd.Int("max-backups", 0, "Max number of old log files")
	maxAge := cmd.Int("max-age", 0, "Max age in days")
	compress := cmd.Bool("compress", true, "Compress old log files")

	cmd.Parse(args)

	existing, err := db.GetLogConfig()
	if err != nil {
		// Ignore err, start fresh
		existing = &db.LogConfig{}
	}

	if *maxSize > 0 {
		existing.MaxSize = *maxSize
	}
	if *maxBackups > 0 {
		existing.MaxBackups = *maxBackups
	}
	if *maxAge > 0 {
		existing.MaxAge = *maxAge
	}
	// compress is bool, tricky if user wants false but default is true.
	// But flags default is true here. If user passes --compress=false it works.
	existing.Compress = *compress

	if err := db.SetLogConfig(*existing); err != nil {
		log.Fatalf("Failed to update log config: %v", err)
	}
	if err := db.SetLogConfig(*existing); err != nil {
		log.Fatalf("Failed to update log config: %v", err)
	}
	fmt.Println("Log configuration updated. Restart daemon to apply.")
}

func runConfigPause(args []string) {
	cmd := flag.NewFlagSet("config-pause", flag.ExitOnError)
	enable := cmd.Bool("enable", false, "Enable/Disable Smart Pause")
	// flag check is tricky for bool default false. User might pass --enable=true or just --enable.
	// But getting current state requires DB.

	cmd.Parse(args)

	// We just blindly set what user requests.
	if err := db.SetPauseConfig(*enable); err != nil {
		log.Fatalf("Failed to update pause config: %v", err)
	}
	fmt.Printf("Smart Pause configuration updated (Enabled: %t). Restart daemon to apply.\n", *enable)
}

func requiresRoot(cmd string) bool {
	switch cmd {
	case "daemon", "add", "remove", "update", "toggle", "config-log", "config-pause":
		return true
	case "list":
		// List might be allowed if DB is readable, but /var/lib/lsm might be root only.
		// For now let's allow it to try, OS will deny if permission missing.
		// But consistency says maybe check root too?
		// User said "cannot do any other activity". Assuming list is passive.
		return false
	}
	return false
}
