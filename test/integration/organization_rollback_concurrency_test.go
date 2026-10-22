//go:build integration

package integration

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOrganizationRollbackBlocksConcurrentWrites(t *testing.T) {
	db, organizationID, downSQL := prepareOrganizationRollback(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	reader, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Rollback()
	if _, err := reader.ExecContext(ctx, `LOCK TABLE deals IN ACCESS SHARE MODE`); err != nil {
		t.Fatal(err)
	}

	down, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		_ = down.Rollback()
	}()
	var pid int
	if err := down.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := down.ExecContext(ctx, downSQL)
		done <- err
	}()
	waitForOrganizationRollbackLock(t, ctx, db, pid)

	writer, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.ExecContext(ctx, `SET LOCAL lock_timeout = '250ms'`); err != nil {
		t.Fatal(err)
	}
	_, err = writer.ExecContext(ctx, `INSERT INTO organization_members (organization_id, user_id, role, created_at, updated_at) VALUES ($1,'concurrent-editor','editor',now(),now())`, organizationID)
	requirePGCode(t, err, "55P03")
	if err := writer.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := reader.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	// SQL の終了後も、トランザクションの完了までは書き込みを防ぐ必要があります。
	for _, table := range []string{"organizations", "organization_members", "customers", "deals", "invoices", "tasks"} {
		t.Run(table, func(t *testing.T) {
			probe, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer probe.Rollback()
			if _, err := probe.ExecContext(ctx, `SET LOCAL lock_timeout = '250ms'`); err != nil {
				t.Fatal(err)
			}
			_, err = probe.ExecContext(ctx, `DELETE FROM `+table+` WHERE false`)
			requirePGCode(t, err, "55P03")
		})
	}
	if err := down.Commit(); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name IN ('customers','deals','invoices','tasks') AND column_name='organization_id'`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("remaining organization columns=%d err=%v", remaining, err)
	}
	m := migrateDB(t, db)
	if err := m.Force(2); err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
}

func TestOrganizationRollbackChecksCommittedConcurrentMembership(t *testing.T) {
	db, organizationID, downSQL := prepareOrganizationRollback(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	writer, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.ExecContext(ctx, `INSERT INTO organization_members (organization_id, user_id, role, created_at, updated_at) VALUES ($1,'concurrent-editor','editor',now(),now())`, organizationID); err != nil {
		t.Fatal(err)
	}
	down, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		_ = down.Rollback()
	}()
	var pid int
	if err := down.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := down.ExecContext(ctx, downSQL)
		done <- err
	}()
	waitForOrganizationRollbackLock(t, ctx, db, pid)
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil || !strings.Contains(err.Error(), "shared membership would be lost") {
		t.Fatalf("rollback must reject committed shared membership: %v", err)
	}
	if err := down.Rollback(); err != nil {
		t.Fatal(err)
	}
	var members int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM organization_members WHERE organization_id=$1`, organizationID).Scan(&members); err != nil || members != 2 {
		t.Fatalf("preserved members=%d err=%v", members, err)
	}
}

func prepareOrganizationRollback(t *testing.T) (*sql.DB, string, string) {
	t.Helper()
	db := openDB(t)
	ensureUp(t, db)
	resetBusinessTables(t, db)
	// 000003 の down SQL を直接流すため、tickets が organizations を参照する 000004 を先に戻す。
	if err := migrateDB(t, db).Migrate(3); err != nil {
		t.Fatal(err)
	}
	var organizationID string
	if err := db.QueryRow(`INSERT INTO organizations (id, name, created_at, updated_at) VALUES (md5('react-app-playground:organization:rollback-owner')::uuid,'rollback-owner',now(),now()) RETURNING id::text`).Scan(&organizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role, created_at, updated_at) VALUES ($1,'rollback-owner','admin',now(),now())`, organizationID); err != nil {
		t.Fatal(err)
	}
	downSQL, err := os.ReadFile("../../migrations/000003_organizations.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	return db, organizationID, string(downSQL)
}

func waitForOrganizationRollbackLock(t *testing.T, ctx context.Context, db *sql.DB, pid int) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='relation' AND NOT granted)`, pid).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("rollback did not wait for a table lock: ", ctx.Err())
		case <-ticker.C:
		}
	}
}
