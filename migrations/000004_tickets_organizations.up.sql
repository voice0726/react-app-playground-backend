CREATE TEMP TABLE legacy_ticket_organization_map (
  owner_user_id text PRIMARY KEY,
  organization_id uuid NOT NULL UNIQUE
) ON COMMIT DROP;

INSERT INTO legacy_ticket_organization_map (owner_user_id, organization_id)
SELECT DISTINCT owner_user_id, md5('react-app-playground:organization:' || owner_user_id)::uuid
FROM tickets;

-- 000003 で作成済みの個別組織はそのまま使う。メンバーを追加するのは、ここで新たに作成した組織だけにする。
WITH created AS (
  INSERT INTO organizations (id, name, created_at, updated_at)
  SELECT organization_id, owner_user_id, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
  FROM legacy_ticket_organization_map
  ON CONFLICT (id) DO NOTHING
  RETURNING id, name
)
INSERT INTO organization_members (organization_id, user_id, role, created_at, updated_at)
SELECT id, name, 'admin', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM created;

ALTER TABLE tickets ADD COLUMN organization_id uuid;
ALTER TABLE ticket_histories ADD COLUMN organization_id uuid;
ALTER TABLE ticket_comments ADD COLUMN organization_id uuid;

UPDATE tickets AS resource
SET organization_id = legacy.organization_id
FROM legacy_ticket_organization_map AS legacy
WHERE legacy.owner_user_id = resource.owner_user_id;
UPDATE ticket_histories AS child
SET organization_id = ticket.organization_id
FROM tickets AS ticket
WHERE ticket.id = child.ticket_id;
UPDATE ticket_comments AS child
SET organization_id = ticket.organization_id
FROM tickets AS ticket
WHERE ticket.id = child.ticket_id;

ALTER TABLE tickets ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE ticket_histories ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE ticket_comments ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE ticket_histories DROP CONSTRAINT ticket_histories_owner_user_id_ticket_id_fkey;
ALTER TABLE ticket_comments DROP CONSTRAINT ticket_comments_owner_user_id_ticket_id_fkey;
ALTER TABLE tickets DROP CONSTRAINT tickets_owner_user_id_id_key;
DROP INDEX tickets_owner_created_idx;
DROP INDEX ticket_histories_ticket_idx;
DROP INDEX ticket_comments_ticket_idx;
ALTER TABLE ticket_histories DROP COLUMN owner_user_id;
ALTER TABLE ticket_comments DROP COLUMN owner_user_id;

ALTER TABLE tickets ADD CONSTRAINT tickets_organization_id_id_key UNIQUE (organization_id, id);
ALTER TABLE tickets ADD CONSTRAINT tickets_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE RESTRICT;
ALTER TABLE ticket_histories ADD CONSTRAINT ticket_histories_organization_ticket_fkey FOREIGN KEY (organization_id, ticket_id) REFERENCES tickets (organization_id, id) ON DELETE CASCADE;
ALTER TABLE ticket_comments ADD CONSTRAINT ticket_comments_organization_ticket_fkey FOREIGN KEY (organization_id, ticket_id) REFERENCES tickets (organization_id, id) ON DELETE CASCADE;

CREATE INDEX tickets_organization_created_idx ON tickets (organization_id, created_at, id);
CREATE INDEX ticket_histories_ticket_idx ON ticket_histories (organization_id, ticket_id, changed_at, id);
CREATE INDEX ticket_comments_ticket_idx ON ticket_comments (organization_id, ticket_id, created_at, id);
