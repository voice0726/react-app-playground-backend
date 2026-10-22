//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
)

func TestTicketOrganizationMigrationPreservesLegacyTicketsAndBlocksUnsafeRollback(t *testing.T) {
	db := openDB(t)
	m := migrateDB(t, db)
	ensureUp(t, db)
	resetBusinessTables(t, db)
	if err := m.Migrate(2); err != nil {
		t.Fatal(err)
	}

	// customer を持つ所有者は 000003 で組織が作られ、tickets だけの所有者は 000004 で組織が作られる。
	sharedOwner, ticketOnlyOwner := "customer-owner", "ticket-only-owner"
	if _, err := db.Exec(`INSERT INTO customers (id, owner_user_id, name, email, phone, status, created_at, updated_at) VALUES ($1,$2,'legacy','legacy@example.com','1','active',now(),now())`, uuid.New(), sharedOwner); err != nil {
		t.Fatal(err)
	}
	ticketIDs := map[string]uint64{}
	for _, owner := range []string{sharedOwner, ticketOnlyOwner} {
		var id uint64
		if err := db.QueryRow(`INSERT INTO tickets (owner_user_id, title, status, created_by_id, created_by_name, created_by_email, updated_by_id, updated_by_name, updated_by_email, created_at, updated_at) VALUES ($1,'legacy','closed',$1,$1,$1,$1,$1,$1,now(),now()) RETURNING id`, owner).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO ticket_comments (owner_user_id, ticket_id, body, created_by_id, created_by_name, created_by_email, created_at) VALUES ($1,$2,'legacy',$1,$1,$1,now())`, owner, id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO ticket_histories (owner_user_id, ticket_id, operation_id, field, before_value, after_value, changed_by_id, changed_by_name, changed_by_email, changed_at) VALUES ($1,$2,'op','status','open','closed',$1,$1,$1,now())`, owner, id); err != nil {
			t.Fatal(err)
		}
		ticketIDs[owner] = id
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}

	for _, owner := range []string{sharedOwner, ticketOnlyOwner} {
		var organizationID uuid.UUID
		var organizations int
		if err := db.QueryRow(`SELECT min(id::text)::uuid, count(*) FROM organizations WHERE name=$1`, owner).Scan(&organizationID, &organizations); err != nil || organizations != 1 {
			t.Fatalf("%s organizations=%d err=%v", owner, organizations, err)
		}
		var role string
		if err := db.QueryRow(`SELECT role FROM organization_members WHERE organization_id=$1 AND user_id=$2`, organizationID, owner).Scan(&role); err != nil || role != "admin" {
			t.Fatalf("%s role=%q err=%v", owner, role, err)
		}
		for _, table := range []string{"tickets", "ticket_comments", "ticket_histories"} {
			column := "ticket_id"
			if table == "tickets" {
				column = "id"
			}
			var actual uuid.UUID
			if err := db.QueryRow(`SELECT organization_id FROM `+table+` WHERE `+column+`=$1`, ticketIDs[owner]).Scan(&actual); err != nil || actual != organizationID {
				t.Fatalf("%s %s organization=%s want=%s err=%v", owner, table, actual, organizationID, err)
			}
		}
	}

	var ticketOnlyOrganizationID uuid.UUID
	if err := db.QueryRow(`SELECT organization_id FROM tickets WHERE id=$1`, ticketIDs[ticketOnlyOwner]).Scan(&ticketOnlyOrganizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role, created_at, updated_at) VALUES ($1,'shared-user','viewer',now(),now())`, ticketOnlyOrganizationID); err != nil {
		t.Fatal(err)
	}
	if err := m.Steps(-1); err == nil {
		t.Fatal("rollback of shared tickets unexpectedly succeeded")
	}
	version, dirty, err := m.Version()
	if err != nil || version != 3 || !dirty {
		t.Fatalf("failed rollback version=%d dirty=%t err=%v", version, dirty, err)
	}
	if err := m.Force(4); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM organization_members WHERE organization_id=$1 AND user_id='shared-user'`, ticketOnlyOrganizationID); err != nil {
		t.Fatal(err)
	}

	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	var owner string
	if err := db.QueryRow(`SELECT owner_user_id FROM ticket_comments WHERE ticket_id=$1`, ticketIDs[ticketOnlyOwner]).Scan(&owner); err != nil || owner != ticketOnlyOwner {
		t.Fatalf("restored comment owner=%q err=%v", owner, err)
	}
	_, err = db.Exec(`INSERT INTO ticket_comments (owner_user_id, ticket_id, body, created_by_id, created_by_name, created_by_email, created_at) VALUES ($1,$2,'x','x','x','x',now())`, sharedOwner, ticketIDs[ticketOnlyOwner])
	requirePGCode(t, err, "23503")

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatal(err)
	}
	var mapped uuid.UUID
	if err := db.QueryRow(`SELECT organization_id FROM tickets WHERE id=$1`, ticketIDs[ticketOnlyOwner]).Scan(&mapped); err != nil || mapped != ticketOnlyOrganizationID {
		t.Fatalf("re-applied organization=%s want=%s err=%v", mapped, ticketOnlyOrganizationID, err)
	}
	resetBusinessTables(t, db)
}

