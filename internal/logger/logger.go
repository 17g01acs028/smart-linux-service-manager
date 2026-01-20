package logger

import (
	"fmt"
	"linux_service_manager/internal/db"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

func Init(logFile string) {
	if logFile == "" {
		// Default to stdout
		log.SetOutput(os.Stdout)
		return
	}

	// Ensure directory exists
	// We assume logFile contains a path
	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log dir: %v\n", err)
	}

	// Load config
	cfg, err := db.GetLogConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load log config: %v. Using defaults.\n", err)
		cfg = &db.LogConfig{
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		}
	}

	// Setup Lumberjack
	lj := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    cfg.MaxSize, // megabytes
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,   // days
		Compress:   cfg.Compress, // disabled by default
	}

	// Multiwriter to write to both stdout and file?
	// Usually daemons write to file only, but for debugging stdout is nice.
	// Let's stick to file mostly, or maybe both.
	// For this task, let's write to file only if daemonized, but we can't easily tell.
	// Let's us configured file.

	log.SetOutput(lj)
	log.Printf("Logger initialized. File: %s, MaxSize: %dMB, MaxBackups: %d, MaxAge: %d days",
		logFile, cfg.MaxSize, cfg.MaxBackups, cfg.MaxAge)
}
