CREATE TABLE organizations (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);

CREATE TABLE organization_members (
  organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  user_id text NOT NULL CHECK (length(user_id) > 0 AND length(user_id) <= 200),
  role text NOT NULL CHECK (role IN ('admin', 'editor', 'viewer')),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  PRIMARY KEY (organization_id, user_id)
);
CREATE INDEX organization_members_user_idx ON organization_members (user_id, organization_id);

CREATE TEMP TABLE legacy_organization_map (
  owner_user_id text PRIMARY KEY,
  organization_id uuid NOT NULL UNIQUE
) ON COMMIT DROP;

INSERT INTO legacy_organization_map (owner_user_id, organization_id)
SELECT owner_user_id, md5('react-app-playground:organization:' || owner_user_id)::uuid
FROM (
  SELECT owner_user_id FROM customers
  UNION
  SELECT owner_user_id FROM deals
  UNION
  SELECT owner_user_id FROM invoices
  UNION
  SELECT owner_user_id FROM tasks
) owners;

INSERT INTO organizations (id, name, created_at, updated_at)
SELECT organization_id, owner_user_id, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM legacy_organization_map;

INSERT INTO organization_members (organization_id, user_id, role, created_at, updated_at)
SELECT organization_id, owner_user_id, 'admin', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM legacy_organization_map;

ALTER TABLE customers ADD COLUMN organization_id uuid;
ALTER TABLE deals ADD COLUMN organization_id uuid;
ALTER TABLE invoices ADD COLUMN organization_id uuid;
ALTER TABLE tasks ADD COLUMN organization_id uuid;

UPDATE customers AS resource
SET organization_id = legacy.organization_id
FROM legacy_organization_map AS legacy
WHERE legacy.owner_user_id = resource.owner_user_id;
UPDATE deals AS resource
SET organization_id = legacy.organization_id
FROM legacy_organization_map AS legacy
WHERE legacy.owner_user_id = resource.owner_user_id;
UPDATE invoices AS resource
SET organization_id = legacy.organization_id
FROM legacy_organization_map AS legacy
WHERE legacy.owner_user_id = resource.owner_user_id;
UPDATE tasks AS resource
SET organization_id = legacy.organization_id
FROM legacy_organization_map AS legacy
WHERE legacy.owner_user_id = resource.owner_user_id;

ALTER TABLE customers ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE deals ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE invoices ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE tasks ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE deals DROP CONSTRAINT IF EXISTS deals_owner_user_id_customer_id_fkey;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_owner_user_id_customer_id_fkey;

ALTER TABLE customers DROP CONSTRAINT IF EXISTS customers_owner_user_id_id_key;
ALTER TABLE deals DROP CONSTRAINT IF EXISTS deals_owner_user_id_id_key;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_owner_user_id_id_key;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_owner_user_id_number_key;

ALTER TABLE customers ADD CONSTRAINT customers_organization_id_id_key UNIQUE (organization_id, id);
ALTER TABLE deals ADD CONSTRAINT deals_organization_id_id_key UNIQUE (organization_id, id);
ALTER TABLE invoices ADD CONSTRAINT invoices_organization_id_id_key UNIQUE (organization_id, id);
ALTER TABLE invoices ADD CONSTRAINT invoices_organization_id_number_key UNIQUE (organization_id, number);

ALTER TABLE customers ADD CONSTRAINT customers_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE RESTRICT;
ALTER TABLE deals ADD CONSTRAINT deals_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE RESTRICT;
ALTER TABLE invoices ADD CONSTRAINT invoices_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE RESTRICT;
ALTER TABLE tasks ADD CONSTRAINT tasks_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE RESTRICT;
ALTER TABLE deals ADD CONSTRAINT deals_organization_customer_fkey FOREIGN KEY (organization_id, customer_id) REFERENCES customers (organization_id, id) ON DELETE RESTRICT;
ALTER TABLE invoices ADD CONSTRAINT invoices_organization_customer_fkey FOREIGN KEY (organization_id, customer_id) REFERENCES customers (organization_id, id) ON DELETE RESTRICT;

DROP INDEX IF EXISTS customers_owner_created_idx;
DROP INDEX IF EXISTS deals_owner_created_idx;
DROP INDEX IF EXISTS invoices_owner_created_idx;
DROP INDEX IF EXISTS tasks_owner_created_idx;
CREATE INDEX customers_organization_created_idx ON customers (organization_id, created_at, id);
CREATE INDEX deals_organization_created_idx ON deals (organization_id, created_at, id);
CREATE INDEX invoices_organization_created_idx ON invoices (organization_id, created_at, id);
CREATE INDEX tasks_organization_created_idx ON tasks (organization_id, created_at, id);
