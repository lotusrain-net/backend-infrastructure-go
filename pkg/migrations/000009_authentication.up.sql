ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;

CREATE TABLE authentication_settings (
 singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
 password_login_enabled BOOLEAN NOT NULL DEFAULT TRUE,
 registration_enabled BOOLEAN NOT NULL DEFAULT FALSE,
 registration_email_verification_required BOOLEAN NOT NULL DEFAULT TRUE,
 allowed_email_domains TEXT[] NOT NULL DEFAULT '{}',
 bootstrap_admin_user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
 initialized_at TIMESTAMPTZ
);
-- seed-admin binds the configured administrator in a transaction. Do not guess
-- from creation order when upgrading installations with multiple administrators.
INSERT INTO authentication_settings DEFAULT VALUES;

CREATE TABLE user_security_settings (
 user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 mode TEXT NOT NULL DEFAULT 'default' CHECK (mode IN ('default','email','totp')),
 totp_secret BYTEA,
 pending_secret BYTEA,
 pending_expires_at TIMESTAMPTZ,
 last_totp_step BIGINT NOT NULL DEFAULT -1,
 recovery_hashes TEXT[] NOT NULL DEFAULT '{}',
 recovery_codes_expires_at TIMESTAMPTZ,
 version BIGINT NOT NULL DEFAULT 0,
 CHECK (mode <> 'totp' OR totp_secret IS NOT NULL)
);
-- A registration receipt is committed together with the user and role. Retain receipts
-- until expiry; Redis cleanup is best effort and cannot authorize a second registration.
CREATE TABLE authentication_consumptions (
 credential_id TEXT PRIMARY KEY,
 expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX authentication_consumptions_expiry_idx ON authentication_consumptions(expires_at);
INSERT INTO permissions (name,description) VALUES
 ('system-settings:read','Read authentication settings'),
 ('system-settings:write','Manage authentication settings')
ON CONFLICT (name) DO NOTHING;
