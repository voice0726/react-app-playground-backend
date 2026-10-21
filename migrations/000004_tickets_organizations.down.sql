DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM tickets
    WHERE organization_id <> md5('react-app-playground:organization:' || owner_user_id)::uuid
  ) THEN
    RAISE EXCEPTION 'cannot roll back ticket organization migration: ticket organization would be lost';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM tickets ticket
    JOIN organization_members member ON member.organization_id = ticket.organization_id
    WHERE member.user_id <> ticket.owner_user_id
  ) THEN
    RAISE EXCEPTION 'cannot roll back ticket organization migration: shared ticket access would be lost';
  END IF;
END $$;

ALTER TABLE ticket_histories ADD COLUMN owner_user_id text;
ALTER TABLE ticket_comments ADD COLUMN owner_user_id text;
UPDATE ticket_histories AS child
SET owner_user_id = ticket.owner_user_id
FROM tickets AS ticket
WHERE ticket.id = child.ticket_id;
UPDATE ticket_comments AS child
SET owner_user_id = ticket.owner_user_id
FROM tickets AS ticket
WHERE ticket.id = child.ticket_id;
ALTER TABLE ticket_histories ALTER COLUMN owner_user_id SET NOT NULL;
ALTER TABLE ticket_comments ALTER COLUMN owner_user_id SET NOT NULL;

ALTER TABLE ticket_histories DROP CONSTRAINT ticket_histories_organization_ticket_fkey;
ALTER TABLE ticket_comments DROP CONSTRAINT ticket_comments_organization_ticket_fkey;
ALTER TABLE tickets DROP CONSTRAINT tickets_organization_fkey;
ALTER TABLE tickets DROP CONSTRAINT tickets_organization_id_id_key;
DROP INDEX tickets_organization_created_idx;
DROP INDEX ticket_histories_ticket_idx;
DROP INDEX ticket_comments_ticket_idx;
ALTER TABLE ticket_histories DROP COLUMN organization_id;
ALTER TABLE ticket_comments DROP COLUMN organization_id;
ALTER TABLE tickets DROP COLUMN organization_id;

ALTER TABLE tickets ADD CONSTRAINT tickets_owner_user_id_id_key UNIQUE (owner_user_id, id);
ALTER TABLE ticket_histories ADD CONSTRAINT ticket_histories_owner_user_id_ticket_id_fkey FOREIGN KEY (owner_user_id, ticket_id) REFERENCES tickets (owner_user_id, id) ON DELETE CASCADE;
ALTER TABLE ticket_comments ADD CONSTRAINT ticket_comments_owner_user_id_ticket_id_fkey FOREIGN KEY (owner_user_id, ticket_id) REFERENCES tickets (owner_user_id, id) ON DELETE CASCADE;
CREATE INDEX tickets_owner_created_idx ON tickets (owner_user_id, created_at, id);
CREATE INDEX ticket_histories_ticket_idx ON ticket_histories (owner_user_id, ticket_id, changed_at, id);
CREATE INDEX ticket_comments_ticket_idx ON ticket_comments (owner_user_id, ticket_id, created_at, id);
