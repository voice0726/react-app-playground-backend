//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Fatal("DATABASE_URL is required for integration tests (start backend PostgreSQL first)")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("DATABASE_URL is unavailable: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func migrateDB(t *testing.T, db *sql.DB) *migrate.Migrate {
	t.Helper()
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://../../migrations", "playground", driver)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = m.Close() })
	return m
}

func ensureUp(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := migrateDB(t, db).Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatal(err)
	}
}

func requirePGCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected PostgreSQL error %s", want)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != want {
		t.Fatalf("error=%v, want PostgreSQL error %s", err, want)
	}
}

func TestMigrationRoundTripUsesSchemaSQL(t *testing.T) {
	db := openDB(t)
	m := migrateDB(t, db)
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatal(err)
	}
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"customers", "deals", "invoices", "tasks", "tickets", "ticket_histories", "ticket_comments"} {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)`, table).Scan(&exists); err != nil || !exists {
			t.Fatalf("table %s was not recreated: %v", table, err)
		}
	}
}

func TestCrossUserIsolationAndCompositeForeignKey(t *testing.T) {
	db := openDB(t)
	ensureUp(t, db)
	_, _ = db.Exec(`DELETE FROM tasks; DELETE FROM invoices; DELETE FROM deals; DELETE FROM customers`)
	ctx := context.Background()
	customerA, customerB := uuid.New(), uuid.New()
	for _, v := range []struct {
		id   uuid.UUID
		user string
	}{{customerA, "user-a"}, {customerB, "user-b"}} {
		_, err := db.ExecContext(ctx, `INSERT INTO customers (id, owner_user_id, name, email, phone, status, created_at, updated_at) VALUES ($1,$2,'name',$3,'1','active',now(),now())`, v.id, v.user, v.user+"@example.com")
		if err != nil {
			t.Fatal(err)
		}
	}
	var visible int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM customers WHERE owner_user_id=$1`, "user-a").Scan(&visible); err != nil || visible != 1 {
		t.Fatalf("user isolation count=%d err=%v", visible, err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO deals (id, owner_user_id, customer_id, title, amount, status, created_at, updated_at) VALUES ($1,'user-a',$2,'cross',1,'open',now(),now())`, uuid.New(), customerB); err == nil {
		t.Fatal("cross-user customer foreign key was accepted")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO invoices (id, owner_user_id, customer_id, number, amount, issue_date, due_date, status, created_at, updated_at) VALUES ($1,'user-a',$2,'I-1',1,'2026-01-01','2026-01-02','draft',now(),now())`, uuid.New(), customerB); err == nil {
		t.Fatal("cross-user invoice foreign key was accepted")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO deals (id, owner_user_id, customer_id, title, amount, status, created_at, updated_at) VALUES ($1,'user-a',$2,'owned',1,'open',now(),now())`, uuid.New(), customerA); err != nil {
		t.Fatal(err)
	}
	_, err := db.ExecContext(ctx, `DELETE FROM customers WHERE id=$1 AND owner_user_id='user-a'`, customerA)
	requirePGCode(t, err, "23001")
}

func TestInvoiceOwnerUniqueAndLockedTransition(t *testing.T) {
	db := openDB(t)
	ensureUp(t, db)
	_, _ = db.Exec(`DELETE FROM invoices; DELETE FROM customers`)
	customerID := uuid.New()
	if _, err := db.Exec(`INSERT INTO customers (id, owner_user_id, name, email, phone, status, created_at, updated_at) VALUES ($1,'user-a','name','a@example.com','1','active',now(),now())`, customerID); err != nil {
		t.Fatal(err)
	}
	invoiceID := uuid.New()
	if _, err := db.Exec(`INSERT INTO invoices (id, owner_user_id, customer_id, number, amount, issue_date, due_date, status, created_at, updated_at) VALUES ($1,'user-a',$2,'I-1',1,'2026-01-01','2026-01-02','draft',now(),now())`, invoiceID, customerID); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(`INSERT INTO invoices (id, owner_user_id, customer_id, number, amount, issue_date, due_date, status, created_at, updated_at) VALUES ($1,'user-a',$2,'I-1',1,'2026-01-01','2026-01-02','draft',now(),now())`, uuid.New(), customerID)
	requirePGCode(t, err, "23505")
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for _, next := range []string{"sent", "paid"} {
		wg.Add(1)
		go func(next string) {
			defer wg.Done()
			tx, err := db.Begin()
			if err != nil {
				results <- false
				return
			}
			var current string
			err = tx.QueryRow(`SELECT status FROM invoices WHERE id=$1 AND owner_user_id='user-a' FOR UPDATE`, invoiceID).Scan(&current)
			if err == nil && ((next == "sent" && current == "draft") || (next == "paid" && current == "sent")) {
				_, err = tx.Exec(`UPDATE invoices SET status=$1, updated_at=now() WHERE id=$2 AND owner_user_id='user-a'`, next, invoiceID)
			} else {
				err = errors.New("invalid transition")
			}
			if err == nil {
				err = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
			results <- err == nil
		}(next)
	}
	wg.Wait()
	close(results)
	succeeded := 0
	for ok := range results {
		if ok {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("locked transition successes=%d, want 1", succeeded)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM invoices WHERE id=$1`, invoiceID).Scan(&status); err != nil || status != "sent" {
		t.Fatalf("final invoice status=%q err=%v", status, err)
	}
	_, err = db.Exec(`DELETE FROM customers WHERE id=$1 AND owner_user_id='user-a'`, customerID)
	requirePGCode(t, err, "23001")
}
