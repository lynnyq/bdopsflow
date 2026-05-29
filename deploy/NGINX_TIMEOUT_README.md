
# Nginx 查询接口超时配置

## 问题背景
Hive 等大数据查询可能执行时间超过 1 分钟，导致 nginx 504 超时错误。

## 配置文件说明

### 1. `nginx.conf` - 完整版配置
包含完整的 nginx 配置，适用于全新部署。

### 2. `nginx-query-timeout.conf` - 查询接口专用配置
仅包含查询相关接口的超时配置，适合在已有配置上添加。

## 核心超时配置

| 接口 | 超时时间 | 说明 |
|-----|--------|------|
| `/api/query/execute` | 30 分钟 | 查询执行接口，支持慢查询 |
| `/api/query/export` | 30 分钟 | 数据导出接口 |
| `/api/datasource/metadata` | 5 分钟 | 元数据查询（表/列等） |
| 其他 API | 5 分钟 | 通用接口 |

## 关键参数说明

```nginx
# 连接超时：建立 TCP 连接的超时时间
proxy_connect_timeout 60s;

# 发送超时：向后端发送请求的超时时间
proxy_send_timeout 1800s;  # 30分钟

# 读取超时：从后端读取响应的超时时间
proxy_read_timeout 1800s;  # 30分钟
```

## 部署步骤

### 方式一：使用简化配置（推荐）
```bash
# 1. 备份原配置
cp /etc/nginx/conf.d/default.conf /etc/nginx/conf.d/default.conf.bak

# 2. 复制配置
cp nginx-query-timeout.conf /etc/nginx/conf.d/bdopsflow.conf

# 3. 检查配置
nginx -t

# 4. 重载配置
nginx -s reload
```

### 方式二：修改现有配置
在现有 nginx 配置中添加以下 location 块：

```nginx
# 添加在 server 块内
server {
    # ... 现有配置 ...
    
    # 查询执行接口 - 30分钟超时
    location ~ ^/api/v?\d*/query/execute$ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        
        proxy_connect_timeout 60s;
        proxy_send_timeout 1800s;
        proxy_read_timeout 1800s;
        proxy_request_buffering off;
        proxy_buffering off;
    }
    
    # ... 其他接口配置 ...
}
```

## 验证配置

### 1. 检查语法
```bash
nginx -t
```

### 2. 测试超时
执行一个耗时较长的查询（例如 `SELECT sleep(120)`），确认不会出现 504 错误。

### 3. 查看日志
```bash
tail -f /var/log/nginx/error.log
tail -f /var/log/nginx/access.log
```

## 注意事项

1. **后端服务也需要相应配置**：确保后端（scheduler）的查询超时设置也足够长
2. **生产环境建议**：30分钟足够长，但根据实际业务调整
3. **监控**：建议监控慢查询，避免恶意查询占用资源
4. **取消查询**：确保取消查询功能正常，避免查询堆积

## 后端超时配置参考

在 `scheduler/config.yaml` 中确保：

```yaml
datasource:
  query_timeout: 3600  # 查询超时（秒）
```

## 故障排查

### 仍然报 504 错误
- 检查是否正确重载了 nginx 配置
- 检查 location 匹配顺序（更具体的 location 放在前面）
- 确认后端服务没有超时

### 查询被取消
- 检查是否有其他代理或防火墙限制
- 确认后端查询超时配置足够长

