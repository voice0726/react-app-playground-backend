-- 安全確認からスキーマ変更の完了まで、確認対象への書き込みを防ぎます。
LOCK TABLE organizations, organization_members, customers, deals, invoices, tasks IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM organizations) AND EXISTS (
    SELECT 1
    FROM organizations organization
    LEFT JOIN organization_members member ON member.organization_id = organization.id
    GROUP BY organization.id
    HAVING COUNT(member.user_id) = 0
  ) THEN
    RAISE EXCEPTION 'cannot roll back organization migration: organization membership would be lost';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM organization_members
    GROUP BY organization_id
    HAVING COUNT(*) <> 1
  ) THEN
    RAISE EXCEPTION 'cannot roll back organization migration: shared membership would be lost';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM organizations organization
    JOIN organization_members member ON member.organization_id = organization.id
    WHERE member.role <> 'admin'
      OR organization.id <> md5('react-app-playground:organization:' || member.user_id)::uuid
      OR organization.name <> member.user_id
  ) THEN
    RAISE EXCEPTION 'cannot roll back organization migration: organization ownership metadata would be lost';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM organization_members member
    JOIN customers resource ON resource.organization_id = member.organization_id
    WHERE resource.owner_user_id <> member.user_id
  ) OR EXISTS (
    SELECT 1
    FROM organization_members member
    JOIN deals resource ON resource.organization_id = member.organization_id
    WHERE resource.owner_user_id <> member.user_id
  ) OR EXISTS (
    SELECT 1
    FROM organization_members member
    JOIN invoices resource ON resource.organization_id = member.organization_id
    WHERE resource.owner_user_id <> member.user_id
  ) OR EXISTS (
    SELECT 1
    FROM organization_members member
    JOIN tasks resource ON resource.organization_id = member.organization_id
    WHERE resource.owner_user_id <> member.user_id
  ) THEN
    RAISE EXCEPTION 'cannot roll back organization migration: shared resource ownership would be lost';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM deals deal
    JOIN customers customer ON customer.id = deal.customer_id
    WHERE customer.organization_id <> deal.organization_id
  ) OR EXISTS (
    SELECT 1
    FROM invoices invoice
    JOIN customers customer ON customer.id = invoice.customer_id
    WHERE customer.organization_id <> invoice.organization_id
  ) THEN
    RAISE EXCEPTION 'cannot roll back organization migration: cross-organization references would be lost';
  END IF;
END $$;

ALTER TABLE deals DROP CONSTRAINT IF EXISTS deals_organization_customer_fkey;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_organization_customer_fkey;
ALTER TABLE customers DROP CONSTRAINT IF EXISTS customers_organization_fkey;
ALTER TABLE deals DROP CONSTRAINT IF EXISTS deals_organization_fkey;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_organization_fkey;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_organization_fkey;

ALTER TABLE customers DROP CONSTRAINT IF EXISTS customers_organization_id_id_key;
ALTER TABLE deals DROP CONSTRAINT IF EXISTS deals_organization_id_id_key;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_organization_id_id_key;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_organization_id_number_key;

DROP INDEX IF EXISTS customers_organization_created_idx;
DROP INDEX IF EXISTS deals_organization_created_idx;
DROP INDEX IF EXISTS invoices_organization_created_idx;
DROP INDEX IF EXISTS tasks_organization_created_idx;
CREATE INDEX customers_owner_created_idx ON customers (owner_user_id, created_at, id);
CREATE INDEX deals_owner_created_idx ON deals (owner_user_id, created_at, id);
CREATE INDEX invoices_owner_created_idx ON invoices (owner_user_id, created_at, id);
CREATE INDEX tasks_owner_created_idx ON tasks (owner_user_id, created_at, id);

ALTER TABLE customers DROP COLUMN organization_id;
ALTER TABLE deals DROP COLUMN organization_id;
ALTER TABLE invoices DROP COLUMN organization_id;
ALTER TABLE tasks DROP COLUMN organization_id;

ALTER TABLE customers ADD CONSTRAINT customers_owner_user_id_id_key UNIQUE (owner_user_id, id);
ALTER TABLE deals ADD CONSTRAINT deals_owner_user_id_id_key UNIQUE (owner_user_id, id);
ALTER TABLE invoices ADD CONSTRAINT invoices_owner_user_id_id_key UNIQUE (owner_user_id, id);
ALTER TABLE invoices ADD CONSTRAINT invoices_owner_user_id_number_key UNIQUE (owner_user_id, number);
ALTER TABLE deals ADD CONSTRAINT deals_owner_user_id_customer_id_fkey FOREIGN KEY (owner_user_id, customer_id) REFERENCES customers (owner_user_id, id) ON DELETE RESTRICT;
ALTER TABLE invoices ADD CONSTRAINT invoices_owner_user_id_customer_id_fkey FOREIGN KEY (owner_user_id, customer_id) REFERENCES customers (owner_user_id, id) ON DELETE RESTRICT;

DROP TABLE organization_members;
DROP TABLE organizations;
