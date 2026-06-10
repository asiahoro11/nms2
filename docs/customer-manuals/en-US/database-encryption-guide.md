# Management Server Database and Backup Encryption Guide

Version: `v1.2.4.8`

## Scope

This guide covers encrypted backups, password protection, and secure retention. Full database encryption availability depends on customer license and deployment policy.

## Principles

- Backup files must be encrypted.
- Backup passwords must not match login passwords.
- Passwords must be stored in the customer password vault.
- Do not write passwords in scripts, repositories, emails, tickets, or documents.

## Password Rules

- At least 12 characters.
- Include uppercase, lowercase, numbers, and symbols.
- Do not use company, project, or device names.
- Rotate after delivery or maintenance when required.

## Encrypted Backup

```http
POST /api/v1/system/backup/encrypted
Authorization: Bearer <admin-jwt>
Content-Type: application/json
```

```json
{
  "password": "<backup-password>"
}
```

## Restore

```http
POST /api/v1/system/restore/encrypted
Authorization: Bearer <admin-jwt>
Content-Type: multipart/form-data
```

| Field | Description |
| --- | --- |
| `file` | Encrypted backup file |
| `password` | Backup password |

## Acceptance

- Backup file does not contain plaintext data.
- Wrong password cannot restore.
- Correct password restores in a test environment.
- `/api/v1/system/info` works after restore.
- Users, permissions, devices, topology, and audit data remain available after restore.

## Retention Policy

- Keep at least daily 7, weekly 4, monthly 3 backups.
- Keep at least one offline or offsite backup.
- Test restore regularly.
- Securely delete expired backups.

