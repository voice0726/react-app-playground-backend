//go:build integration

package integration

import (
	"errors"
	"testing"

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
