CREATE TABLE customers (
  id uuid PRIMARY KEY,
  owner_user_id text NOT NULL,
  name text NOT NULL,
  email text NOT NULL,
  phone text NOT NULL,
  status text NOT NULL CHECK (status IN ('active', 'inactive')),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  UNIQUE (owner_user_id, id)
);
CREATE INDEX customers_owner_created_idx ON customers (owner_user_id, created_at, id);
CREATE TABLE deals (
  id uuid PRIMARY KEY,
  owner_user_id text NOT NULL,
  customer_id uuid NOT NULL,
  title text NOT NULL,
  amount bigint NOT NULL CHECK (amount >= 0),
  status text NOT NULL CHECK (status IN ('open', 'won', 'lost')),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  UNIQUE (owner_user_id, id),
  FOREIGN KEY (owner_user_id, customer_id) REFERENCES customers (owner_user_id, id) ON DELETE RESTRICT
);
CREATE INDEX deals_owner_created_idx ON deals (owner_user_id, created_at, id);
CREATE TABLE invoices (
  id uuid PRIMARY KEY,
  owner_user_id text NOT NULL,
  customer_id uuid NOT NULL,
  number text NOT NULL,
  amount bigint NOT NULL CHECK (amount >= 0),
  issue_date date NOT NULL,
  due_date date NOT NULL CHECK (due_date >= issue_date),
  status text NOT NULL CHECK (status IN ('draft', 'sent', 'paid', 'cancelled')),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  UNIQUE (owner_user_id, id),
  UNIQUE (owner_user_id, number),
  FOREIGN KEY (owner_user_id, customer_id) REFERENCES customers (owner_user_id, id) ON DELETE RESTRICT
);
CREATE INDEX invoices_owner_created_idx ON invoices (owner_user_id, created_at, id);
CREATE TABLE tasks (
  id uuid PRIMARY KEY,
  owner_user_id text NOT NULL,
  title text NOT NULL,
  description text NOT NULL,
  status text NOT NULL CHECK (status IN ('TODO', 'IN_PROGRESS', 'DONE')),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  UNIQUE (owner_user_id, id)
);
CREATE INDEX tasks_owner_created_idx ON tasks (owner_user_id, created_at, id);
