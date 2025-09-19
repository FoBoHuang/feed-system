-- 初始化数据库
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 创建数据库（如果不存在）
SELECT 'CREATE DATABASE feedsystem'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'feedsystem')\gexec

-- 切换到数据库
\c feedsystem;

-- 创建用户（如果不存在）
DO
$do$
BEGIN
   IF NOT EXISTS (
      SELECT FROM pg_catalog.pg_roles
      WHERE  rolname = 'feeduser') THEN

      CREATE ROLE feeduser LOGIN PASSWORD 'feedpass';
   END IF;
END
$do$;

-- 授予权限
GRANT ALL PRIVILEGES ON DATABASE feedsystem TO feeduser;