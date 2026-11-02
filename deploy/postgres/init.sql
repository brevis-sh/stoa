-- Um Postgres, dois bancos, cada um com um dono diferente da conversa:
-- `lake` é o catálogo Iceberg (duas tabelas e um compare-and-swap no ponteiro
-- de metadados), `brevis` é o engine (fila, runs, workflows, console).
-- Roda só na primeira subida do volume, que é como o Postgres oficial trata
-- este diretório.
CREATE DATABASE brevis;
