-- DesignHub version 1 schema.
-- Engineering items and pages live in git; the tables marked "index" are
-- derived from git and can be rebuilt at any time.

CREATE TABLE audit_log (
    id         bigserial PRIMARY KEY,
    at         timestamptz NOT NULL DEFAULT now(),
    user_id    uuid,
    username   text NOT NULL DEFAULT '',
    action     text NOT NULL,
    record_id  text NOT NULL DEFAULT '',
    detail     jsonb NOT NULL DEFAULT '{}',
    ip         text NOT NULL DEFAULT ''
);
CREATE INDEX audit_log_at ON audit_log (at DESC);
CREATE INDEX audit_log_record ON audit_log (record_id) WHERE record_id <> '';

CREATE TABLE settings (
    key        text PRIMARY KEY,
    value      jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid
);

-- People and access ---------------------------------------------------------

CREATE TABLE roles (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name                 text NOT NULL UNIQUE,
    built_in             boolean NOT NULL DEFAULT false,
    applies_to           text NOT NULL CHECK (applies_to IN ('library', 'space', 'both')),
    permissions          text[] NOT NULL DEFAULT '{}',
    can_manage_approvers boolean NOT NULL DEFAULT false,
    sort_order           integer NOT NULL DEFAULT 0
);

CREATE TABLE users (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username          text NOT NULL,
    display_name      text NOT NULL,
    email             text NOT NULL DEFAULT '',
    password_hash     text NOT NULL DEFAULT '',
    sign_in_method    text NOT NULL DEFAULT 'local' CHECK (sign_in_method IN ('local', 'google')),
    library_role_id   uuid NOT NULL REFERENCES roles (id),
    default_edit_mode text NOT NULL DEFAULT 'visual' CHECK (default_edit_mode IN ('visual', 'split', 'source', 'preview')),
    time_zone         text NOT NULL DEFAULT '',
    theme             text NOT NULL DEFAULT 'device' CHECK (theme IN ('light', 'dark', 'device')),
    home_layout       jsonb NOT NULL DEFAULT '[]',
    notify_events     jsonb NOT NULL DEFAULT '{}',
    active            boolean NOT NULL DEFAULT true,
    created_at        timestamptz NOT NULL DEFAULT now(),
    last_sign_in      timestamptz
);
CREATE UNIQUE INDEX users_username ON users (lower(username));

CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    csrf_token text NOT NULL,
    keep       boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen  timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    user_agent text NOT NULL DEFAULT '',
    ip         text NOT NULL DEFAULT ''
);
CREATE INDEX sessions_user ON sessions (user_id);

CREATE TABLE spaces (
    id                  text PRIMARY KEY,
    name                text NOT NULL,
    description         text NOT NULL DEFAULT '',
    home_page_id        text NOT NULL DEFAULT '',
    archived            boolean NOT NULL DEFAULT false,
    controlled_defaults jsonb NOT NULL DEFAULT '{}',
    pinned_links        jsonb NOT NULL DEFAULT '[]',
    created_by          uuid REFERENCES users (id),
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE space_members (
    space_id text NOT NULL REFERENCES spaces (id),
    user_id  uuid NOT NULL REFERENCES users (id),
    role_id  uuid NOT NULL REFERENCES roles (id),
    PRIMARY KEY (space_id, user_id)
);

-- Record locking: a record with rows here is visible only to these users.
CREATE TABLE record_access (
    record_id text NOT NULL,
    user_id   uuid NOT NULL REFERENCES users (id),
    PRIMARY KEY (record_id, user_id)
);

-- Identification ------------------------------------------------------------

-- Every identifier ever issued, across all record types. Retired numbers stay.
CREATE TABLE identifiers (
    id         text PRIMARY KEY,
    kind       text NOT NULL,
    state      text NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'retired')),
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid
);
CREATE INDEX identifiers_production_base ON identifiers (substring(id from 1 for 6)) WHERE id ~ '^[0-9]{6}-';

CREATE TABLE counters (
    name text PRIMARY KEY,
    next bigint NOT NULL
);

-- Admin lists ---------------------------------------------------------------

