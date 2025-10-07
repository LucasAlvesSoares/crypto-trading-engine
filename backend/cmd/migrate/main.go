package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/crypto-trading-bot/internal/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

func main() {
	// Parse command line flags
	flag.String("direction", "up", "Migration direction: up or down")
	steps := flag.Int("steps", 0, "Number of migrations to apply (0 = all)")
	flag.Parse()

	// Get remaining args
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: migrate [up|down] [-steps N]")
		os.Exit(1)
	}

	command := args[0]

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	db, err := sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create migrate driver
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatalf("Failed to create migrate driver: %v", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres",
		driver,
	)
	if err != nil {
		log.Fatalf("Failed to create migrate instance: %v", err)
	}

	// Run migration
	switch command {
	case "up":
		fmt.Println("Running migrations up...")
		if *steps > 0 {
			err = m.Steps(*steps)
		} else {
			err = m.Up()
		}
	case "down":
		fmt.Println("Running migrations down...")
		if *steps > 0 {
			err = m.Steps(-*steps)
		} else {
			err = m.Down()
		}
	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("Failed to get version: %v", err)
		}
		fmt.Printf("Version: %d, Dirty: %v\n", version, dirty)
		return
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Available commands: up, down, version")
		os.Exit(1)
	}

	if err != nil {
		if err == migrate.ErrNoChange {
			fmt.Println("No changes to apply")
		} else {
			log.Fatalf("Migration failed: %v", err)
		}
	} else {
		fmt.Println("Migration completed successfully!")
	}
}
