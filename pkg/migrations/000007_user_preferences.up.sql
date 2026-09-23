CREATE TABLE user_preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    theme TEXT NOT NULL DEFAULT 'enterprise',
    color_mode TEXT NOT NULL DEFAULT 'system',
    accent_color TEXT,
    font_scale TEXT NOT NULL DEFAULT 'standard',
    radius_scale TEXT NOT NULL DEFAULT 'compact',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_preferences_theme_check CHECK (theme IN ('enterprise', 'cyberpunk')),
    CONSTRAINT user_preferences_color_mode_check CHECK (color_mode IN ('light', 'dark', 'system')),
    CONSTRAINT user_preferences_font_scale_check CHECK (font_scale IN ('small', 'standard', 'large')),
    CONSTRAINT user_preferences_radius_scale_check CHECK (radius_scale IN ('square', 'compact', 'rounded')),
    CONSTRAINT user_preferences_accent_color_check CHECK (accent_color IS NULL OR accent_color ~ '^#[0-9A-Fa-f]{6}$')
);
