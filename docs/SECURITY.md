# BDopsFlow 安全配置指南

本文档提供了 BDopsFlow 在生产环境中的安全最佳实践和配置建议。

## 目录

- [安全概述](#安全概述)
- [身份验证](#身份验证)
- [密钥管理](#密钥管理)
- [网络安全](#网络安全)
- [数据安全](#数据安全)
- [审计与监控](#审计与监控)
- [安全检查清单](#安全检查清单)
- [漏洞报告](#漏洞报告)

## 安全概述

BDopsFlow 提供了多层次的安全机制：

- **认证**: JWT Token、SSO 支持
- **授权**: RBAC 权限模型
- **加密**: 数据传输和存储加密
- **审计**: 完整的操作审计日志
- **隔离**: 多租户/多域支持

## 身份验证

### JWT 配置

生产环境必须修改默认的 JWT secret：

```yaml
jwt:
  secret: "your-very-long-and-random-secret-key-at-least-32-characters"
  expiry_hours: 2
  refresh_expiry_hours: 168  # 7天
```

**生成安全密钥**:
```bash
# 使用 OpenSSL 生成强密钥
openssl rand -hex 32
```

### SSO 配置 (推荐)

启用 SSO 单点登录，避免密码管理：

```yaml
sso:
  enabled: true
  url: "https://your-sso-provider.com"
  public_key: "your-sso-public-key-base64"
  timeout: 10
```

### 密码策略

修改默认管理员密码：
1. 首次登录后立即修改
2. 使用强密码策略
3. 启用定期密码轮换

```yaml
app:
  allow_register: false  # 生产环境禁用自注册
```

### 安全认证最佳实践

1. **启用 HTTPS**
2. **配置安全的 Cookie 属性**
3. **实现登录失败锁定**
4. **启用多因素认证 (MFA)** - 建议扩展
5. **定期轮换 JWT Secret**

## 密钥管理

### RSA 密钥对

BDopsFlow 使用 RSA 加密敏感配置，必须生成并安全存储密钥对：

```bash
# 使用内置工具生成
cd scheduler
go run ./cmd keygen

# 输出类似:
rsa:
  public_key: "MIIBIjANBgkqhkiG9w0BAQEFAAO..."
  private_key: "MIIEvQIBADANBgkqhkiG9w0BAQEFAAS..."
```

配置到 `config.yaml`:
```yaml
rsa:
  public_key: "your-public-key-base64"
  private_key: "your-private-key-base64"
```

### 加密数据库密码

不要在配置文件中明文存储密码，使用加密功能：

```bash
# 1. 加密密码
./scheduler encrypt-password -config config.yaml -password "your-db-password"

# 输出: RSA_ENCRYPTED:abc123...

# 2. 更新配置文件
database:
  rqlite_password: "RSA_ENCRYPTED:abc123..."
  
redis:
  password: "RSA_ENCRYPTED:xyz789..."
```

### 密钥存储最佳实践

1. **使用密钥管理系统 (KMS)**:
   - AWS KMS
   - HashiCorp Vault
   - Azure Key Vault

2. **环境变量注入**:
```bash
export BD_RSA_PRIVATE_KEY="your-private-key"
export BD_JWT_SECRET="your-jwt-secret"
```

3. **密钥轮换**:
   - 每 90 天轮换一次 RSA 密钥
   - 支持旧密钥解密，避免数据丢失

## 数据源加密

### 配置加密密钥

数据源密码使用 AES-256-GCM 加密存储：

```yaml
datasource:
  encryption_key: "change-in-prod-32byte-key1-here1"  # 必须 32 字节
  key_source: "direct"  # 或 "env", "file"
  key_env_var: "BDOPSFLOW_ENCRYPTION_KEY"
  key_file: "/path/to/encryption.key"
  auto_rotate_days: 90
```

### 生成强加密密钥

```bash
# 生成 32 字节密钥
openssl rand -hex 16  # 32 字符
# 或
head -c 32 /dev/urandom | base64
```

## 网络安全

### TLS 配置

生产环境必须启用 HTTPS：

#### 1. 使用 Nginx 终止 TLS

```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;
    
    ssl_certificate /path/to/fullchain.pem;
    ssl_certificate_key /path/to/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    
    # 安全头
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Content-Security-Policy "default-src 'self'" always;
    
    # HSTS preload
    add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload" always;
    
    location / {
        proxy_pass http://127.0.0.1:8080;
        # ... 其他代理配置
    }
}
```

#### 2. 使用 Let's Encrypt 获取免费证书

```bash
certbot certonly --nginx -d your-domain.com
```

### CORS 配置

严格限制跨域访问：

```yaml
app:
  cors_allow_origins:
    - "https://your-domain.com"
    - "https://app.your-domain.com"
    # 不要使用 "*" 在生产环境
```

### 网络隔离

1. **防火墙规则**:
   - 仅开放必要端口 (80, 443)
   - 限制 SSH 访问特定 IP
   - 禁止公网访问 rqlite/Redis

2. **内网部署**:
   - 调度器和执行器在同一 VPC
   - 使用内网 DNS 和安全组
   - gRPC 通信不对外暴露

3. **VPN 访问**:
   - 管理界面需要 VPN 连接
   - API 访问通过 API Gateway

## 数据安全

### 数据库安全

1. **rqlite 认证**:
```yaml
database:
  rqlite_user: "bdopsflow"
  rqlite_password: "RSA_ENCRYPTED:encrypted-password"
  rqlite_tls: true  # 启用 TLS
```

2. **Redis 认证与隔离**:
```yaml
redis:
  password: "RSA_ENCRYPTED:redis-password"
  db: 0  # 使用独立的 DB
```

3. **定期数据备份**:
参考 [部署指南](./DEPLOYMENT.md#备份与恢复)

### 数据保留策略

配置审计日志和查询历史的保留策略：

```yaml
# 通过系统配置 API 设置
audit_log_retention_days: 90
query_history_retention_days: 30
task_execution_retention_days: 180
```

### 敏感数据处理

1. **密码脱敏**:
   - 日志中不记录密码
   - API 响应中脱敏处理

2. **数据源连接信息**:
   - 始终加密存储
   - 访问需要 `manage` 权限

## RBAC 权限模型

### 权限层次

```
system_admin (全局)
    └── domain_admin (域内)
            └── user (基本)
```

### 权限最佳实践

1. **最小权限原则**:
   - 只授予必要的权限
   - 定期审查权限分配

2. **角色分离**:
   - 管理账号和操作账号分离
   - 不同环境使用不同账号

3. **实例级权限**:
   - 数据源和 Webhook 支持细粒度权限
   - 配置实例权限矩阵

### 权限配置示例

```yaml
# 数据分析师角色
permissions:
  datasource: ["query", "read"]
  query_history: ["read"]
  saved_sql: ["read", "create", "update"]

# 运维工程师角色
permissions:
  task: ["*"]
  workflow: ["*"]
  executor: ["read", "online", "offline"]
  datasource: ["read", "query"]
```

## 审计与监控

### 审计日志

BDopsFlow 记录所有关键操作：

- 用户登录/登出
- 权限变更
- 任务创建/执行
- 数据源配置变更
- 系统配置变更

### 安全监控

1. **使用健康检查端点**:
```bash
# 监控服务健康
curl /healthz
curl /readyz
```

2. **监控异常行为**:
   - 频繁的登录失败
   - 非工作时间的敏感操作
   - 大规模权限变更

3. **日志收集与分析**:
   - 集成 ELK / Grafana Loki
   - 配置关键告警
   - 定期安全审计

### 告警配置

建议监控以下指标:

- 认证失败 > 5次/分钟
- 权限授予操作
- 系统配置修改
- 健康检查失败

## 安全检查清单

### 部署前检查

- [ ] 修改所有默认密码
- [ ] 生成并配置 RSA 密钥对
- [ ] 配置 JWT Secret (32+ 字符)
- [ ] 配置数据源加密密钥 (32 字节)
- [ ] 禁用用户自注册
- [ ] 配置 HTTPS / TLS
- [ ] 配置 CORS 白名单
- [ ] 配置防火墙规则
- [ ] 设置备份策略
- [ ] 配置日志收集

### 定期检查 (每月)

- [ ] 审查用户账户
- [ ] 审查权限分配
- [ ] 轮换密钥
- [ ] 检查审计日志
- [ ] 验证备份完整性
- [ ] 更新依赖包安全补丁

### 年度检查

- [ ] 完整安全审计
- [ ] 渗透测试
- [ ] 灾难恢复演练
- [ ] 安全策略评审

## Webhook 安全

### 签名验证

Webhook 使用 HMAC-SHA256 签名验证：

```yaml
webhook:
  secret: "your-webhook-signing-secret"
  retry:
    max_attempts: 3
    backoff: "exponential"
```

### Webhook 接收方安全

1. **验证签名**:
```
X-BDOPSFLOW-SIGNATURE: sha256=abc123...
X-BDOPSFLOW-TIMESTAMP: 1699999999
```

2. **速率限制**:
3. **IP 白名单**:

## 漏洞报告

### 安全问题反馈

如果你发现安全漏洞，请通过以下方式报告：

1. **私密报告**: security@bdopsflow.example.com
2. **GitHub Security Advisories** (如果开源)

请提供:
- 问题描述
- 复现步骤
- 影响范围
- 可能的修复建议

### 响应时间

- **严重漏洞**: 24小时内响应
- **高危**: 48小时内响应
- **中低危**: 72小时内响应

## 额外安全建议

1. **容器安全**:
   - 使用最小基础镜像
   - 非 root 用户运行
   - 镜像漏洞扫描

2. **依赖管理**:
   - 定期更新依赖
   - 使用 `go mod` 和 `npm audit`
   - 监控 CVE 数据库

3. **安全开发**:
   - 代码审查
   - 安全测试
   - 威胁建模

4. **事件响应**:
   - 制定响应计划
   - 定期演练
   - 保留取证数据

## 参考资源

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CIS Benchmarks](https://www.cisecurity.org/cis-benchmarks/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [部署指南](./DEPLOYMENT.md)
