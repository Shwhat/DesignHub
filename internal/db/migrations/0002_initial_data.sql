-- Initial lists and counters from DATA-MODEL.md section 7.

INSERT INTO manufacturers (code, name) VALUES
    ('PXC', 'Phoenix Contact'),
    ('SIE', 'Siemens'),
    ('RIT', 'Rittal'),
    ('WAG', 'WAGO'),
    ('SCH', 'Schneider Electric'),
    ('ABB', 'ABB'),
    ('AB', 'Allen-Bradley (Rockwell Automation)');

INSERT INTO admin_lists (list, value, sort_order) VALUES
    ('units', 'Each', 1), ('units', 'Set', 2), ('units', 'Inch', 3), ('units', 'Foot', 4),
    ('units', 'Millimeter', 5), ('units', 'Meter', 6), ('units', 'Pound', 7), ('units', 'Kilogram', 8),
    ('units', 'Gallon', 9), ('units', 'Liter', 10), ('units', 'Square foot', 11),
    ('requirement_categories', 'Functional', 1), ('requirement_categories', 'Performance', 2),
    ('requirement_categories', 'Interface', 3), ('requirement_categories', 'Safety', 4),
    ('requirement_categories', 'Regulatory', 5), ('requirement_categories', 'Environmental', 6),
    ('procedure_types', 'Test', 1), ('procedure_types', 'Inspection', 2), ('procedure_types', 'Work instruction', 3),
    ('procedure_types', 'Calibration', 4), ('procedure_types', 'Maintenance', 5),
    ('document_types', 'Specification', 1), ('document_types', 'Standard', 2), ('document_types', 'Drawing', 3),
    ('document_types', 'Interface control', 4), ('document_types', 'Plan', 5), ('document_types', 'Report', 6),
    ('document_types', 'Agreement', 7), ('document_types', 'Other', 8),
    ('page_types', 'General', 1), ('page_types', 'Design review', 2), ('page_types', 'Test report', 3),
    ('page_types', 'Meeting notes', 4), ('page_types', 'Decision record', 5),
    ('issue_areas', 'Design', 1), ('issue_areas', 'Factory', 2), ('issue_areas', 'Process', 3), ('issue_areas', 'Supplier', 4);

INSERT INTO admin_lists (list, value, sort_order, expected_on) VALUES
    ('file_roles', 'Drawing', 1, '{0XX,1XX,2XX,3XX,4XX,5XX,6XX}'),
    ('file_roles', '3D model', 2, '{000,3XX}'),
    ('file_roles', 'Schematic', 3, '{3XX,4XX,5XX,6XX}'),
    ('file_roles', 'BOM file', 4, '{4XX,5XX}'),
    ('file_roles', 'Datasheet', 5, '{COTS}'),
    ('file_roles', 'Native design files', 6, '{}'),
    ('file_roles', 'Image', 7, '{}'),
    ('file_roles', 'Other', 8, '{}');

INSERT INTO counters (name, next) VALUES
    ('production', 100001),
    ('reference', 1000001),
    ('REQ', 1), ('PRC', 1), ('DOC', 1), ('PG', 1), ('CO', 1), ('SP', 1),
    ('TSK', 1), ('ISS', 1), ('NCR', 1), ('ACT', 1), ('EPC', 1);

INSERT INTO settings (key, value) VALUES
    ('allowed_hosts', '[]'),
    ('sign_in_message', '""'),
    ('co_reject_returns_to_draft', 'false'),
    ('backup_schedule', '{"hour": 2, "keep": 14}');
