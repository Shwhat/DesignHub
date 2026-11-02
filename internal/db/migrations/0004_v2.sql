-- Version 2: companies and company-scoped accounts, custom roles, editable
-- workflows, sprints, milestones, time tracking, templates, inline and
-- internal comments, CSV imports. DATA-MODEL.md marks these (v2).

-- Custom roles
ALTER TABLE roles ADD COLUMN copied_from uuid REFERENCES roles (id);
ALTER TABLE roles ADD COLUMN company_scope text NOT NULL DEFAULT '' CHECK (company_scope IN ('', 'vendor', 'contractor'));
ALTER TABLE roles ADD COLUMN active boolean NOT NULL DEFAULT true;

-- Companies
CREATE TABLE companies (
    id                 text PRIMARY KEY,
    name               text NOT NULL,
    kinds              text[] NOT NULL DEFAULT '{}',
    supplier_status    text NOT NULL DEFAULT '' CHECK (supplier_status IN ('', 'Approved', 'Conditional', 'Not approved')),
    manufacturer_codes text[] NOT NULL DEFAULT '{}',
    address            text NOT NULL DEFAULT '',
    phone              text NOT NULL DEFAULT '',
    website            text NOT NULL DEFAULT '',
    contacts           jsonb NOT NULL DEFAULT '[]',
    notes              text NOT NULL DEFAULT '',
    labels             text[] NOT NULL DEFAULT '{}',
    attachments        jsonb NOT NULL DEFAULT '[]',
    active             boolean NOT NULL DEFAULT true,
    created_by         uuid REFERENCES users (id),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    version            integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX companies_name ON companies (lower(name));

CREATE TABLE company_history (
    id         bigserial PRIMARY KEY,
    company_id text NOT NULL,
    at         timestamptz NOT NULL DEFAULT now(),
    user_id    uuid,
    changes    jsonb NOT NULL
);
CREATE INDEX company_history_company ON company_history (company_id, at);

-- Accounts linked to a company, and expiry
ALTER TABLE users ADD COLUMN company_id text REFERENCES companies (id);
ALTER TABLE users ADD COLUMN expires_on date;
ALTER TABLE users ADD COLUMN expiry_notices text[] NOT NULL DEFAULT '{}';

-- Supplier documents: files a Vendor adds to an item or NCR, kept apart
-- from revision files.
CREATE TABLE supplier_documents (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    record_id   text NOT NULL,
    company_id  text NOT NULL REFERENCES companies (id),
    sha256      text NOT NULL REFERENCES files (sha256),
    name        text NOT NULL,
    uploaded_by uuid NOT NULL REFERENCES users (id),
    uploaded_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX supplier_documents_record ON supplier_documents (record_id);

-- Comments: internal (hidden from Vendors) and inline anchors
ALTER TABLE comments ADD COLUMN internal boolean NOT NULL DEFAULT false;
ALTER TABLE comments ADD COLUMN anchor jsonb;

-- Editable workflows
CREATE TABLE workflows (
    type       text PRIMARY KEY CHECK (type IN ('task', 'issue', 'action', 'epic', 'ncr')),
    version    integer NOT NULL,
    definition jsonb NOT NULL,
    changed_by uuid REFERENCES users (id),
    changed_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE workflow_versions (
    type       text NOT NULL,
    version    integer NOT NULL,
    definition jsonb NOT NULL,
    changed_by uuid REFERENCES users (id),
    changed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (type, version)
);

-- Work item planning fields. state_cat is the workflow category of the
-- current status: open, active, done or cancelled.
ALTER TABLE work_items ADD COLUMN estimate numeric(7, 2);
ALTER TABLE work_items ADD COLUMN sprint_id text NOT NULL DEFAULT '';
ALTER TABLE work_items ADD COLUMN milestone_id text NOT NULL DEFAULT '';
ALTER TABLE work_items ADD COLUMN state_cat text NOT NULL DEFAULT 'open';
UPDATE work_items SET state_cat = CASE
    WHEN status IN ('Done', 'Closed') THEN 'done'
    WHEN status = 'Cancelled' THEN 'cancelled'
    WHEN status IN ('Open', 'Reported') THEN 'open'
    ELSE 'active' END;
CREATE INDEX work_items_sprint ON work_items (sprint_id) WHERE sprint_id <> '';
CREATE INDEX work_items_milestone ON work_items (milestone_id) WHERE milestone_id <> '';

CREATE TABLE sprints (
    id          text PRIMARY KEY,
    space_id    text NOT NULL REFERENCES spaces (id),
    name        text NOT NULL,
    goal        text NOT NULL DEFAULT '',
    start_date  date NOT NULL,
    end_date    date NOT NULL,
    state       text NOT NULL DEFAULT 'Planned' CHECK (state IN ('Planned', 'Active', 'Closed')),
    committed   jsonb NOT NULL DEFAULT '{}',
    completed   jsonb NOT NULL DEFAULT '{}',
    created_by  uuid REFERENCES users (id),
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX sprints_name ON sprints (space_id, lower(name));
CREATE UNIQUE INDEX sprints_one_active ON sprints (space_id) WHERE state = 'Active';

CREATE TABLE milestones (
    id           text PRIMARY KEY,
    space_id     text NOT NULL REFERENCES spaces (id),
    name         text NOT NULL,
    target_date  date NOT NULL,
    description  text NOT NULL DEFAULT '',
    owner_id     uuid NOT NULL REFERENCES users (id),
    completed_on date,
    risk_noticed boolean NOT NULL DEFAULT false,
    created_by   uuid REFERENCES users (id),
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE time_entries (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id    text NOT NULL REFERENCES work_items (id),
    user_id    uuid NOT NULL REFERENCES users (id),
    day        date NOT NULL,
    hours      numeric(5, 2) NOT NULL CHECK (hours >= 0.25 AND hours <= 24),
    note       text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    edited_at  timestamptz
);
CREATE INDEX time_entries_item ON time_entries (item_id);
CREATE INDEX time_entries_user_day ON time_entries (user_id, day);

-- Page templates
CREATE TABLE templates (
    id            text PRIMARY KEY,
    name          text NOT NULL,
    space_id      text NOT NULL DEFAULT '',
    description   text NOT NULL DEFAULT '',
    page_type     text NOT NULL,
    title_pattern text NOT NULL DEFAULT '',
    body          text NOT NULL DEFAULT '',
    active        boolean NOT NULL DEFAULT true,
    changed_by    uuid REFERENCES users (id),
    changed_at    timestamptz NOT NULL DEFAULT now(),
    version       integer NOT NULL DEFAULT 1
);
CREATE TABLE template_versions (
    template_id text NOT NULL,
    version     integer NOT NULL,
    body        text NOT NULL,
    changed_by  uuid REFERENCES users (id),
    changed_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (template_id, version)
);

-- CSV imports
CREATE TABLE csv_imports (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    record_type text NOT NULL,
    target      text NOT NULL DEFAULT '',
    mode        text NOT NULL,
    file_sha256 text NOT NULL REFERENCES files (sha256),
    file_name   text NOT NULL,
    rows        integer NOT NULL,
    created     integer NOT NULL DEFAULT 0,
    updated     integer NOT NULL DEFAULT 0,
    imported_by uuid NOT NULL REFERENCES users (id),
    imported_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO counters (name, next) VALUES ('COMP', 1), ('MS', 1), ('SPR', 1), ('TPL', 1)
    ON CONFLICT (name) DO NOTHING;
