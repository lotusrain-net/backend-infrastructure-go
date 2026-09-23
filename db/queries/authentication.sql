-- name: GetAuthenticationSettings :one
SELECT * FROM authentication_settings WHERE singleton = TRUE;

-- name: LockAuthenticationSettings :one
SELECT * FROM authentication_settings WHERE singleton = TRUE FOR UPDATE;

-- name: PutAuthenticationSettings :exec
UPDATE authentication_settings SET password_login_enabled=$1, registration_enabled=$2,
 registration_email_verification_required=$3, allowed_email_domains=$4 WHERE singleton=TRUE;

-- name: BindBootstrapAdmin :execrows
UPDATE authentication_settings SET bootstrap_admin_user_id=$1
WHERE singleton=TRUE AND (bootstrap_admin_user_id IS NULL OR bootstrap_admin_user_id=$1);

-- name: InitializeAuthentication :execrows
UPDATE authentication_settings SET initialized_at=COALESCE(initialized_at,NOW())
WHERE singleton=TRUE AND bootstrap_admin_user_id=$1;

-- name: EnsureSecuritySettings :exec
INSERT INTO user_security_settings(user_id) VALUES ($1) ON CONFLICT DO NOTHING;

-- name: GetSecuritySettings :one
SELECT * FROM user_security_settings WHERE user_id=$1;

-- name: LockSecuritySettings :one
SELECT * FROM user_security_settings WHERE user_id=$1 FOR UPDATE;

-- name: SaveSecuritySettings :execrows
UPDATE user_security_settings SET mode=$2,totp_secret=$3,pending_secret=$4,pending_expires_at=$5,
 last_totp_step=$6,recovery_hashes=$7,recovery_codes_expires_at=$9,version=version+1 WHERE user_id=$1 AND version=$8;

-- name: MarkEmailVerified :one
UPDATE users SET email_verified_at=NOW() WHERE id=$1 RETURNING *;

-- name: InsertAuthenticationConsumption :exec
INSERT INTO authentication_consumptions(credential_id,expires_at) VALUES ($1,$2);

-- name: PruneAuthenticationConsumptions :exec
DELETE FROM authentication_consumptions WHERE expires_at < NOW();

-- name: SaveConsumedSecurityCredential :exec
UPDATE user_security_settings SET last_totp_step=$2,recovery_hashes=$3,version=version+1 WHERE user_id=$1;

-- name: InvalidatePasswordCredentials :exec
UPDATE user_security_settings SET version=version+1, pending_secret=NULL, pending_expires_at=NULL WHERE user_id=$1;