func TestTicketOrganizationMigrationKeepsExistingOrganizationMembership(t *testing.T) {
	db := openDB(t)
	m := migrateDB(t, db)
	ensureUp(t, db)
	resetBusinessTables(t, db)
	if err := m.Migrate(2); err != nil {
		t.Fatal(err)
	}

	owner := "removed-owner"
	if _, err := db.Exec(`INSERT INTO customers (id, owner_user_id, name, email, phone, status, created_at, updated_at) VALUES ($1,$2,'legacy','legacy@example.com','1','active',now(),now())`, uuid.New(), owner); err != nil {
		t.Fatal(err)
	}
	if err := m.Migrate(3); err != nil {
		t.Fatal(err)
	}
	// 000003 で作られた個別組織から所有者を外した後に、000004 が所有者の権限を戻さないことを確かめる。
	var organizationID uuid.UUID
	if err := db.QueryRow(`SELECT md5('react-app-playground:organization:' || $1::text)::uuid`, owner).Scan(&organizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role, created_at, updated_at) VALUES ($1,'successor-admin','admin',now(),now())`, organizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM organization_members WHERE organization_id=$1 AND user_id=$2`, organizationID, owner); err != nil {
		t.Fatal(err)
	}
	var ticketID uint64
	if err := db.QueryRow(`INSERT INTO tickets (owner_user_id, title, status, created_by_id, created_by_name, created_by_email, updated_by_id, updated_by_name, updated_by_email, created_at, updated_at) VALUES ($1,'legacy','open',$1,$1,$1,$1,$1,$1,now(),now()) RETURNING id`, owner).Scan(&ticketID); err != nil {
		t.Fatal(err)
	}

	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	var mapped uuid.UUID
	if err := db.QueryRow(`SELECT organization_id FROM tickets WHERE id=$1`, ticketID).Scan(&mapped); err != nil || mapped != organizationID {
		t.Fatalf("ticket organization=%s want=%s err=%v", mapped, organizationID, err)
	}
	rows, err := db.Query(`SELECT user_id || ':' || role FROM organization_members WHERE organization_id=$1 ORDER BY user_id`, organizationID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var members []string
	for rows.Next() {
		var member string
		if err := rows.Scan(&member); err != nil {
			t.Fatal(err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0] != "successor-admin:admin" {
		t.Fatalf("organization members=%v, want only successor-admin:admin", members)
	}
	resetBusinessTables(t, db)
}

func TestTicketOrganizationRollbackBlocksConcurrentSharing(t *testing.T) {
	db := openDB(t)
	m := migrateDB(t, db)
	ensureUp(t, db)
	resetBusinessTables(t, db)
	if err := m.Migrate(2); err != nil {
		t.Fatal(err)
	}
	owner := "rollback-owner"
	if _, err := db.Exec(`INSERT INTO tickets (owner_user_id, title, status, created_by_id, created_by_name, created_by_email, updated_by_id, updated_by_name, updated_by_email, created_at, updated_at) VALUES ($1,'legacy','open',$1,$1,$1,$1,$1,$1,now(),now())`, owner); err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	var ownerOrganizationID uuid.UUID
	if err := db.QueryRow(`SELECT organization_id FROM tickets WHERE owner_user_id=$1`, owner).Scan(&ownerOrganizationID); err != nil {
		t.Fatal(err)
	}
	sharedOrganizationID := uuid.New()
	if _, err := db.Exec(`INSERT INTO organizations (id, name, created_at, updated_at) VALUES ($1,'shared',now(),now())`, sharedOrganizationID); err != nil {
		t.Fatal(err)
	}

	// ticket_histories のロックで down を安全確認の後に止め、その間の共有操作が待たされることを確かめる。
	ctx := context.Background()
	blocker, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()
	if _, err := blocker.ExecContext(ctx, `LOCK TABLE ticket_histories IN ACCESS SHARE MODE`); err != nil {
		t.Fatal(err)
	}
	rollback := make(chan error, 1)
	go func() { rollback <- m.Steps(-1) }()
	waitForUngrantedLock(t, db, "ticket_histories")

	err = execWithLockTimeout(ctx, db, `INSERT INTO organization_members (organization_id, user_id, role, created_at, updated_at) VALUES ($1,'editor-user','editor',now(),now())`, ownerOrganizationID)
	requirePGCode(t, err, "55P03")
	err = execWithLockTimeout(ctx, db, `INSERT INTO tickets (organization_id, owner_user_id, title, status, created_by_id, created_by_name, created_by_email, updated_by_id, updated_by_name, updated_by_email, created_at, updated_at) VALUES ($1,'shared-user','shared','open','a','a','a','a','a','a',now(),now())`, sharedOrganizationID)
	requirePGCode(t, err, "55P03")

	if err := blocker.Rollback(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-rollback:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("rollback did not finish after the blocker was released")
	}
	version, dirty, err := m.Version()
	if err != nil || version != 3 || dirty {
		t.Fatalf("rollback version=%d dirty=%t err=%v", version, dirty, err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	resetBusinessTables(t, db)
}

func waitForUngrantedLock(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_locks WHERE relation = to_regclass($1) AND NOT granted)`, table).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no session waited for a lock on %s", table)
}

func execWithLockTimeout(ctx context.Context, db *sql.DB, query string, args ...any) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '500ms'`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return err
	}
	return tx.Commit()
}
