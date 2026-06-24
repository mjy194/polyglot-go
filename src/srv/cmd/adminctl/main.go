// Command adminctl is a small ops tool for the admin user store:
//
//	go run ./cmd/adminctl list
//	go run ./cmd/adminctl reset <email-or-id> <new-password>
//
// It opens the same SQLite store the server uses (config path resolved the same
// way as the server) and lets you recover access when the admin password is lost.
package main

import (
	"context"
	"fmt"
	"os"

	"polyglot/internal/authn"
	"polyglot/internal/data"
	"polyglot/internal/domain"
)

func main() {
	store, err := data.Open(data.Config{
		Driver:      data.DriverSQLite,
		DSN:         dbPath(),
		AutoMigrate: false,
	})
	if err != nil {
		fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "list":
		users, err := store.Identity().ListUsers(ctx)
		if err != nil {
			fatalf("list users: %v", err)
		}
		fmt.Printf("%-24s %-32s %s\n", "ID", "EMAIL", "STATUS")
		for _, u := range users {
			fmt.Printf("%-24s %-32s %s\n", u.ID, u.Email, u.Status)
		}
		if len(users) == 0 {
			fmt.Println("(no users — bootstrap via POST /api/admin/bootstrap)")
		}
	case "reset":
		if len(os.Args) < 4 {
			usage()
		}
		ident, newpw := os.Args[2], os.Args[3]
		if err := resetPassword(ctx, store, ident, newpw); err != nil {
			fatalf("%v", err)
		}
	default:
		usage()
	}
}

func resetPassword(ctx context.Context, store *data.Store, ident, newpw string) error {
	user, found, err := store.Identity().GetUserByEmail(ctx, ident)
	if err != nil {
		return err
	}
	if !found {
		users, _ := store.Identity().ListUsers(ctx)
		for _, u := range users {
			if u.ID == ident {
				user, found = u, true
				break
			}
		}
	}
	if !found {
		return fmt.Errorf("no user matching %q", ident)
	}
	hash, err := authn.HashPassword(newpw)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	user.Status = domain.StatusActive
	if _, err := store.Identity().UpsertUser(ctx, user); err != nil {
		return err
	}
	fmt.Printf("✅ Reset password for %s (%s). Login with that password now.\n", user.Email, user.ID)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: adminctl {list | reset <email-or-id> <new-password>}")
	os.Exit(2)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "adminctl: "+format+"\n", args...)
	os.Exit(1)
}

// dbPath resolves the SQLite path the same way the server defaults to.
func dbPath() string {
	if p := os.Getenv("POLYGLOT_DB"); p != "" {
		return p
	}
	return "data/data.db"
}
