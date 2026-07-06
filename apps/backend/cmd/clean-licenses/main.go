// Made by YTSworks
// YTS工作室製作
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	dbPathPtr := flag.String("db", "./data/nms.db", "Path to SQLite database")
	flag.Parse()

	dbPath := *dbPathPtr
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("Database not found at %s", dbPath)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Printf("Cleaning licenses from database: %s\n", dbPath)

	// Clear licenses table
	if _, err := db.Exec("DELETE FROM licenses"); err != nil {
		log.Fatalf("Failed to delete licenses: %v", err)
	}
	fmt.Println("- DELETE FROM licenses [OK]")

	// Reset trial status
	if _, err := db.Exec("UPDATE system_config SET config_value = 'false' WHERE config_key = 'trial_activated'"); err != nil {
		log.Printf("Warning: Failed to reset trial status: %v", err)
	} else {
		fmt.Println("- Reset 'trial_activated' to false [OK]")
	}

	// Clear alerts settings to defaults just in case? No, user only asked for licenses.

	// Vacuum to reclaim space
	if _, err := db.Exec("VACUUM"); err != nil {
		log.Printf("Warning: VACUUM failed: %v", err)
	} else {
		fmt.Println("- VACUUM [OK]")
	}

	fmt.Println("\nAll licenses removed. Database is clean for fresh testing.")
}
