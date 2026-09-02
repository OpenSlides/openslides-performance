CREATE TEMP TABLE config AS
SELECT
    'dummy'::TEXT AS base_username,
    1::INT AS suffix_start,
    200000::INT AS suffix_end,
    1::INT AS meeting_id,
    'Delegates'::TEXT AS group_external_id,
    1::INT AS organization_id;

DO $$
DECLARE
    v_meeting_id INT;
BEGIN
    SELECT meeting_id INTO v_meeting_id FROM config;
    IF NOT EXISTS (SELECT 1 FROM meeting_t WHERE id = v_meeting_id) THEN
        RAISE EXCEPTION 'Meeting % does not exist!', v_meeting_id;
    END IF;
END $$;

DO $$
DECLARE
    v_meeting_id INT;
    v_group_external_id TEXT;
    v_group_id INT;
BEGIN
    SELECT meeting_id, group_external_id INTO v_meeting_id, v_group_external_id FROM config;
    SELECT id INTO v_group_id
    FROM group_t
    WHERE meeting_id = v_meeting_id
      AND external_id = v_group_external_id;
    IF v_group_id IS NULL THEN
        RAISE EXCEPTION 'Group with external_id "%" does not exist in meeting %!',
            v_group_external_id, v_meeting_id;
    END IF;
    RAISE NOTICE 'Gruppe gefunden: ID %', v_group_id;
END $$;

BEGIN;

CREATE TEMP TABLE temp_usernames AS
SELECT c.base_username || '_' || n AS username
FROM config c
CROSS JOIN generate_series(
    (SELECT suffix_start FROM config),
    (SELECT suffix_end FROM config)
) AS n;

CREATE INDEX idx_temp_usernames ON temp_usernames(username);

INSERT INTO user_t (username, password, is_active)
SELECT
    tu.username,
    '$argon2id$v=19$m=65536,t=3,p=4$SGDZnDTQ2T18QHKEck/kyA$/ovaNzv3JOKcdAGIPQPloZTAFV8q0sC0QeyHI+ouX1I',
    true
FROM temp_usernames tu
WHERE NOT EXISTS (
    SELECT 1 FROM user_t u WHERE u.username = tu.username
);

DO $$
DECLARE
    v_inserted_count INT;
BEGIN
    GET DIAGNOSTICS v_inserted_count = ROW_COUNT;
    RAISE NOTICE 'created % new users', v_inserted_count;
END $$;

CREATE TEMP TABLE target_user_ids AS
SELECT u.id
FROM user_t u
JOIN temp_usernames tu ON u.username = tu.username;

CREATE INDEX idx_target_user_ids ON target_user_ids(id);

INSERT INTO meeting_user_t (user_id, meeting_id)
SELECT
    tui.id,
    c.meeting_id
FROM target_user_ids tui
CROSS JOIN config c
WHERE NOT EXISTS (
    SELECT 1
    FROM meeting_user_t mu
    WHERE mu.user_id = tui.id
      AND mu.meeting_id = c.meeting_id
);

DO $$
DECLARE
    v_inserted_count INT;
BEGIN
    GET DIAGNOSTICS v_inserted_count = ROW_COUNT;
    RAISE NOTICE 'Created % new meeting users', v_inserted_count;
END $$;

INSERT INTO nm_group_meeting_user_ids_meeting_user_t (group_id, meeting_user_id)
SELECT
    g.id AS group_id,
    mu.id AS meeting_user_id
FROM meeting_user_t mu
JOIN target_user_ids tui ON mu.user_id = tui.id
CROSS JOIN config c
JOIN group_t g ON g.meeting_id = c.meeting_id AND g.external_id = c.group_external_id
WHERE mu.meeting_id = c.meeting_id
ON CONFLICT (group_id, meeting_user_id) DO NOTHING;

INSERT INTO nm_meeting_present_user_ids_user_t (meeting_id, user_id)
SELECT
    c.meeting_id,
    tui.id
FROM target_user_ids tui
CROSS JOIN config c
ON CONFLICT (meeting_id, user_id) DO NOTHING;

COMMIT;

DROP TABLE IF EXISTS temp_usernames;
DROP TABLE IF EXISTS target_user_ids;
DROP TABLE IF EXISTS config;
