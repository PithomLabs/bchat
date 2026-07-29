package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	fmt.Println("=== Demo Ticket Seeding Script ===")
	fmt.Println("This script seeds demo tickets for the CockroachDB hackathon demo.")
	fmt.Println("")

	// Check required environment variables
	dsn := os.Getenv("COCKROACH_DSN")
	if dsn == "" {
		fmt.Println("ERROR: COCKROACH_DSN environment variable is required")
		fmt.Println("")
		fmt.Println("Usage:")
		fmt.Println("  export COCKROACH_DSN='postgresql://user:pass@host/db?sslmode=require'")
		fmt.Println("  go run cmd/seed/seed_demo_tickets.go")
		os.Exit(1)
	}

	// Connect to database
	fmt.Println("Connecting to CockroachDB...")
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("Connected successfully!")

	// Get or create demo tenant
	fmt.Println("\nLooking for demo tenant...")
	var tenantID int32
	err = db.QueryRowContext(ctx, `
		SELECT id FROM agent_tenants WHERE slug = 'hackathon-demo' LIMIT 1
	`).Scan(&tenantID)

	if err == sql.ErrNoRows {
		// Create demo tenant
		fmt.Println("Creating demo tenant...")
		err = db.QueryRowContext(ctx, `
			INSERT INTO agent_tenants (slug, company_name, vertical, is_active)
			VALUES ('hackathon-demo', 'Hackathon Demo', 'restoration', true)
			RETURNING id
		`).Scan(&tenantID)
		if err != nil {
			log.Fatalf("Failed to create tenant: %v", err)
		}
		fmt.Printf("Created tenant with ID: %d\n", tenantID)
	} else if err != nil {
		log.Fatalf("Failed to query tenant: %v", err)
	} else {
		fmt.Printf("Found existing tenant with ID: %d\n", tenantID)
	}

	// Create demo tickets
	fmt.Println("\nCreating demo tickets...")
	tickets := []struct {
		Title       string
		Description string
		Priority    string
		Tags        []string
	}{
		{
			Title:       "Water damage in bathroom",
			Description: "Customer reports water damage from leaky pipe. Bathroom floor is warped and mold is starting to form.",
			Priority:    "HIGH",
			Tags:        []string{"water-damage", "emergency", "bathroom"},
		},
		{
			Title:       "Fire damage restoration",
			Description: "Kitchen fire caused significant damage to cabinets and appliances. Need full restoration service.",
			Priority:    "HIGH",
			Tags:        []string{"fire-damage", "restoration", "kitchen"},
		},
		{
			Title:       "Mold remediation",
			Description: "Mold found in basement after flooding. Need professional remediation service.",
			Priority:    "HIGH",
			Tags:        []string{"mold", "remediation", "basement"},
		},
		{
			Title:       "Storm damage repair",
			Description: "Roof damage from recent storm. Water leaking into attic and bedrooms.",
			Priority:    "MEDIUM",
			Tags:        []string{"storm-damage", "roof", "repair"},
		},
		{
			Title:       "Carpet cleaning service",
			Description: "Customer needs professional carpet cleaning for entire house after pet damage.",
			Priority:    "LOW",
			Tags:        []string{"cleaning", "carpet", "pet-damage"},
		},
		{
			Title:       "Drywall repair",
			Description: "Hole in drywall from moving furniture. Need patching and painting.",
			Priority:    "LOW",
			Tags:        []string{"drywall", "repair", "painting"},
		},
		{
			Title:       "Emergency board-up",
			Description: "Broken window from break-in. Need emergency board-up service.",
			Priority:    "HIGH",
			Tags:        []string{"emergency", "board-up", "security"},
		},
		{
			Title:       "Air duct cleaning",
			Description: "Customer wants air duct cleaning service for improved air quality.",
			Priority:    "LOW",
			Tags:        []string{"cleaning", "air-ducts", "maintenance"},
		},
		{
			Title:       "Water extraction",
			Description: "Basement flooded after heavy rain. Need immediate water extraction service.",
			Priority:    "HIGH",
			Tags:        []string{"water-damage", "extraction", "emergency"},
		},
		{
			Title:       "Contents packing",
			Description: "Customer moving out for restoration. Need contents packing and storage service.",
			Priority:    "MEDIUM",
			Tags:        []string{"packing", "storage", "moving"},
		},
	}

	created := 0
	for _, ticketData := range tickets {
		// Check if ticket already exists
		var exists bool
		err = db.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM tickets WHERE title = $1 AND tenant_id = $2)
		`, ticketData.Title, tenantID).Scan(&exists)
		if err != nil {
			log.Printf("Warning: Failed to check ticket existence: %v", err)
			continue
		}

		if exists {
			fmt.Printf("  Skipped (exists): %s\n", ticketData.Title)
			continue
		}

		// Create ticket
		_, err = db.ExecContext(ctx, `
			INSERT INTO tickets (title, description, status, priority, tenant_id, tags)
			VALUES ($1, $2, 'OPEN', $3, $4, $5)
		`, ticketData.Title, "/m/"+ticketData.Description, ticketData.Priority, tenantID, ticketData.Tags)
		if err != nil {
			log.Printf("Warning: Failed to create ticket %q: %v", ticketData.Title, err)
			continue
		}

		fmt.Printf("  Created: %s\n", ticketData.Title)
		created++
		time.Sleep(100 * time.Millisecond) // Small delay
	}

	fmt.Printf("\n=== Seeding Complete ===\n")
	fmt.Printf("Created %d new tickets (tenant ID: %d)\n", created, tenantID)
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("1. Start the application with ticket embedding enabled:")
	fmt.Println("   export TICKET_EMBEDDING_ENABLED=true")
	fmt.Println("   task run:cockroach")
	fmt.Println("")
	fmt.Println("2. Wait for the cron job to run (every 5 minutes)")
	fmt.Println("   or trigger manually via API endpoint")
	fmt.Println("")
	fmt.Println("3. Test ticket escalation:")
	fmt.Println("   curl -X POST http://localhost:8081/api/v1/agent/hackathon-demo/escalate \\")
	fmt.Println("     -H 'Content-Type: application/json' \\")
	fmt.Println("     -d '{\"title\": \"Water damage in bathroom\", \"description\": \"Customer reports water damage from leaky pipe\", \"priority\": \"high\"}'")
}
