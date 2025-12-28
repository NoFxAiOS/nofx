-- ============================================================
-- Crossmint支付系统 - 回滚脚本
-- 版本: 2025-12-28
-- 描述: 安全回滚payment_orders表及其依赖项
-- ============================================================

-- 删除触发器
DROP TRIGGER IF EXISTS update_payment_orders_updated_at ON payment_orders;

-- 删除索引
DROP INDEX IF EXISTS idx_payment_orders_user_status;
DROP INDEX IF EXISTS idx_payment_orders_user_id;
DROP INDEX IF EXISTS idx_payment_orders_crossmint_order_id;
DROP INDEX IF EXISTS idx_payment_orders_status;
DROP INDEX IF EXISTS idx_payment_orders_created_at;

-- 删除表
DROP TABLE IF EXISTS payment_orders;

-- 验证回滚
DO $$
DECLARE
    table_exists BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'payment_orders'
    ) INTO table_exists;

    IF table_exists THEN
        RAISE EXCEPTION '回滚失败：payment_orders表仍然存在';
    END IF;

    RAISE NOTICE 'Crossmint支付系统回滚成功：payment_orders表及其依赖项已删除';
END $$;
