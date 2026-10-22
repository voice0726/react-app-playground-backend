//go:build integration

package integration

import (
	"errors"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
)

func TestOrganizationMigrationPreservesLegacyDataAndBlocksUnsafeRollback(t *testing.T) {
	db := openDB(t)
	m := migrateDB(t, db)
	ensureUp(t, db)
	if _, err := db.Exec(`DELETE FROM tasks; DELETE FROM invoices; DELETE FROM deals; DELETE FROM customers; DELETE FROM organization_members; DELETE FROM organizations`); err != nil {
		t.Fatal(err)
	}
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}

	owner := "legacy-owner"
	customerID, dealID, invoiceID, taskID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO customers (id, owner_user_id, name, email, phone, status, created_at, updated_at) VALUES ($1,$2,'legacy','legacy@example.com','1','active',now(),now())`, customerID, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO deals (id, owner_user_id, customer_id, title, amount, status, created_at, updated_at) VALUES ($1,$2,$3,'legacy deal',100,'open',now(),now())`, dealID, owner, customerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO invoices (id, owner_user_id, customer_id, number, amount, issue_date, due_date, status, created_at, updated_at) VALUES ($1,$2,$3,'LEGACY-1',100,'2026-01-01','2026-01-02','sent',now(),now())`, invoiceID, owner, customerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO tasks (id, owner_user_id, title, description, status, created_at, updated_at) VALUES ($1,$2,'legacy task','legacy','TODO',now(),now())`, taskID, owner); err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}

	var organizationID uuid.UUID
	if err := db.QueryRow(`SELECT id FROM organizations WHERE name=$1`, owner).Scan(&organizationID); err != nil {
		t.Fatal(err)
	}
	var memberRole string
	if err := db.QueryRow(`SELECT role FROM organization_members WHERE organization_id=$1 AND user_id=$2`, organizationID, owner).Scan(&memberRole); err != nil || memberRole != "admin" {
		t.Fatalf("legacy admin role=%q err=%v", memberRole, err)
	}
	for _, item := range []struct {
		table string
		id    uuid.UUID
	}{
		{table: "customers", id: customerID},
		{table: "deals", id: dealID},
		{table: "invoices", id: invoiceID},
		{table: "tasks", id: taskID},
	} {
		var actual uuid.UUID
		if err := db.QueryRow(`SELECT organization_id FROM `+item.table+` WHERE id=$1`, item.id).Scan(&actual); err != nil || actual != organizationID {
			t.Fatalf("%s organization=%s want=%s err=%v", item.table, actual, organizationID, err)
		}
	}

	newOrganizationID := uuid.New()
	if _, err := db.Exec(`INSERT INTO organizations (id, name, created_at, updated_at) VALUES ($1,'New organization',now(),now())`, newOrganizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role, created_at, updated_at) VALUES ($1,'new-user','admin',now(),now())`, newOrganizationID); err != nil {
		t.Fatal(err)
	}
	if err := m.Steps(-1); err == nil {
		t.Fatal("rollback of a newly-created organization unexpectedly succeeded")
	}
	version, dirty, err := m.Version()
	if err != nil || version != 2 || !dirty {
		t.Fatalf("failed new organization rollback version=%d dirty=%t err=%v", version, dirty, err)
	}
	if err := m.Force(3); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM organization_members WHERE organization_id=$1`, newOrganizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM organizations WHERE id=$1`, newOrganizationID); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role, created_at, updated_at) VALUES ($1,'shared-user','viewer',now(),now())`, organizationID); err != nil {
		t.Fatal(err)
	}
	if err := m.Steps(-1); err == nil {
		t.Fatal("unsafe rollback unexpectedly succeeded")
	}
	version, dirty, err = m.Version()
	if err != nil || version != 2 || !dirty {
		t.Fatalf("failed rollback version=%d dirty=%t err=%v", version, dirty, err)
	}
	if err := m.Force(3); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM tasks; DELETE FROM invoices; DELETE FROM deals; DELETE FROM customers; DELETE FROM organization_members; DELETE FROM organizations`); err != nil {
		t.Fatal(err)
	}
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatal(err)
	}
}
