# RSA 密码加密设计

## 概述

将系统中的密码传输机制从 Base64 编码升级为 RSA 非对称加密，同时支持配置文件中敏感密码的加密存储。

### 目标

1. 前端使用 RSA 公钥加密用户密码传输，后端使用私钥解密
2. 调度中心配置文件中的 rqlite/Redis 密码使用 RSA 加密存储，避免明文暴露
3. 提供 CLI 命令行工具生成密钥对和加密密码
4. 数据库中密码存储方式不变（bcrypt 哈希）

### 约束

- RSA 加密仅用于传输层（前端→后端）和配置文件存储，不影响数据库存储
- 强制切换，移除 Base64 编码支持
- 前后端统一使用 PKCS#8 格式密钥
- 密钥对配置在调度中心配置文件中

## 配置结构

### config.yaml 新增项

```yaml
rsa:
  public_key: "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQ..."   # PKCS#8公钥（Base64，不含PEM头尾）
  private_key: "MIIEvgIBADANBgkqhkiG9w0BAQEFAASC..."        # PKCS#8私钥（Base64，不含PEM头尾）

database:
  rqlite_addrs: ["http://localhost:4001"]
  rqlite_user: ""
  rqlite_password: "RSA_ENCRYPTED:base64encodedciphertext..."  # RSA加密密文，前缀标识
  rqlite_tls: false

redis:
  mode: "single"
  addr: "localhost:6379"
  password: "RSA_ENCRYPTED:base64encodedciphertext..."         # RSA加密密文，前缀标识
  db: 0
  sentinel_password: "RSA_ENCRYPTED:base64encodedciphertext..." # RSA加密密文，前缀标识
```

### 密钥格式

- 公钥：PKCS#8 格式（`-----BEGIN PUBLIC KEY-----`），配置文件中存储去掉 PEM 头尾和换行的纯 Base64 字符串
- 私钥：PKCS#8 格式（`-----BEGIN PRIVATE KEY-----`），配置文件中存储去掉 PEM 头尾和换行的纯 Base64 字符串
- 加密算法：RSA/ECB/PKCS1Padding（JSEncrypt 默认），Go 后端使用 `rsa.EncryptPKCS1v15` / `rsa.DecryptPKCS1v15`
- 密钥长度：2048 位

### 配置密码前缀规则

- `RSA_ENCRYPTED:` 前缀表示该值为 RSA 加密后的密文
- 无前缀的值视为明文（兼容未加密配置）
- 解密时检测前缀，有则解密，无则原样返回

## 架构设计

### 新增模块：pkg/rsautil

```
pkg/rsautil/
├── rsautil.go        # RSA工具包核心
└── rsautil_test.go   # 单元测试
```

核心接口：

```go
type RSAUtil struct {
    publicKey  *rsa.PublicKey
    privateKey *rsa.PrivateKey
}

func NewFromConfig(publicKeyB64, privateKeyB64 string) (*RSAUtil, error)
func (u *RSAUtil) Encrypt(plaintext string) (string, error)
func (u *RSAUtil) Decrypt(ciphertextB64 string) (string, error)
func (u *RSAUtil) DecryptConfigPassword(password string) (string, error)
func GenerateKeyPair() (publicKeyB64, privateKeyB64 string, err error)
```

- `NewFromConfig`：从 Base64 字符串解析 PKCS#8 密钥对
- `Encrypt`：RSA 加密，返回 Base64 编码密文
- `Decrypt`：RSA 解密，输入 Base64 编码密文
- `DecryptConfigPassword`：检测 `RSA_ENCRYPTED:` 前缀并解密配置密码
- `GenerateKeyPair`：生成 2048 位 RSA 密钥对，返回 PKCS#8 格式 Base64 字符串

### 数据流

#### 用户密码场景

```
前端输入密码
  → JSEncrypt RSA公钥加密（PKCS1v15）
  → HTTP传输Base64密文
  → 后端RSA私钥解密
  → bcrypt哈希比对/生成
  → 存储到数据库
```

#### 配置密码场景

