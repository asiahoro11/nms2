package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./reset_admin <new_password>")
		os.Exit(1)
	}

	newPassword := os.Args[1]
	dbPath := "./data/nms.db"

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Update admin user
	res, err := db.Exec("UPDATE users SET password_hash = ? WHERE username = 'admin'", string(hash))
	if err != nil {
		log.Fatalf("Failed to update password: %v", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		fmt.Println("User 'admin' not found!")
	} else {
		fmt.Printf("Successfully reset password for 'admin' to '%s'\n", newPassword)
	}
}
