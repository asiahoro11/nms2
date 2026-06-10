# Management Server 数据库与备份加密手册

版本: `v1.2.4.8`

## 范围

本手册说明客户如何处理备份加密 密码保护与安全保存  
实际是否启用完整数据库加密需依客户授权与部署策略确认

## 基本原则

- 备份文件必须加密
- 加密密码不得与系统登录密码相同
- 密码需由客户密码库保存
- 不要把密码写在 script repo email ticket 或文件内

## 建议密码规则

- 至少 12 字符
- 包含大小写 英数与符号
- 不使用公司名称 项目名称 设备名称
- 每次交付或维护后可轮替

## 加密备份流程

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

## 还原流程

```http
POST /api/v1/system/restore/encrypted
Authorization: Bearer <admin-jwt>
Content-Type: multipart/form-data
```

| 字段 | 说明 |
| --- | --- |
| `file` | 加密备份文件 |
| `password` | 备份密码 |

## 验收

- 备份文件没有明文数据
- 错误密码无法还原
- 正确密码可在测试环境还原
- 还原后 `/api/v1/system/info` 正常
- 还原后用户 权限 device topology audit 数据存在

## 保存策略

- 至少保留每日 7 份 每周 4 份 每月 3 份
- 至少一份离线或异地保存
- 定期抽测还原
- 过期备份需安全删除

