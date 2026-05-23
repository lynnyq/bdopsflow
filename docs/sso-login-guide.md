# SSO 第三方登录 — 说明与使用文档

## 1. 功能概述

BDopsFlow 支持两种登录方式：

| 登录方式           | 说明                              | 默认                      |
| -------------- | ------------------------------- | ----------------------- |
| **SSO 统一认证登录** | 通过第三方 SSO 服务验证用户身份，首次登录自动创建本地账号 | ✅ 启用后默认                 |
| **本地登录**       | 使用本地数据库存储的账号密码登录                | `/login?isSso=false` 切换 |

### 核心流程

```
用户访问 /login
  │
  ├─ SSO 模式（默认）
  │   前端用 SSO 公钥加密密码
  │   → POST /api/auth/sso-login
  │   → 后端转发到 SSO 服务验证
  │   → 验证成功：查找/创建本地用户 → 生成 JWT → 登录成功
  │   → 验证失败：返回 SSO 错误信息
  │
  └─ 本地模式（isSso=false）
      前端用本地公钥加密密码
      → POST /api/auth/login
      → 后端本地 RSA 解密 + bcrypt 校验
      → 登录成功/失败
```

## 2. 配置说明

### 2.1 配置文件

在 `config.yaml` 中新增 `sso` 配置段：

```yaml
sso:
  # 是否启用 SSO 登录
  # true: 登录页默认显示 SSO 登录，可通过 /login?isSso=false 切换本地登录
  # false: 仅使用本地登录，登录页不显示切换按钮
  enabled: true

  # SSO 验证接口地址
  # 后端将前端传来的用户名和加密密码原样转发到此地址
  url: "http://sso.com.cn/account"

  # SSO RSA 公钥（PKCS#8 格式，Base64 编码，不含 PEM 头尾）
  # 此公钥由 SSO 服务提供，用于前端加密用户密码
  # 注意：此公钥与本项目的 RSA 密钥对（rsa.public_key）无关
  public_key: ""

  # SSO 请求超时时间（秒）
  # 超时后返回 "SSO登录失败，请稍后再试"
  timeout: 10
```

### 2.2 配置项详解

| 配置项              | 类型     | 必填  | 默认值   | 说明                          |
| ---------------- | ------ | --- | ----- | --------------------------- |
| `sso.enabled`    | bool   | 否   | false | 是否启用 SSO 登录                 |
| `sso.url`        | string | 是\* | -     | SSO 验证接口地址，enabled=true 时必填 |
| `sso.public_key` | string | 是\* | -     | SSO RSA 公钥，enabled=true 时必填 |
| `sso.timeout`    | int    | 否   | 10    | 请求超时秒数，最小 1 秒               |

> \*当 `sso.enabled=true` 时，`url` 和 `public_key` 必须配置，否则启动报错。

### 2.3 与 RSA 配置的关系

项目中有两套 RSA 密钥，用途完全不同：

| 配置                                   | 用途                                | 密钥来源                       |
| ------------------------------------ | --------------------------------- | -------------------------- |
| `rsa.public_key` / `rsa.private_key` | 本地登录密码加解密、配置文件密码解密                | 使用 `./scheduler keygen` 生成 |
| `sso.public_key`                     | SSO 登录密码加密（前端加密后传给后端，后端原样转发给 SSO） | 由 SSO 服务方提供                |

**关键区别**：SSO 公钥加密的密码后端不解密，而是原样转发给 SSO 服务验证。本地公钥加密的密码后端用本地私钥解密后做 bcrypt 校验。

## 3. 接口说明

### 3.1 获取公钥

```
GET /api/auth/public-key
```

**响应**：

```json
{
  "code": 0,
  "data": {
    "public_key": "本地RSA公钥(Base64)",
    "sso_enabled": true,
    "sso_public_key": "SSO RSA公钥(Base64)"
  }
}
```

| 字段               | 说明                                  |
| ---------------- | ----------------------------------- |
| `public_key`     | 本地 RSA 公钥，用于本地登录密码加密                |
| `sso_enabled`    | SSO 是否启用                            |
| `sso_public_key` | SSO RSA 公钥，仅 `sso_enabled=true` 时返回 |

