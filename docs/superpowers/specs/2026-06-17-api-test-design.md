# 接口测试模块设计文档

> 日期：2026-06-17
> 状态：已确认

## 1. 概述

新增"接口测试"功能模块，包含 HTTP 接口测试和 gRPC 接口测试两大功能。采用单模块整合架构，HTTP 和 gRPC 共享保存用例、权限、响应历史等基础设施，proto 文件和证书管理独立建表但归属同一模块。

**核心特性：**
- HTTP 接口测试：参考 Postman，支持全 HTTP 方法、多种请求体、认证配置、前置/后置脚本、一键生成 curl 命令
- gRPC 接口测试：支持 proto 文件上传 + gRPC Server Reflection 双模式、TLS 三种模式（insecure/TLS/mTLS）、证书管理
- 测试用例保存：全部私有，按用户完全隔离，无共享机制
- 响应历史：记录每次执行结果，支持断言和对比

**请求执行模式：** 后端代理请求（前端将请求参数发给后端，后端代理发起实际请求并返回结果）

## 2. 数据模型

### 2.1 测试用例表 `bdopsflow_api_tests`

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PK AUTOINCREMENT | 主键 |
| name | TEXT | NOT NULL | 用例名称 |
| type | TEXT | NOT NULL | `http` 或 `grpc` |
| config | TEXT | NOT NULL | JSON 存储完整请求配置 |
| created_by | INTEGER | NOT NULL | 创建者（所有数据按用户隔离） |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 更新时间 |

**config 字段 JSON 结构：**

HTTP 类型：
```json
{
  "method": "POST",
  "url": "https://api.example.com/users",
  "headers": [{"key": "Content-Type", "value": "application/json"}],
  "params": [{"key": "page", "value": "1"}],
  "body": {
    "type": "json",
    "content": "{\"name\": \"test\"}"
  },
  "auth": {
    "type": "bearer",
    "token": "xxx"
  },
  "pre_script": "",
  "post_script": "",
  "timeout": 30
}
```

gRPC 类型：
```json
{
  "address": "localhost:50051",
  "service": "UserService",
  "method": "GetUser",
  "request_body": "{\"id\": 1}",
  "metadata": [{"key": "authorization", "value": "Bearer xxx"}],
  "tls_mode": "insecure",
  "certificate_id": null,
  "proto_file_id": 1,
  "use_reflection": false,
  "timeout": 30
}
```

### 2.2 Proto 文件表 `bdopsflow_proto_files`

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PK AUTOINCREMENT | 主键 |
| name | TEXT | NOT NULL | 文件名 |
| content | TEXT | NOT NULL | proto 文件内容 |
| file_hash | TEXT | NOT NULL | 文件哈希（去重） |
| parsed_result | TEXT | | 解析结果缓存（JSON：package/services/messages） |
| dependencies | TEXT | DEFAULT '[]' | 依赖的其他 proto 文件 ID 列表（JSON 数组） |
| created_by | INTEGER | NOT NULL | 创建者（所有数据按用户隔离） |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 更新时间 |

### 2.3 证书文件表 `bdopsflow_certificates`

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PK AUTOINCREMENT | 主键 |
| name | TEXT | NOT NULL | 证书名称 |
| ca_cert | TEXT | | CA 证书内容（PEM） |
| client_cert | TEXT | | 客户端证书内容（PEM） |
| client_key | TEXT | | 客户端私钥内容（PEM，加密存储） |
| created_by | INTEGER | NOT NULL | 创建者（所有数据按用户隔离） |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 更新时间 |

### 2.4 响应历史表 `bdopsflow_api_test_results`

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PK AUTOINCREMENT | 主键 |
| test_id | INTEGER | FK | 关联测试用例 ID |
| type | TEXT | NOT NULL | `http` 或 `grpc` |
| status_code | INTEGER | | HTTP 状态码 / gRPC status code |
| latency_ms | INTEGER | | 响应耗时(ms) |
| headers | TEXT | | 响应头（JSON） |
| body | TEXT | | 响应体 |
| error | TEXT | | 错误信息 |
| assertions_result | TEXT | | 断言结果（JSON） |
| executed_by | INTEGER | | 执行者 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 执行时间 |

### 2.5 索引

