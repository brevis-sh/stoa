-- One Postgres, two databases, each owned by a different part of the story:
-- `lake` is the Iceberg catalog (two tables and a compare-and-swap on the
-- metadata pointer), `brevis` is the engine (queue, runs, workflows, console).
-- This runs only on the volume's first start, which is how the official
-- Postgres image treats this directory.
CREATE DATABASE brevis;
