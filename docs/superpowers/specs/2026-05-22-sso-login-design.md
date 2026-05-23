# SSO 第三方登录设计文档

## 概述

新增第三方 SSO 登录功能，默认使用 SSO 登录，通过 URL 参数 `isSso=false` 切换为本地登录。

## 配置

```yaml
sso:
  enabled: true
  url: "http://sso.com/account"
  public_key: "SSO RSA公钥(Base64编码PKCS#8)"
  timeout: 10
```

## 数据流

| 步骤 | SSO 登录 (默认) | 本地登录 (isSso=false) |
|------|----------------|----------------------|
| 前端加密 | SSO公钥加密 | 本地公钥加密 |
| 请求接口 | POST /api/auth/sso-login | POST /api/auth/login |
| 后端验证 | 转发到SSO服务 | 本地RSA解密 + bcrypt比对 |
| 用户创建 | SSO成功后自动创建(如不存在) | 不自动创建 |
| 密码存储 | 空 | bcrypt哈希 |

## 后端变更

1. config.go: 新增 SSOEnabled/SSOUrl/SSOPublicKey/SSOTimeout
2. auth.go: 新增 SSOLogin handler，GetPublicKey 返回双公钥
3. main.go: 初始化 SSO RSAUtil，注册路由

## 前端变更

1. password.ts: SSO公钥独立缓存和加密
2. api/index.ts: 新增 ssoLogin，getPublicKey 返回 sso_public_key
3. stores/auth.ts: 新增 ssoLogin 方法
4. Login.vue: 根据 isSso 参数切换登录模式

## SSO 登录流程

1. 前端用 SSO 公钥加密密码，POST /api/auth/sso-login
2. 后端原样转发 {loginName, password} 到 SSO 服务
3. code=3000 → 查找/创建本地用户 → 生成 JWT
4. code=3001 → 返回 SSO 错误信息
5. 网络错误 → "SSO登录失败，请稍后再试"

## 自动创建用户规则

- username = SSO loginName
- real_name = SSO idCardName
- phone = SSO mobileNo
- email = SSO email
- role = user
- password = 空