```
管理员运行 ./scheduler encrypt-password
  → RSA公钥加密
  → 写入config.yaml（RSA_ENCRYPTED:前缀）
  → 启动时读取配置
  → 检测RSA_ENCRYPTED:前缀
  → RSA私钥解密
  → 连接rqlite/Redis
```

## 前端变更

### 新增依赖

- `jsencrypt`：RSA 加密库

### password.ts 重写

```typescript
import JSEncrypt from "jsencrypt/bin/jsencrypt.min";

let cachedPublicKey: string | null = null;

export function setPublicKey(key: string) {
  cachedPublicKey = key;
}

export function encryptPassword(txt: string): string {
  if (!cachedPublicKey) {
    throw new Error("公钥未加载");
  }
  const encryptor = new JSEncrypt();
  encryptor.setPublicKey(cachedPublicKey);
  const encrypted = encryptor.getKey().encrypt(txt);
  if (!encrypted) {
    throw new Error("密码加密失败");
  }
  return encrypted;
}
```

### 公钥获取流程

1. `authStore` 新增 `publicKey` 状态和 `fetchPublicKey()` 方法
2. 应用初始化时（Login.vue mounted、App.vue 初始化）调用 `GET /api/auth/public-key`
3. 公钥缓存在内存中，整个应用生命周期复用

### 涉及密码加密的场景（5处）

| 场景 | 文件 | 加密字段 |
|------|------|---------|
| 登录 | `stores/auth.ts` → `login()` | password |
| 注册 | `handler/auth.go` → `Register()` | password |
| 修改密码 | `views/Profile.vue` | old_password, new_password |
| 重置密码 | `views/admin/Users.vue` | new_password |
| 创建用户 | `views/admin/Users.vue` | password |

所有场景统一调用 `encryptPassword()` 替换原有的 `passwordUtils.encodePassword()` (btoa)。

## 后端变更

### config.go

新增字段：

```go
type Config struct {
    // ... 现有字段
    RSAPublicKey  string
    RSAPrivateKey string
}
```

Load 方法新增读取：

```go
RSAPublicKey:  cfg.GetString("rsa.public_key", ""),
RSAPrivateKey: cfg.GetString("rsa.private_key", ""),
```

### main.go

#### 子命令处理

在 `flag.Parse()` 之前检测子命令：

```go
func main() {
    if len(os.Args) > 1 {
        switch os.Args[1] {
        case "keygen":
            runKeygen()
            return
        case "encrypt-password":
            runEncryptPassword()
            return
        case "decrypt-password":
            runDecryptPassword()
            return
        }
    }

    // 原有启动逻辑...
}
```

#### 启动时密码解密

```go
cfg := config.Load(*configFile)

rsaUtil, err := rsautil.NewFromConfig(cfg.RSAPublicKey, cfg.RSAPrivateKey)

cfg.RQLitePass, err = rsaUtil.DecryptConfigPassword(cfg.RQLitePass)
cfg.RedisPassword, err = rsaUtil.DecryptConfigPassword(cfg.RedisPassword)
cfg.RedisSentinelPassword, err = rsaUtil.DecryptConfigPassword(cfg.RedisSentinelPassword)
```

#### 注入私钥到 Handler/Service

- `AuthHandler` 新增 `rsaUtil *rsautil.RSAUtil` 字段
- `UserAdminService` 新增 `rsaUtil *rsautil.RSAUtil` 字段
- 构造函数传入 `rsaUtil`

### auth.go

#### 新增 GetPublicKey 接口

```
GET /api/auth/public-key
无需鉴权
Response: { "code": 0, "data": { "public_key": "MIGfMA0GCSqGSIb3..." } }
```

路由注册在公开路由组（与 login 同级）。

#### Login 方法变更

```go
func (h *AuthHandler) Login(c *gin.Context) {
    // ... 绑定请求

    // RSA解密密码（替换原来的base64解码）
    decryptedPassword, err := h.rsaUtil.Decrypt(req.Password)
    if err != nil {
        Unauthorized(c, "用户名或密码错误")
        return
    }

    // bcrypt比对
    if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(decryptedPassword)); err != nil {
        Unauthorized(c, "用户名或密码错误")
        return
    }
    // ...
}
```

