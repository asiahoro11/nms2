# Management Server 数据库维护手册

版本: `v1.2.4.8`

## 范围

本手册提供客户运维人员执行 SQLite 数据库备份 还原 健康检查与安全维护的基本流程

## 重要数据

| 路径 | 用途 |
| --- | --- |
| `data/nms.db` | 主数据库 |
| `data/` | runtime data |
| `config.yaml` | 客户环境配置 |

## 备份

```bash
sudo systemctl stop nms
cp -a data "data-backup-$(date +%Y%m%d-%H%M%S)"
sudo systemctl start nms
```

在线备份建议使用系统内置备份功能  或由客户 DB/文件备份工具执行一致性快照

## 还原

```bash
sudo systemctl stop nms
cp -a data data-before-restore
rm -rf data
cp -a data-backup-YYYYMMDD-HHMMSS data
sudo systemctl start nms
```

## 健康检查

```bash
sqlite3 data/nms.db "PRAGMA integrity_check;"
sqlite3 data/nms.db "PRAGMA quick_check;"
```

预期结果

```text
ok
```

## 清理与性能

```bash
sqlite3 data/nms.db "VACUUM;"
sqlite3 data/nms.db "ANALYZE;"
```

建议在维护时段执行

## 安全维护

- 限制 `data/` 只有服务账号可读写
- 备份文件需加密保存
- 备份文件不可放在 Web root
- 不要在 ticket email 或聊天工具中传送 `nms.db`
- 离职或权限调整后需更换 admin password 与 integration token

## 升级前检查

- 已备份 `data/`
- 已备份 `config.yaml`
- 已确认可回退上一版 artifact
- 已记录当前版本与部署路径