```sql
CREATE INDEX IF NOT EXISTS idx_bdopsflow_api_tests_type ON bdopsflow_api_tests(type);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_api_tests_created_by ON bdopsflow_api_tests(created_by);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_api_tests_name_user ON bdopsflow_api_tests(name, created_by);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_proto_files_created_by ON bdopsflow_proto_files(created_by);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_proto_files_name_user ON bdopsflow_proto_files(name, created_by);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_proto_files_file_hash ON bdopsflow_proto_files(file_hash);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_certificates_created_by ON bdopsflow_certificates(created_by);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_certificates_name_user ON bdopsflow_certificates(name, created_by);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_api_test_results_test_id ON bdopsflow_api_test_results(test_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_api_test_results_type ON bdopsflow_api_test_results(type);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_api_test_results_executed_by ON bdopsflow_api_test_results(executed_by);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_api_test_results_created_at ON bdopsflow_api_test_results(created_at DESC);
```

## 3. 后端 API 设计

### 3.1 接口测试 API（`/api/api-tests`）

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/api-tests` | 列表（支持 type/domain 过滤） | api_test:read |
| POST | `/api/api-tests` | 创建测试用例 | api_test:create |
| GET | `/api/api-tests/:id` | 获取用例详情 | api_test:read |
| PUT | `/api/api-tests/:id` | 更新用例 | api_test:update |
| DELETE | `/api/api-tests/:id` | 删除用例 | api_test:delete |
| POST | `/api/api-tests/execute` | 执行测试（不保存，临时请求） | api_test:execute |
| POST | `/api/api-tests/:id/execute` | 执行已保存的用例 | api_test:execute |
| GET | `/api/api-tests/:id/results` | 获取用例的响应历史 | api_test:read |
| DELETE | `/api/api-tests/results/:id` | 删除响应历史 | api_test:delete |
| POST | `/api/api-tests/generate-curl` | 根据 HTTP 配置生成 curl 命令 | api_test:execute |

### 3.2 Proto 文件 API（`/api/proto-files`）

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/proto-files` | 列表 | api_test:read |
| POST | `/api/proto-files` | 上传 proto 文件 | api_test:create |
| GET | `/api/proto-files/:id` | 获取文件内容 | api_test:read |
| PUT | `/api/proto-files/:id` | 更新 proto 文件 | api_test:update |
| DELETE | `/api/proto-files/:id` | 删除 | api_test:delete |
| POST | `/api/proto-files/parse` | 解析 proto 文件返回服务/方法列表 | api_test:execute |
| POST | `/api/proto-files/reflect` | 通过 gRPC Reflection 发现服务 | api_test:execute |

