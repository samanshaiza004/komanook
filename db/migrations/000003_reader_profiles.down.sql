BEGIN;
DROP TABLE profile_favorites;
DROP TABLE reader_profiles;
ALTER TABLE series DROP CONSTRAINT series_work_id_key;
ALTER TABLE series DROP CONSTRAINT series_work_id_fkey;
ALTER TABLE series DROP COLUMN work_id;
DROP TABLE works;
COMMIT;
