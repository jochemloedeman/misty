SET extension_directory = '/opt/duckdb/extensions';
SET memory_limit = '384MB';
SET temp_directory = '/tmp';
LOAD postgres;

ATTACH '' AS pg (TYPE postgres, READ_ONLY);
ATTACH '/data/staging/misty.duckdb' AS snap;
USE snap;

CREATE TABLE users AS
    FROM postgres_query('pg', '
        SELECT id,
               push_token IS NOT NULL AS push_enable
        FROM users');

CREATE TABLE monitors AS SELECT * FROM pg.public.monitors;
CREATE TABLE forecasts AS SELECT * FROM pg.public.forecasts;
CREATE TABLE notifications AS SELECT * FROM pg.public.notifications;