### 3.3 证书管理 API（`/api/certificates`）

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/certificates` | 列表（私钥不返回） | api_test:read |
| POST | `/api/certificates` | 上传证书 | api_test:create |
| GET | `/api/certificates/:id` | 获取证书详情（私钥不返回） | api_test:read |
| PUT | `/api/certificates/:id` | 更新证书 | api_test:update |
| DELETE | `/api/certificates/:id` | 删除 | api_test:delete |

### 3.4 权限定义

新增 `api_test` 资源权限：

| resource | action | 说明 |
|----------|--------|------|
| api_test | create | 创建测试用例/proto/证书 |
| api_test | read | 查看测试用例/proto/证书 |
| api_test | update | 更新测试用例/proto/证书 |
| api_test | delete | 删除测试用例/proto/证书 |
| api_test | execute | 执行测试、解析 proto、gRPC Reflection |
| api_test | manage | 完整管理 |

角色权限分配：
- 系统管理员：全部 api_test 权限
- 领域管理员：create/read/update/delete/execute/manage
- 普通用户：无任何 api_test 权限（前端不显示入口）

> 注：普通用户前端默认不显示"接口测试"菜单入口。只有领域管理员及以上角色才能看到并使用此模块。

### 3.5 数据可见性规则

**核心原则：整个接口测试模块按用户完全隔离，无共享机制。**

所有数据（测试用例、Proto 文件、证书、响应历史）仅对创建者本人可见：

- 系统管理员：可查看所有用户的数据
- 领域管理员：仅可查看自己创建的数据（`created_by = 当前用户ID`）
- 普通用户：无权限访问此模块

**具体规则：**
- 测试用例：仅 `created_by = 当前用户ID` 的记录可见
- Proto 文件：仅 `created_by = 当前用户ID` 的记录可见
- 证书：仅 `created_by = 当前用户ID` 的记录可见
- 响应历史：仅 `executed_by = 当前用户ID` 的记录可见
- 更新/删除操作：仅可操作自己创建的资源

## 4. 后端核心逻辑

### 4.1 HTTP 请求执行器（`service/http_executor.go`）

- 使用 Go `net/http` 构建请求，支持 GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS
- 请求体类型：
  - `json`：JSON 格式化，Content-Type 自动设为 application/json
  - `form-urlencoded`：表单键值对，Content-Type 自动设为 application/x-www-form-urlencoded
  - `form-multipart`：多部分表单（支持文件上传），Content-Type 自动设为 multipart/form-data
  - `raw`：原始文本，自定义 Content-Type
  - `binary`：二进制文件，base64 编码传输，后端解码
- 认证支持：
  - `none`：无认证
  - `bearer`：Bearer Token
  - `basic`：Basic Auth（用户名+密码）
  - `apikey`：API Key（支持 header/query 两种位置）
- 前置脚本：使用 Go `text/template` 做变量替换，内置函数：`timestamp`、`uuid`、`randomInt`
- 后置脚本：断言检查
  - 状态码断言：等于/不等于/包含
  - JSON Path 值断言：等于/不等于/包含/大于/小于
  - 响应头断言：存在/等于
- curl 生成：根据请求配置拼接完整 curl 命令，包含方法、URL、headers、body
- 超时控制：默认 30s，可配置，最长 300s
- 响应体大小限制：默认 10MB

### 4.2 gRPC 请求执行器（`service/grpc_executor.go`）

- proto 文件解析：使用 `protobuf` 库解析 `.proto` 文件，提取 service/method/消息定义
- 支持多文件依赖：通过 dependencies 字段关联其他 proto 文件
- gRPC Reflection：连接目标服务，通过 `grpc_reflection_v1` 自动发现服务列表和方法
- TLS 模式：
  - `insecure`：`grpc.WithTransportCredentials(insecure.NewCredentials())`
  - `tls`：`grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))`，使用系统 CA
  - `mtls`：加载 CA 证书 + 客户端证书 + 私钥构建 `tls.Config`
- 请求构建：用户填写 JSON 格式请求体，后端通过 `protojson` 转换为 protobuf 消息
- 响应处理：将 protobuf 响应转为 JSON 返回前端，同时返回 gRPC status code 和 trailer metadata
- 超时控制：默认 30s，可配置

### 4.3 Proto 文件管理（`service/proto_service.go`）

- 上传时计算文件 SHA256 hash 去重
- 解析 proto 文件提取 package/service/message 定义，缓存到 parsed_result 字段
- 支持多文件依赖：上传时指定依赖的其他 proto 文件 ID
- proto 文件内容存储在数据库中（适合分布式部署）
- 解析接口返回结构化的服务/方法列表供前端展示

### 4.4 证书管理（`service/certificate_service.go`）

- 客户端私钥使用项目已有的 RSA 加密机制加密存储
- 列表/详情接口不返回私钥明文
- 执行 gRPC 请求时解密私钥构建 TLS 配置
- 证书内容以 PEM 格式存储

## 5. 前端设计

### 5.1 菜单结构

在 `menuPermissionMap.ts` 中新增：

```
接口测试 (key: api-test, icon: Monitor, resources: ['api_test'])
  ├── HTTP 测试     (key: api-test-http, path: /api-test/http, resources: ['api_test'])
  ├── gRPC 测试     (key: api-test-grpc, path: /api-test/grpc, resources: ['api_test'])
  ├── Proto 文件    (key: api-test-proto, path: /api-test/proto-files, resources: ['api_test'])
  └── 证书管理      (key: api-test-cert, path: /api-test/certificates, resources: ['api_test'])
