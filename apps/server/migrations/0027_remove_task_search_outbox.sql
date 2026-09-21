-- Remove the OpenSearch change capture after reverting indexed task search.
-- IF EXISTS also handles installations where migration 0026 only partially ran.
DROP TRIGGER IF EXISTS task_search_after_insert;
DROP TRIGGER IF EXISTS task_search_after_update;
DROP TRIGGER IF EXISTS task_search_after_delete;
DROP TABLE IF EXISTS task_search_outbox;
