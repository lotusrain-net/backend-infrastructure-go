# SP-04 IAM and RBAC

## Ownership

`internal/modules/iam/` and IAM sections of `api/openapi.yaml` after foundation integration.

## Deliverables

- User, role, permission, user-role, and role-permission domain/application layers.
- Argon2id password hashing, JWT access tokens, opaque refresh tokens stored as SHA-256 hashes, logout, and revoke-all.
- Cookie and bearer authentication.
- Exact, module wildcard, and global wildcard authorization.
- Login, refresh, logout, current-user, user, role, and permission endpoints.

## Tests

- Password vectors, token tampering/expiry, refresh replay/revocation, cookie flags, inactive users, duplicate identities, wildcard authorization, and denied access.

## Exclusions

- No OAuth, captcha, Douyin permissions, or old password compatibility.