```

> 前端权限控制：整个"接口测试"菜单仅对有 `api_test` 资源权限的用户（领域管理员及以上）显示。普通用户前端不显示此菜单入口。所有数据按用户隔离，每个用户只能看到和操作自己创建的资源。

### 5.2 HTTP 测试页面（`views/ApiTest/HttpTest.vue`）

**布局**：左右分栏，类似 Postman

- **左侧面板**：已保存的用例列表树，支持搜索、新建、删除
- **右侧面板**：请求编辑区 + 响应展示区（上下分栏）
  - 请求编辑区：
    - 顶部：方法选择器 + URL 输入框 + 发送按钮 + 保存按钮 + 生成 Curl 按钮
    - Tab 切换：Params | Headers | Body | Auth | Pre-Script | Post-Script
    - Body Tab 内按类型切换：none / JSON / Form-Urlencoded / Form-Multipart / Raw / Binary
    - Auth Tab：认证类型选择 + 对应配置表单
  - 响应展示区：
    - 顶部：状态码 + 耗时 + 大小
    - Tab 切换：Body (JSON/Raw/Preview) | Headers | Assertions | History
    - History Tab：历史响应列表，点击可查看，支持并排对比（选择两次响应，左右展示差异）

### 5.3 gRPC 测试页面（`views/ApiTest/GrpcTest.vue`）

**布局**：与 HTTP 测试类似的左右分栏

- **左侧面板**：已保存的用例列表
- **右侧面板**：
  - 顶部：服务地址输入 + 连接方式选择（Proto 文件 / Server Reflection）+ 发送按钮 + 保存按钮
  - Tab 切换：Service | Request | Metadata | TLS
  - Service Tab：
    - Proto 文件模式：选择已上传的 proto 文件 → 展示服务/方法树 → 选择方法
    - Reflection 模式：输入地址后点击"发现服务" → 展示服务/方法树
  - Request Tab：JSON 编辑器（CodeMirror 6）填写请求体
  - Metadata Tab：键值对编辑 gRPC metadata
  - TLS Tab：模式选择（insecure/TLS/mTLS）+ 证书选择
  - 响应展示区：JSON 响应 + gRPC status + metadata

### 5.4 Proto 文件管理页面（`views/ApiTest/ProtoFiles.vue`）

- 列表展示：文件名、包名、服务数、创建时间
- 数据可见性：仅展示当前用户上传的 proto 文件
- 操作：上传新文件、编辑内容、删除
- 支持多文件批量上传

### 5.5 证书管理页面（`views/ApiTest/Certificates.vue`）

- 列表展示：名称、类型（CA/客户端证书/完整 mTLS）、创建时间
- 数据可见性：仅展示当前用户上传的证书
- 操作：新增（上传 CA 证书、客户端证书、客户端私钥）、编辑、删除
- 私钥字段始终以密码形式展示

### 5.6 前端 API 层

新增 `web/src/api/apiTest.ts`，封装所有接口测试相关 API。

## 6. 文件结构

### 6.1 后端新增文件

```
scheduler/internal/
├── handler/
│   ├── api_test.go          # 接口测试 handler
│   ├── proto_file.go        # Proto 文件 handler
│   └── certificate.go       # 证书管理 handler
├── service/
│   ├── api_test_service.go  # 测试用例 CRUD + 执行
│   ├── http_executor.go     # HTTP 请求执行器
│   ├── grpc_executor.go     # gRPC 请求执行器
│   ├── proto_service.go     # Proto 文件管理
│   └── certificate_service.go # 证书管理
└── model/
    ├── api_test.go          # 测试用例模型
    ├── proto_file.go        # Proto 文件模型
    └── certificate.go       # 证书模型
```

### 6.2 前端新增文件

```
web/src/
├── api/
│   └── apiTest.ts           # 接口测试 API 封装
└── views/
    └── ApiTest/
        ├── HttpTest.vue     # HTTP 测试页面
        ├── GrpcTest.vue     # gRPC 测试页面
        ├── ProtoFiles.vue   # Proto 文件管理
        └── Certificates.vue # 证书管理
```

### 6.3 数据库变更

- `deploy/schema.sql`：新增 4 张表
- `deploy/migrations/v4_api_test.sql`：迁移脚本

## 7. 新增依赖

| 依赖 | 用途 |
|------|------|
| `google.golang.org/protobuf` | protobuf 消息解析 |
| `google.golang.org/grpc/reflection` | gRPC Reflection 客户端 |
| `github.com/jhump/protoreflect` | 动态 protobuf 消息构建（无需生成 Go 代码即可调用任意 gRPC 方法） |

## 8. 测试策略

- 后端单元测试：覆盖 HTTP 执行器、gRPC 执行器、proto 解析、证书加解密
- 后端集成测试：启动真实 HTTP/gRPC 服务进行端到端测试
- 前端组件测试：覆盖请求配置表单、响应展示、用例保存等核心交互
