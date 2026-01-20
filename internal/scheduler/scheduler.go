package scheduler

import (
	"linux_service_manager/internal/db"
	"log"
	"os/exec"

	"github.com/robfig/cron/v3"
)

var c *cron.Cron

func Start() {
	c = cron.New()

	err := loadJobs()
	if err != nil {
		log.Printf("[Scheduler] Failed to load jobs: %v", err)
	}

	c.Start()
	log.Println("[Scheduler] Started cron scheduler")
}

func Stop() {
	if c != nil {
		c.Stop()
	}
}

func loadJobs() error {
	services, err := db.ListServices()
	if err != nil {
		return err
	}

	for _, s := range services {
		// If schedule is empty, we don't schedule it (Monitor loop still checks it)
		if s.CronSchedule == "" || !s.Enabled {
			continue
		}

		// Capture variable for closure
		svc := s

		_, err := c.AddFunc(svc.CronSchedule, func() {
			safeRestart(svc)
		})
		if err != nil {
			log.Printf("[Scheduler] Failed to schedule service %s with schedule '%s': %v", svc.Name, svc.CronSchedule, err)
		} else {
			log.Printf("[Scheduler] Scheduled restart for %s at '%s'", svc.Name, svc.CronSchedule)
		}
	}
	return nil
}

func safeRestart(s db.Service) {
	log.Printf("[Scheduler] Triggered scheduled restart for %s", s.Name)

	// Safe Check: Only restart if running
	if s.StatusCommand != "" {
		err := runCommand(s.StatusCommand)
		if err != nil {
			log.Printf("[Scheduler] Skipping restart for %s: Status check failed (not running?)", s.Name)
			return
		}
	} else {
		log.Printf("[Scheduler] Warning: No status_command for %s. Restarting blindly.", s.Name)
	}

	// Restart
	err := runCommand(s.RestartCommand)
	if err != nil {
		log.Printf("[Scheduler] Failed to restart %s: %v", s.Name, err)
	} else {
		log.Printf("[Scheduler] Successfully restarted %s", s.Name)
		db.UpdateLastRestarted(s.ID)
	}
}

func runCommand(cmdStr string) error {
	cmd := exec.Command("sh", "-c", cmdStr)
	return cmd.Run()
}
