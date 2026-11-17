-- Page forms: a template can define form fields. The body is the layout,
-- with {{key}} where each field goes.
ALTER TABLE templates ADD COLUMN form jsonb NOT NULL DEFAULT '[]';
ALTER TABLE template_versions ADD COLUMN form jsonb NOT NULL DEFAULT '[]';