#### Register 方法变更

```go
decryptedPassword, err := h.rsaUtil.Decrypt(req.Password)
if err != nil {
    BadRequest(c, "密码解密失败")
    return
}
hashedPassword, err := bcrypt.GenerateFromPassword([]byte(decryptedPassword), bcrypt.DefaultCost)
```

### user_admin.go / user_admin service

#### 移除 decodePassword (base64)

删除现有的 `decodePassword` 函数。

#### 替换为 RSA 解密

```go
// Service 方法签名变更
func (s *UserAdminService) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
    decryptedOld, err := s.rsaUtil.Decrypt(oldPassword)
    decryptedNew, err := s.rsaUtil.Decrypt(newPassword)
    // ...
}
```

所有涉及密码的方法（CreateUser、ChangePassword、ResetPassword）统一替换。

### requests.go

更新注释，将 `Base64编码` 改为 `RSA加密`。

## CLI 命令行工具

### keygen - 生成密钥对

```bash
./scheduler keygen
```

输出可直接复制到 config.yaml 的密钥对：

```
rsa:
  public_key: "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQ..."
  private_key: "MIIEvgIBADANBgkqhkiG9w0BAQEFAASC..."
```

实现：调用 `rsa.GenerateKey(rand.Reader, 2048)` → 序列化为 PKCS#8 → Base64 编码输出。

### encrypt-password - 加密密码

```bash
./scheduler encrypt-password --config config.yaml --password "mypassword"
```

- 读取配置文件中的公钥
- RSA 加密后输出 `RSA_ENCRYPTED:base64encodedciphertext...`
- 可直接复制到 config.yaml 的 password 字段

### decrypt-password - 解密密码（验证用）

```bash
./scheduler decrypt-password --config config.yaml --ciphertext "RSA_ENCRYPTED:base64encoded..."
```

- 读取配置文件中的私钥
- 解密并输出明文

## 错误处理

| 场景 | 处理方式 |
|------|---------|
| 公钥/私钥格式错误 | 启动时报错退出，提示密钥格式无效 |
| 私钥解密失败（用户密码） | 返回 401 "用户名或密码错误"，不暴露解密细节 |
| 配置密码解密失败 | 启动时报错退出，提示具体配置项解密失败 |
| 公钥未配置 | 启动时报错退出，提示缺少 RSA 密钥配置 |
| 前端公钥未加载 | 前端抛出异常，阻止密码提交 |

## 文件变更清单

### 后端新增

| 文件 | 说明 |
|------|------|
| `pkg/rsautil/rsautil.go` | RSA 工具包 |
| `pkg/rsautil/rsautil_test.go` | 单元测试 |

### 后端修改

| 文件 | 变更 |
|------|------|
| `internal/config/config.go` | 新增 RSAPublicKey/RSAPrivateKey 字段 |
| `cmd/main.go` | 子命令、密码解密、注入 rsaUtil |
| `internal/handler/auth.go` | RSA 解密密码、GetPublicKey 接口 |
| `internal/handler/user_admin.go` | RSA 解密密码 |
| `internal/service/user_admin.go` | decodePassword → RSA 解密 |
| `internal/model/requests.go` | 更新注释 |

### 前端修改

| 文件 | 变更 |
|------|------|
| `package.json` | 新增 jsencrypt 依赖 |
| `src/utils/password.ts` | 重写为 RSA 加密 |
| `src/stores/auth.ts` | 新增 publicKey 状态和 fetchPublicKey |
| `src/views/Login.vue` | 启动时获取公钥，RSA 加密密码 |
| `src/views/Profile.vue` | RSA 加密密码 |
| `src/views/admin/Users.vue` | RSA 加密密码 |
| `src/api/index.ts` | 新增 getPublicKey API |

### 不变

- 数据库密码存储（bcrypt 哈希）
- JWT 令牌机制
- 权限体系
