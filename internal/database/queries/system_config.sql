-- name: GetSystemConfig :one
SELECT * FROM system_config
WHERE key = $1;

-- name: SetSystemConfig :one
INSERT INTO system_config (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, updated_at = NOW()
RETURNING *;

-- name: GetKillSwitchStatus :one
SELECT value FROM system_config
WHERE key = 'kill_switch';

-- name: EnableKillSwitch :exec
UPDATE system_config
SET value = jsonb_build_object(
    'enabled', true,
    'reason', $1,
    'timestamp', to_jsonb(NOW())
)
WHERE key = 'kill_switch';

-- name: DisableKillSwitch :exec
UPDATE system_config
SET value = jsonb_build_object(
    'enabled', false,
    'reason', null,
    'timestamp', to_jsonb(NOW())
)
WHERE key = 'kill_switch';

