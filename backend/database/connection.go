package database

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

// Connect establishes connection to PostgreSQL database
func Connect() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Println("DATABASE_URL not set, using in-memory storage")
		return nil
	}

	var err error
	DB, err = sql.Open("postgres", databaseURL)
	if err != nil {
		return err
	}

	// Configure connection pool
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * time.Minute)

	// Test connection with retries
	for i := 0; i < 10; i++ {
		err = DB.Ping()
		if err == nil {
			log.Println("Connected to PostgreSQL database")
			return nil
		}
		log.Printf("Waiting for database... (attempt %d/10)", i+1)
		time.Sleep(2 * time.Second)
	}

	return err
}

// Close closes the database connection
func Close() {
	if DB != nil {
		DB.Close()
	}
}

// IsConnected returns true if database is connected
func IsConnected() bool {
	return DB != nil
}
