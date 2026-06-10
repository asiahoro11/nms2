# Management Server Database Maintenance Guide

Version: `v1.2.4.8`

## Scope

This guide covers basic SQLite backup, restore, health check, and security maintenance for customer operations teams.

## Important Paths

| Path | Purpose |
| --- | --- |
| `data/nms.db` | Main database |
| `data/` | Runtime data |
| `config.yaml` | Customer environment configuration |

## Backup

```bash
sudo systemctl stop nms
cp -a data "data-backup-$(date +%Y%m%d-%H%M%S)"
sudo systemctl start nms
```

For online backups, use the built-in backup function or customer snapshot tooling that guarantees consistency.

## Restore

```bash
sudo systemctl stop nms
cp -a data data-before-restore
rm -rf data
cp -a data-backup-YYYYMMDD-HHMMSS data
sudo systemctl start nms
```

## Health Check

```bash
sqlite3 data/nms.db "PRAGMA integrity_check;"
sqlite3 data/nms.db "PRAGMA quick_check;"
```

Expected result:

```text
ok
```

## Cleanup and Performance

```bash
sqlite3 data/nms.db "VACUUM;"
sqlite3 data/nms.db "ANALYZE;"
```

Run these during a maintenance window.

## Security Maintenance

- Restrict `data/` access to the service account.
- Store backups encrypted.
- Do not place backups under a Web root.
- Do not send `nms.db` through tickets, email, or chat.
- Rotate admin passwords and integration tokens after personnel or permission changes.

## Pre-Upgrade Checklist

- `data/` is backed up.
- `config.yaml` is backed up.
- Previous artifact is available for rollback.
- Current version and deployment path are recorded.

