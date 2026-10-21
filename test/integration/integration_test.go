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
	if os.Getenv("PLAYGROUND_INTEGRATION_ALLOW_RESET") != "1" {
		t.Fatal("integration tests reset their DATABASE_URL; set PLAYGROUND_INTEGRATION_ALLOW_RESET=1 for a disposable database")
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

// resetBusinessTables は組織を参照する業務データを、外部キーの依存順に削除する。
func resetBusinessTables(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DELETE FROM ticket_comments; DELETE FROM ticket_histories; DELETE FROM tickets; DELETE FROM tasks; DELETE FROM invoices; DELETE FROM deals; DELETE FROM customers; DELETE FROM organization_members; DELETE FROM organizations`); err != nil {
		t.Fatal(err)
	}
}

func requirePGCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected PostgreSQL error %s", want)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || (pgErr.Code != want && !(want == "23503" && pgErr.Code == "23001")) {
		t.Fatalf("error=%v, want PostgreSQL error %s", err, want)
	}
}

func TestMigrationRoundTripUsesSchemaSQL(t *testing.T) {
	db := openDB(t)
	m := migrateDB(t, db)
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatal(err)
	}
	resetBusinessTables(t, db)
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"organizations", "organization_members", "customers", "deals", "invoices", "tasks", "tickets", "ticket_histories", "ticket_comments"} {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)`, table).Scan(&exists); err != nil || !exists {
			t.Fatalf("table %s was not recreated: %v", table, err)
		}
	}
}

func TestOrganizationIsolationAndCompositeForeignKey(t *testing.T) {
	db := openDB(t)
	ensureUp(t, db)
	resetBusinessTables(t, db)
	ctx := context.Background()
	organizationA, organizationB := uuid.New(), uuid.New()
	customerA, customerB := uuid.New(), uuid.New()
	for _, organization := range []struct {
		id   uuid.UUID
		user string
	}{{organizationA, "user-a"}, {organizationB, "user-b"}} {
		if _, err := db.ExecContext(ctx, `INSERT INTO organizations (id, name, created_at, updated_at) VALUES ($1,$2,now(),now())`, organization.id, organization.user); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO organization_members (organization_id, user_id, role, created_at, updated_at) VALUES ($1,$2,'admin',now(),now())`, organization.id, organization.user); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []struct {
		id   uuid.UUID
		org  uuid.UUID
		user string
	}{{customerA, organizationA, "user-a"}, {customerB, organizationB, "user-b"}} {
		_, err := db.ExecContext(ctx, `INSERT INTO customers (id, organization_id, owner_user_id, name, email, phone, status, created_at, updated_at) VALUES ($1,$2,$3,'name',$4,'1','active',now(),now())`, v.id, v.org, v.user, v.user+"@example.com")
		if err != nil {
			t.Fatal(err)
		}
	}
	var visible int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM customers WHERE organization_id=$1`, organizationA).Scan(&visible); err != nil || visible != 1 {
		t.Fatalf("organization isolation count=%d err=%v", visible, err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO deals (id, organization_id, owner_user_id, customer_id, title, amount, status, created_at, updated_at) VALUES ($1,$2,'user-a',$3,'cross',1,'open',now(),now())`, uuid.New(), organizationA, customerB); err == nil {
		t.Fatal("cross-organization customer foreign key was accepted")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO invoices (id, organization_id, owner_user_id, customer_id, number, amount, issue_date, due_date, status, created_at, updated_at) VALUES ($1,$2,'user-a',$3,'I-1',1,'2026-01-01','2026-01-02','draft',now(),now())`, uuid.New(), organizationA, customerB); err == nil {
		t.Fatal("cross-organization invoice foreign key was accepted")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO deals (id, organization_id, owner_user_id, customer_id, title, amount, status, created_at, updated_at) VALUES ($1,$2,'user-a',$3,'owned',1,'open',now(),now())`, uuid.New(), organizationA, customerA); err != nil {
		t.Fatal(err)
	}
	_, err := db.ExecContext(ctx, `DELETE FROM customers WHERE id=$1 AND organization_id=$2`, customerA, organizationA)
	requirePGCode(t, err, "23001")
}

func TestInvoiceOwnerUniqueAndLockedTransition(t *testing.T) {
	db := openDB(t)
	ensureUp(t, db)
	resetBusinessTables(t, db)
	organizationID := uuid.New()
	if _, err := db.Exec(`INSERT INTO organizations (id, name, created_at, updated_at) VALUES ($1,'user-a',now(),now())`, organizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role, created_at, updated_at) VALUES ($1,'user-a','admin',now(),now())`, organizationID); err != nil {
		t.Fatal(err)
	}
	customerID := uuid.New()
	if _, err := db.Exec(`INSERT INTO customers (id, organization_id, owner_user_id, name, email, phone, status, created_at, updated_at) VALUES ($1,$2,'user-a','name','a@example.com','1','active',now(),now())`, customerID, organizationID); err != nil {
		t.Fatal(err)
	}
	invoiceID := uuid.New()
	if _, err := db.Exec(`INSERT INTO invoices (id, organization_id, owner_user_id, customer_id, number, amount, issue_date, due_date, status, created_at, updated_at) VALUES ($1,$2,'user-a',$3,'I-1',1,'2026-01-01','2026-01-02','draft',now(),now())`, invoiceID, organizationID, customerID); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(`INSERT INTO invoices (id, organization_id, owner_user_id, customer_id, number, amount, issue_date, due_date, status, created_at, updated_at) VALUES ($1,$2,'user-a',$3,'I-1',1,'2026-01-01','2026-01-02','draft',now(),now())`, uuid.New(), organizationID, customerID)
	requirePGCode(t, err, "23505")
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for _, next := range []string{"sent", "sent"} {
		wg.Add(1)
		go func(next string) {
			defer wg.Done()
			tx, err := db.Begin()
			if err != nil {
				results <- false
				return
			}
			var current string
			err = tx.QueryRow(`SELECT status FROM invoices WHERE id=$1 AND organization_id=$2 FOR UPDATE`, invoiceID, organizationID).Scan(&current)
			if err == nil && next == "sent" && current == "draft" {
				_, err = tx.Exec(`UPDATE invoices SET status=$1, updated_at=now() WHERE id=$2 AND organization_id=$3`, next, invoiceID, organizationID)
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
	_, err = db.Exec(`DELETE FROM customers WHERE id=$1 AND organization_id=$2`, customerID, organizationID)
	requirePGCode(t, err, "23001")
}