### 3.2 SSO 登录

```
POST /api/auth/sso-login
Content-Type: application/json
```

**请求**：

```json
{
  "username": "用户名",
  "password": "SSO公钥加密后的hex密文"
}
```

**成功响应**（SSO 验证通过）：

```json
{
  "code": 0,
  "data": {
    "token": "JWT令牌",
    "user": {
      "id": 1,
      "username": "",
      "real_name": "",
      "phone": "",
      "role": "user",
      "email": "",
      "domain_id": 0,
      "permissions": [...]
    }
  }
}
```

**失败响应**：

| 场景        | HTTP 状态码 | 错误信息                                   |
| --------- | -------- | -------------------------------------- |
| SSO 密码错误  | 401      | SSO 返回的 message（如 "用户名密码不一致,请重新登录..."） |
| SSO 用户不存在 | 401      | SSO 返回的 message（如 "用户名密码不一致，请联系管理员"）   |
| SSO 公钥错误  | 401      | "密码解析不通过"                              |
| SSO 服务不可达 | 502      | "SSO登录失败，请稍后再试"                        |
| SSO 未启用   | 400      | "SSO登录未启用，请使用本地登录"                     |

### 3.3 本地登录（已有接口，无变更）

```
POST /api/auth/login
Content-Type: application/json
```

```json
{
  "username": "用户名",
  "password": "本地公钥加密后的hex密文"
}
```

## 4. SSO 登录详细流程

### 4.1 前端流程

```
1. 用户访问 /login
2. 前端调用 GET /api/auth/public-key 获取公钥
3. 根据 sso_enabled 和 URL 参数 isSso 决定登录模式：
   - isSso=true（默认）且 sso_enabled=true → SSO 登录模式
   - isSso=false → 本地登录模式
4. 用户输入用户名和密码
5. SSO 模式：用 sso_public_key 加密密码 → POST /api/auth/sso-login
   本地模式：用 public_key 加密密码 → POST /api/auth/login
6. 登录成功 → 跳转首页
   登录失败 → 显示错误信息
```

### 4.2 后端 SSO 验证流程

```
1. 接收 {username, password}（password 是 SSO 公钥加密的 hex 密文）
2. 构造 SSO 请求：
   {
     "loginName": "前端传来的username",
     "password": "前端传来的加密password（原样透传，不解密）"
   }
3. POST 到 SSO URL，超时时间由 sso.timeout 控制
4. 解析 SSO 响应：
   - code="3000" → 验证成功
   - code="3001" → 验证失败，返回 SSO 的 message
   - 其他/网络错误 → "SSO登录失败，请稍后再试"
```

### 4.3 SSO 登录成功后的用户处理

```
SSO 验证成功 → 获取 SSO 用户信息
  │
  ├─ 本地数据库已有该用户（username = loginName）
  │   → 更新 last_login_at
  │   → 生成 JWT
  │   → 返回登录成功
  │
  └─ 本地数据库没有该用户
      → 自动创建用户：
        - username = SSO loginName
        - real_name = SSO idCardName
        - phone    = SSO mobileNo
        - email    = SSO email
        - role     = user（普通用户）
        - password = 空（SSO 用户无本地密码）
      → 生成 JWT
      → 返回登录成功
```

### 4.4 SSO 响应字段映射

| SSO 字段       | 本地字段        | 说明         |
| ------------ | ----------- | ---------- |
| `loginName`  | `username`  | 登录用户名      |
| `idCardName` | `real_name` | 真实姓名       |
| `mobileNo`   | `phone`     | 手机号        |
| `email`      | `email`     | 邮箱         |
| -            | `role`      | 固定为 `user` |
| -            | `password`  | 固定为空       |

## 5. 前端使用指南

### 5.1 登录模式切换

- **默认 SSO 登录**：直接访问 `/login`
- **切换本地登录**：访问 `/login?isSso=false`
- 登录页顶部显示切换按钮（仅 SSO 启用时可见）：