CREATE TABLE manufacturers (
    code       text PRIMARY KEY CHECK (code ~ '^[A-Z]{2,5}$'),
    name       text NOT NULL,
    website    text NOT NULL DEFAULT '',
    active     boolean NOT NULL DEFAULT true,
    used       boolean NOT NULL DEFAULT false
);

CREATE TABLE admin_lists (
    list        text NOT NULL,
    value       text NOT NULL,
    sort_order  integer NOT NULL,
    active      boolean NOT NULL DEFAULT true,
    expected_on text[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (list, value)
);

-- Index of every record (index for git records, mirror for database records)

CREATE TABLE records (
    id         text PRIMARY KEY,
    kind       text NOT NULL,
    title      text NOT NULL DEFAULT '',
    status     text NOT NULL DEFAULT '',
    revision   text NOT NULL DEFAULT '',
    space_id   text NOT NULL DEFAULT '',
    owner_id   uuid,
    labels     text[] NOT NULL DEFAULT '{}',
    body       text NOT NULL DEFAULT '',
    origin     text NOT NULL CHECK (origin IN ('git', 'db')),
    path       text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid,
    search     tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', id), 'A') ||
        setweight(to_tsvector('english', title), 'B') ||
        setweight(to_tsvector('english', body), 'C')) STORED
);
CREATE INDEX records_search ON records USING gin (search);
CREATE INDEX records_kind ON records (kind);
CREATE INDEX records_space ON records (space_id);

CREATE TABLE links (
    src     text NOT NULL,
    type    text NOT NULL,
    dst     text NOT NULL,
    dst_rev text NOT NULL DEFAULT '',
    origin  text NOT NULL CHECK (origin IN ('git', 'db')),
    PRIMARY KEY (src, type, dst, dst_rev)
);
CREATE INDEX links_dst ON links (dst);

-- Revision history of engineering items, written when a change order releases.
CREATE TABLE item_revisions (
    item_id     text NOT NULL,
    revision    text NOT NULL,
    status      text NOT NULL CHECK (status IN ('Released', 'Superseded', 'Obsolete')),
    co_id       text NOT NULL,
    commit_hash text NOT NULL,
    released_at timestamptz NOT NULL,
    PRIMARY KEY (item_id, revision)
);

-- Attachments ---------------------------------------------------------------

CREATE TABLE files (
    sha256     text PRIMARY KEY,
    size       bigint NOT NULL,
    mime       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid
);

-- Editing ------------------------------------------------------------------

-- Private autosaved drafts, one per user and record.
CREATE TABLE edit_drafts (
    user_id    uuid NOT NULL REFERENCES users (id),
    record_id  text NOT NULL,
    base_hash  text NOT NULL DEFAULT '',
    content    jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, record_id)
);

-- Change control -------------------------------------------------------------

CREATE TABLE change_orders (
    id                text PRIMARY KEY,
    title             text NOT NULL,
    change_type       text NOT NULL CHECK (change_type IN ('Initial release', 'Design change', 'Documentation only', 'Obsolescence')),
    reason            text NOT NULL DEFAULT '',
    description       text NOT NULL DEFAULT '',
    stock_disposition text NOT NULL DEFAULT '',
    need_date         date,
    priority          text NOT NULL DEFAULT 'Normal',
    state             text NOT NULL DEFAULT 'Draft' CHECK (state IN ('Draft', 'In Review', 'Approved', 'Released', 'Rejected', 'Cancelled')),
    owner_id          uuid NOT NULL REFERENCES users (id),
    created_by        uuid NOT NULL REFERENCES users (id),
    home_space_id     text NOT NULL DEFAULT '',
    labels            text[] NOT NULL DEFAULT '{}',
    branch            text NOT NULL,
    baseline          text NOT NULL DEFAULT '',
    released_by       uuid REFERENCES users (id),
    released_at       timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    version           integer NOT NULL DEFAULT 1
);

CREATE TABLE co_items (
    co_id    text NOT NULL REFERENCES change_orders (id),
    item_id  text NOT NULL,
    action   text NOT NULL CHECK (action IN ('Release', 'Revise', 'Obsolete')),
    from_rev text NOT NULL DEFAULT '',
    to_rev   text NOT NULL DEFAULT '',
    PRIMARY KEY (co_id, item_id)
);
CREATE INDEX co_items_item ON co_items (item_id);

