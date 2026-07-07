-- BDopsFlow 迁移脚本
-- 版本：v5
-- 日期：2026-07-06
-- 描述：系统配置项同步与对齐
--   1. 新增缺失配置项（api_test.allow_private_network、datasource.max_concurrent_per_datasource、
--      datasource.metadata_timeout、wecom.app_msg_url、wecom.ewechat_url）
--   2. 修正 connection_max_idle / connection_max_open 默认值与代码一致
--   3. 补充 audit_log.retention_days 元数据（该项此前仅 schema 初始化，未纳入动态配置体系）
--
-- 说明：
-- - 使用 INSERT OR IGNORE，仅对未存在的配置项进行初始化，已存在的配置值保持不变
-- - 对 connection_max_idle/max_open 的 UPDATE 仅在值仍为旧默认值时生效，避免覆盖用户自定义
-- - 配置项的元数据（label/description/type/min/max/unit/group）由代码中 configMetaList 提供

-- 新增缺失配置项
INSERT OR IGNORE INTO bdopsflow_system_config (config_key, config_value, description, updated_at) VALUES
('api_test.allow_private_network', 'false', '是否允许HTTP/gRPC接口测试访问内网地址（SSRF防护）', datetime('now')),
('datasource.max_concurrent_per_datasource', '10', '单数据源并发查询限制', datetime('now')),
('datasource.metadata_timeout', '60', '元数据查询超时时间(秒)', datetime('now')),
('wecom.app_msg_url', 'https://qyapi.weixin.qq.com/cgi-bin/app/send', '企业微信应用消息URL', datetime('now')),
('wecom.ewechat_url', 'https://qyapi.weixin.qq.com/cgi-bin/app/send', '企业微信网关URL', datetime('now'));

-- 修正连接池默认值（仅当仍为旧默认值时更新，避免覆盖用户已修改的配置）
UPDATE bdopsflow_system_config
SET config_value = '2', description = '连接池最小空闲连接数', updated_at = datetime('now')
WHERE config_key = 'datasource.connection_max_idle' AND config_value = '5';

UPDATE bdopsflow_system_config
SET config_value = '5', description = '连接池最大打开连接数', updated_at = datetime('now')
WHERE config_key = 'datasource.connection_max_open' AND config_value = '10';

-- 修正 audit_log.retention_days 描述
UPDATE bdopsflow_system_config
SET description = '审计日志保留天数', updated_at = datetime('now')
WHERE config_key = 'audit_log.retention_days';
