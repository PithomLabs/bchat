package main

import (
	"context"
	"fmt"
	"log"

	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
	"github.com/usememos/memos/store/db"
)

func main() {
	p := &profile.Profile{
		Mode:    "dev",
		Port:    5230,
		Data:    "/home/chaschel/Documents/go/bchat/build/data",
		Driver:  "cockroach",
		DSN:     "postgresql://root@localhost:26257/bchat?sslmode=disable&default_query_exec_mode=simple_protocol",
		Version: "0.35.0",
	}
	driver, err := db.NewDBDriver(p)
	if err != nil {
		log.Fatal(err)
	}
	defer driver.Close()
	s := store.New(driver, p)
	ctx := context.Background()
	user, err := s.GetUser(ctx, &store.FindUser{Username: stringPtr("admin")})
	if err != nil {
		log.Fatal(err)
	}
	if user == nil {
		fmt.Println("User NOT FOUND")
	} else {
		fmt.Printf("Found user: ID=%d, Username=%s, Role=%s, Email=%s\n", user.ID, user.Username, user.Role, user.Email)
	}
}

func stringPtr(s string) *string {
	return &s
}