CREATE TABLE co_approvers (
    co_id      text NOT NULL REFERENCES change_orders (id),
    user_id    uuid NOT NULL REFERENCES users (id),
    decision   text NOT NULL DEFAULT '' CHECK (decision IN ('', 'Approved', 'Rejected')),
    decided_at timestamptz,
    comment    text NOT NULL DEFAULT '',
    PRIMARY KEY (co_id, user_id)
);

-- Work ---------------------------------------------------------------------

CREATE TABLE work_items (
    id          text PRIMARY KEY,
    type        text NOT NULL CHECK (type IN ('task', 'issue', 'action', 'epic', 'ncr')),
    space_id    text NOT NULL REFERENCES spaces (id),
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    status      text NOT NULL,
    priority    text NOT NULL DEFAULT 'Normal' CHECK (priority IN ('Low', 'Normal', 'High', 'Urgent')),
    assignee_id uuid REFERENCES users (id),
    reporter_id uuid REFERENCES users (id),
    created_by  uuid NOT NULL REFERENCES users (id),
    due_date    date,
    epic_id     text NOT NULL DEFAULT '',
    labels      text[] NOT NULL DEFAULT '{}',
    fields      jsonb NOT NULL DEFAULT '{}',
    attachments jsonb NOT NULL DEFAULT '[]',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    version     integer NOT NULL DEFAULT 1
);
CREATE INDEX work_items_space ON work_items (space_id, type);
CREATE INDEX work_items_assignee ON work_items (assignee_id);

CREATE TABLE ncr_approvals (
    ncr_id     text NOT NULL REFERENCES work_items (id),
    user_id    uuid NOT NULL REFERENCES users (id),
    decision   text NOT NULL DEFAULT '' CHECK (decision IN ('', 'Approved', 'Rejected')),
    decided_at timestamptz,
    comment    text NOT NULL DEFAULT '',
    PRIMARY KEY (ncr_id, user_id)
);

CREATE TABLE work_history (
    id        bigserial PRIMARY KEY,
    item_id   text NOT NULL,
    at        timestamptz NOT NULL DEFAULT now(),
    user_id   uuid,
    changes   jsonb NOT NULL
);
CREATE INDEX work_history_item ON work_history (item_id, at);

-- Collaboration --------------------------------------------------------------

CREATE TABLE comments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    record_id  text NOT NULL,
    revision   text NOT NULL DEFAULT '',
    author_id  uuid NOT NULL REFERENCES users (id),
    body       text NOT NULL,
    reply_to   uuid REFERENCES comments (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    edited_at  timestamptz,
    history    jsonb NOT NULL DEFAULT '[]',
    resolved   boolean NOT NULL DEFAULT false,
    hidden     boolean NOT NULL DEFAULT false
);
CREATE INDEX comments_record ON comments (record_id, created_at);

CREATE TABLE watchers (
    record_id text NOT NULL,
    user_id   uuid NOT NULL REFERENCES users (id),
    PRIMARY KEY (record_id, user_id)
);

CREATE TABLE notifications (
    id           bigserial PRIMARY KEY,
    recipient_id uuid NOT NULL REFERENCES users (id),
    event        text NOT NULL,
    record_id    text NOT NULL,
    triggered_by uuid REFERENCES users (id),
    summary      text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    read         boolean NOT NULL DEFAULT false,
    dismissed    boolean NOT NULL DEFAULT false
);
CREATE INDEX notifications_recipient ON notifications (recipient_id, created_at DESC);

-- Per-user conveniences ------------------------------------------------------

CREATE TABLE saved_filters (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id),
    view       text NOT NULL,
    name       text NOT NULL,
    filter     jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_prefs (
    user_id uuid NOT NULL REFERENCES users (id),
    key     text NOT NULL,
    value   jsonb NOT NULL,
    PRIMARY KEY (user_id, key)
);

CREATE TABLE recent_views (
    user_id   uuid NOT NULL REFERENCES users (id),
    record_id text NOT NULL,
    viewed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, record_id)
);
