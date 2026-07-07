-- BDopsFlow 迁移脚本
-- 版本：v7
-- 日期：2026-07-07
-- 描述：已保存SQL表增加 database 列，记录保存时使用的数据库

-- 新增 database 列
-- SQLite 支持 ALTER TABLE ADD COLUMN，重复执行会报错可忽略
ALTER TABLE bdopsflow_saved_sql ADD COLUMN database TEXT;