```
┌─────────────────────────────┐
│  SSO 登录  |  本地登录      │  ← 点击切换
├─────────────────────────────┤
│  用户名: [              ]   │
│  密  码: [              ]   │
│                             │
│  [     SSO 登录     ]       │  ← 按钮文本随模式变化
└─────────────────────────────┘
```

### 5.2 URL 参数

| 参数      | 值           | 效果                |
| ------- | ----------- | ----------------- |
| `isSso` | 不传 / `true` | SSO 登录模式（SSO 启用时） |
| `isSso` | `false`     | 本地登录模式            |

## 6. 部署指南

### 6.1 开发环境

1. 在 `config.yaml` 中配置 SSO：

```yaml
sso:
  enabled: false    # 开发环境可关闭 SSO
```

1. 不启用 SSO 时，登录页仅显示本地登录。

### 6.2 生产环境

1. 在 `config.prod.yaml` 中配置 SSO：

```yaml
sso:
  enabled: true
  url: "http://sso.com/account"
  public_key: "SSO服务方提供的RSA公钥"
  timeout: 10
```

1. 获取 SSO 公钥：
   - 联系 SSO 服务管理员获取 PKCS#8 格式的 RSA 公钥
   - 公钥为 Base64 编码，不含 PEM 头尾（`-----BEGIN PUBLIC KEY-----` 等）
2. 启动服务验证：
   - 查看日志输出 `SSO login enabled` 表示 SSO 初始化成功
   - 访问 `/login` 确认登录页显示 SSO/本地切换按钮

### 6.3 网络要求

- 调度中心后端必须能访问 SSO 服务地址（`http://sso.com`）
- 如有防火墙，需放行对应端口
- SSO 请求超时默认 10 秒，网络较慢时可适当增大 `timeout`

## 7. 常见问题

### Q1: SSO 登录提示"SSO登录失败，请稍后再试"

**可能原因**：

- SSO 服务不可达，检查网络连通性：`curl http://sso.com/account`
- 请求超时，尝试增大 `sso.timeout`
- 后端日志查看具体错误：`SSOLogin: failed to call SSO service`

### Q2: SSO 登录提示"密码解析不通过"

**原因**：`sso.public_key` 配置错误，前端用错误的公钥加密密码，SSO 服务无法解密。

**解决**：联系 SSO 服务管理员确认正确的公钥。

### Q3: SSO 登录成功但菜单空白

**原因**：SSO 自动创建的用户角色为 `user`（普通用户），需要管理员在后台为该用户分配角色权限，或在 `bdopsflow_role_permissions` 表中配置 `user` 角色的菜单权限。

### Q4: SSO 用户能否修改密码？

SSO 用户的本地密码为空，不能通过"修改密码"功能修改。密码管理统一由 SSO 服务负责。

### Q5: 如何禁用 SSO 登录？

将 `sso.enabled` 设为 `false`，重启服务即可。禁用后：

- 登录页不显示 SSO/本地切换按钮
- `/api/auth/sso-login` 接口返回 400 错误
- 已创建的 SSO 用户账号仍保留在数据库中

### Q6: SSO 用户首次登录后被分配了什么权限？

SSO 自动创建的用户角色为 `user`（普通用户），权限取决于 `bdopsflow_role_permissions` 表中 `user` 角色配置的权限。如需调整，请联系系统管理员配置角色权限。

## 8. 安全说明

1. **密码传输**：SSO 模式下，密码由前端使用 SSO 公钥加密后传输，后端不解密，原样转发给 SSO 服务验证
2. **密码存储**：SSO 用户本地不存储密码，避免密码泄露风险
3. **JWT 令牌**：SSO 登录和本地登录使用相同的 JWT 机制，令牌格式和过期时间一致
4. **审计日志**：SSO 登录操作记录审计日志，与本地登录一致
5. **超时保护**：SSO 请求设置超时时间，防止服务不可用时长时间阻塞
6. **公钥隔离**：SSO 公钥和本地公钥完全独立，互不影响

