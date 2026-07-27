-- name: GetUserPreferences :one
SELECT user_id, theme, color_mode, accent_color, font_scale, radius_scale, created_at, updated_at
FROM user_preferences
WHERE user_id = $1;

-- name: UpsertUserPreferences :exec
INSERT INTO user_preferences (user_id, theme, color_mode, accent_color, font_scale, radius_scale)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id) DO UPDATE
SET theme = EXCLUDED.theme,
    color_mode = EXCLUDED.color_mode,
    accent_color = EXCLUDED.accent_color,
    font_scale = EXCLUDED.font_scale,
    radius_scale = EXCLUDED.radius_scale,
    updated_at = NOW();
