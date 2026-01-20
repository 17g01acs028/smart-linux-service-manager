package monitor

import (
	"linux_service_manager/internal/db"
	"log"
	"os/exec"
	"time"
)

var stopChan = make(chan struct{})

func RunLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Starting monitoring loop with interval %v", interval)

	for {
		select {
		case <-ticker.C:
			// Check for Smart Pause
			pause, err := db.GetPauseConfig()
			if err != nil {
				log.Printf("Error reading pause config: %v", err)
			}
			if pause && IsUserActive() {
				log.Println("[Smart Pause] Active user session detected. Skipping checks...")
				continue
			}
			checkAllServices()
		case <-stopChan:
			log.Println("Stopping monitoring loop")
			return
		}
	}
}

func Stop() {
	close(stopChan)
}

func checkAllServices() {
	services, err := db.ListServices()
	if err != nil {
		log.Printf("Error listing services: %v", err)
		return
	}

	for _, s := range services {
		if !s.Enabled {
			continue
		}
		go checkAndRestart(s)
	}
}

func checkAndRestart(s db.Service) {
	// Execute Check Command
	// We assume a non-zero exit code means failure -> Restart needed.
	// For 'systemctl is-failed', user should use '! systemctl is-failed <service>' so that:
	// - Not Failed (Active/Inactive) -> is-failed returns 1 -> ! makes it 0 (OK)
	// - Failed -> is-failed returns 0 -> ! makes it 1 (FAIL)
	err := runCommand(s.CheckCommand)

	db.UpdateLastChecked(s.ID)

	if err != nil {
		log.Printf("[Monitor] Service %s check failed (cmd: %s). Restarting...", s.Name, s.CheckCommand)
		restartErr := runCommand(s.RestartCommand)
		if restartErr != nil {
			log.Printf("[Monitor] Failed to restart service %s: %v", s.Name, restartErr)
		} else {
			log.Printf("[Monitor] Successfully restarted service %s", s.Name)
			db.UpdateLastRestarted(s.ID)
		}
	} else {
		// Log verbose if needed, but keeping quiet for success is better for logs
		// log.Printf("[Monitor] Service %s is healthy.", s.Name)
	}
}

func runCommand(cmdStr string) error {
	// Use sh -c to allow shell features (pipes, redirection, negation !)
	cmd := exec.Command("sh", "-c", cmdStr)
	return cmd.Run()
}

// IsUserActive checks if any user is logged in using the 'who' command
func IsUserActive() bool {
	cmd := exec.Command("who")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error checking active users: %v", err)
		return false // Assume no user if check fails, to be safe? Or fail open? Safe is assume no user -> monitor.
	}
	// If output is not empty, someone is logged in
	return len(output) > 0
}
