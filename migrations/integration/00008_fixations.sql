-- +goose Up
CREATE UNIQUE INDEX fixations_phone_hash_project_id_idx ON integration.fixations(phone_hash, project_id) WHERE status = 'active';

DROP INDEX IF EXISTS integration.fixations_phone_hash_agency_id_idx;

-- +gooseDown
DROP INDEX IF EXISTS integration.fixations_phone_hash_project_id_idx;




