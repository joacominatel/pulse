-- migration: 000003_create_communities.down.sql
-- drops the communities table and related objects

DROP INDEX IF EXISTS pulse.idx_communities_creator;
DROP INDEX IF EXISTS pulse.idx_communities_momentum;
DROP INDEX IF EXISTS pulse.idx_communities_slug;
DROP TABLE IF EXISTS pulse.communities;
