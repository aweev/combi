-- Drop Schema Isolation for search
DROP SCHEMA IF EXISTS search CASCADE;

DROP TRIGGER IF EXISTS tsvectorupdate ON search_documents;
DROP FUNCTION IF EXISTS search_documents_vector_update;
DROP TABLE IF EXISTS search_documents;