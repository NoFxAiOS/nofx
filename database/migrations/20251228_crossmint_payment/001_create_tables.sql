-- ============================================================
-- Crossmint支付系统 - 数据库迁移
-- 版本: 2025-12-28
-- 描述: 创建支付订单表，支持Crossmint集成
-- ============================================================

-- ============================================================
-- 1. 支付订单表 (payment_orders)
-- 存储Crossmint支付订单记录，防止重复处理webhook
-- ============================================================
CREATE TABLE IF NOT EXISTS payment_orders (
    id TEXT PRIMARY KEY,
    crossmint_order_id TEXT UNIQUE,
    user_id TEXT NOT NULL,
    package_id TEXT NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    currency TEXT NOT NULL DEFAULT 'USDT',
    credits INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    payment_method TEXT,
    crossmint_client_secret TEXT,
    webhook_received_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    failed_reason TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (package_id) REFERENCES credit_packages(id),
    CONSTRAINT chk_status CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'cancelled', 'refunded')),
    CONSTRAINT chk_amount_positive CHECK (amount > 0),
    CONSTRAINT chk_credits_positive CHECK (credits > 0),
    CONSTRAINT chk_currency CHECK (currency IN ('USDT', 'USDC', 'ETH', 'BTC'))
);

-- ============================================================
-- 索引 - 优化查询性能
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_payment_orders_user_id
    ON payment_orders(user_id);

CREATE INDEX IF NOT EXISTS idx_payment_orders_crossmint_order_id
    ON payment_orders(crossmint_order_id);

CREATE INDEX IF NOT EXISTS idx_payment_orders_status
    ON payment_orders(status);

CREATE INDEX IF NOT EXISTS idx_payment_orders_created_at
    ON payment_orders(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_payment_orders_user_status
    ON payment_orders(user_id, status);

-- ============================================================
-- 触发器 (复用现有 update_updated_at_column 函数)
-- ============================================================
DROP TRIGGER IF EXISTS update_payment_orders_updated_at ON payment_orders;
CREATE TRIGGER update_payment_orders_updated_at
    BEFORE UPDATE ON payment_orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- 验证迁移
-- ============================================================
DO $$
DECLARE
    table_exists BOOLEAN;
    index_count INTEGER;
BEGIN
    -- 验证表是否存在
    SELECT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'payment_orders'
    ) INTO table_exists;

    IF NOT table_exists THEN
        RAISE EXCEPTION 'Crossmint支付迁移失败：payment_orders表未创建';
    END IF;

    -- 验证索引数量
    SELECT COUNT(*) INTO index_count
    FROM pg_indexes
    WHERE tablename = 'payment_orders';

    IF index_count < 5 THEN
        RAISE EXCEPTION 'Crossmint支付迁移失败：索引数量不足（预期5个，实际%个）', index_count;
    END IF;

    RAISE NOTICE 'Crossmint支付系统迁移成功：payment_orders表已创建，%个索引已生成', index_count;
END $$;
