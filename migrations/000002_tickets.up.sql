CREATE TABLE tickets (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  owner_user_id text NOT NULL,
  title text NOT NULL,
  status text NOT NULL CHECK (status IN ('open', 'closed')),
  assignee text,
  created_by_id text NOT NULL,
  created_by_name text NOT NULL,
  created_by_email text NOT NULL,
  updated_by_id text NOT NULL,
  updated_by_name text NOT NULL,
  updated_by_email text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  UNIQUE (owner_user_id, id)
);
CREATE INDEX tickets_owner_created_idx ON tickets (owner_user_id, created_at, id);
CREATE TABLE ticket_histories (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  owner_user_id text NOT NULL,
  ticket_id bigint NOT NULL,
  operation_id text NOT NULL,
  field text NOT NULL,
  before_value text,
  after_value text,
  changed_by_id text NOT NULL,
  changed_by_name text NOT NULL,
  changed_by_email text NOT NULL,
  changed_at timestamptz NOT NULL,
  FOREIGN KEY (owner_user_id, ticket_id) REFERENCES tickets (owner_user_id, id) ON DELETE CASCADE
);
CREATE INDEX ticket_histories_ticket_idx ON ticket_histories (owner_user_id, ticket_id, changed_at, id);
CREATE TABLE ticket_comments (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  owner_user_id text NOT NULL,
  ticket_id bigint NOT NULL,
  body text NOT NULL,
  created_by_id text NOT NULL,
  created_by_name text NOT NULL,
  created_by_email text NOT NULL,
  created_at timestamptz NOT NULL,
  FOREIGN KEY (owner_user_id, ticket_id) REFERENCES tickets (owner_user_id, id) ON DELETE CASCADE
);
CREATE INDEX ticket_comments_ticket_idx ON ticket_comments (owner_user_id, ticket_id, created_at, id);
