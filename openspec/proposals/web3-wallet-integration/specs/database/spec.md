# Web3 Wallet Database Specification

## Overview

This document specifies the database schema for Web3 wallet support in Monnaire Trading Agent OS. The design follows a normalized approach with proper foreign key constraints and indexes for optimal performance.

## Tables

### 1. web3_wallets

Stores validated Ethereum wallet addresses.

#### Schema

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | TEXT | PRIMARY KEY | Unique identifier (UUID) |
| wallet_addr | TEXT | UNIQUE, NOT NULL | Ethereum address (0x...) |
| chain_id | INTEGER | NOT NULL, DEFAULT 1 | Blockchain network ID |
| wallet_type | TEXT | NOT NULL | Wallet provider type |
| label | TEXT | NULL | User-defined label |
| is_active | BOOLEAN | DEFAULT TRUE | Whether wallet is active |
| created_at | TIMESTAMPTZ | DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | DEFAULT NOW() | Last update timestamp |

#### Constraints

```sql
-- Address format validation (Ethereum)
CONSTRAINT chk_wallet_addr
CHECK (wallet_addr ~ '^0x[a-fA-F0-9]{40}$')

-- Chain ID validation
CONSTRAINT chk_chain_id
CHECK (chain_id > 0)

-- Valid wallet types
CONSTRAINT chk_wallet_type
CHECK (wallet_type IN ('metamask', 'tp', 'other'))
```

#### Indexes

```sql
-- Primary lookup index
CREATE INDEX idx_web3_wallets_addr ON web3_wallets(wallet_addr);

-- Filter by type
CREATE INDEX idx_web3_wallets_type ON web3_wallets(wallet_type);

-- Active wallets
CREATE INDEX idx_web3_wallets_active ON web3_wallets(is_active);
```

#### Example Data

```sql
INSERT INTO web3_wallets (
    id, wallet_addr, chain_id, wallet_type, label, is_active
) VALUES (
    'w_001',
    '0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0',
    1,
    'metamask',
    'My MetaMask',
    true
);
```

### 2. user_wallets

Association table linking users to their wallet addresses.

#### Schema

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | TEXT | PRIMARY KEY | Unique identifier (UUID) |
| user_id | TEXT | NOT NULL, FK users(id) | Reference to user |
| wallet_addr | TEXT | NOT NULL, FK web3_wallets(wallet_addr) | Reference to wallet |
| is_primary | BOOLEAN | DEFAULT FALSE | Primary wallet flag |
| bound_at | TIMESTAMPTZ | DEFAULT NOW() | When wallet was bound |
| last_used_at | TIMESTAMPTZ | DEFAULT NOW() | Last authentication time |

#### Constraints

```sql
-- Unique user-wallet pair
UNIQUE(user_id, wallet_addr)

-- One primary wallet per user
CONSTRAINT chk_is_primary
CHECK (
    CASE
        WHEN is_primary = TRUE THEN
            NOT EXISTS (
                SELECT 1 FROM user_wallets uw2
                WHERE uw2.user_id = user_wallets.user_id
                AND uw2.is_primary = TRUE
                AND uw2.wallet_addr != user_wallets.wallet_addr
            )
        ELSE TRUE
    END
)
```

#### Indexes

```sql
-- User wallet lookup
CREATE INDEX idx_user_wallets_user_id ON user_wallets(user_id);

-- Primary wallet search
CREATE INDEX idx_user_wallets_primary ON user_wallets(user_id, is_primary);

-- Wallet binding search
CREATE INDEX idx_user_wallets_addr ON user_wallets(wallet_addr);

-- Recently used
CREATE INDEX idx_user_wallets_last_used ON user_wallets(last_used_at);
```

#### Example Data

```sql
INSERT INTO user_wallets (
    id, user_id, wallet_addr, is_primary, bound_at
) VALUES (
    'uw_001',
    'user_123',
    '0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0',
    true,
    NOW()
);
```

## Relationships

### ER Diagram

```
users (1) ----< (N) user_wallets (N) >---- (1) web3_wallets
   |                                           |
   |                                           |
id |                                  wallet_addr
   |                                           |
   └── user_id (FK)                           └── id
```

### Foreign Keys

```sql
-- user_wallets.user_id → users.id
ALTER TABLE user_wallets
ADD CONSTRAINT fk_user_wallets_user_id
FOREIGN KEY (user_id) REFERENCES users(id)
ON DELETE CASCADE;

-- user_wallets.wallet_addr → web3_wallets.wallet_addr
ALTER TABLE user_wallets
ADD CONSTRAINT fk_user_wallets_wallet_addr
FOREIGN KEY (wallet_addr) REFERENCES web3_wallets(wallet_addr)
ON DELETE CASCADE;
```

## Triggers

### Auto-update updated_at

```sql
CREATE TRIGGER update_web3_wallets_updated_at
    BEFORE UPDATE ON web3_wallets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

### Update last_used_at

```sql
CREATE OR REPLACE FUNCTION update_last_used_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_used_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_user_wallets_last_used_at
    AFTER UPDATE ON user_wallets
    FOR EACH ROW EXECUTE FUNCTION update_last_used_at();
```

## System Configuration

Add supported wallet types to system_config:

```sql
INSERT INTO system_config (key, value) VALUES
    ('web3.supported_wallet_types', '["metamask", "tp", "other"]'),
    ('web3.max_wallets_per_user', '10'),
    ('web3.nonce_expiry_minutes', '10')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
```

## Common Queries

### Get all wallets for a user

```sql
SELECT
    w.wallet_addr,
    w.wallet_type,
    w.label,
    uw.is_primary,
    uw.bound_at,
    uw.last_used_at
FROM user_wallets uw
JOIN web3_wallets w ON uw.wallet_addr = w.wallet_addr
WHERE uw.user_id = $1
ORDER BY uw.is_primary DESC, uw.bound_at DESC;
```

### Check if address is already bound

```sql
SELECT user_id, is_primary
FROM user_wallets
WHERE wallet_addr = $1;
```

### Get user by wallet address

```sql
SELECT u.id, u.email, uw.is_primary
FROM users u
JOIN user_wallets uw ON u.id = uw.user_id
WHERE uw.wallet_addr = $1;
```

### Set primary wallet

```sql
BEGIN;

-- Remove primary flag from all other wallets
UPDATE user_wallets
SET is_primary = false
WHERE user_id = $1;

-- Set new primary wallet
UPDATE user_wallets
SET is_primary = true
WHERE user_id = $1 AND wallet_addr = $2;

COMMIT;
```

### Unlink wallet

```sql
DELETE FROM user_wallets
WHERE user_id = $1 AND wallet_addr = $2;
```

## Migration Script

### Up Script

See: `database/migrations/20251201_add_web3_wallets.sql`

### Down Script

See: `database/migrations/20251201_rollback_web3_wallets.sql`

## Performance Considerations

### Query Optimization

1. **Use covering indexes** for frequent queries
2. **Avoid SELECT *** in wallet lookups
3. **Use prepared statements** for repeated queries
4. **Connection pooling** for concurrent access

### Index Usage

- `idx_web3_wallets_addr`: Used for address lookups
- `idx_user_wallets_user_id`: Used for user wallet lists
- `idx_user_wallets_primary`: Used for primary wallet queries

### Caching Strategy

```sql
-- Cache wallet data for 5 minutes
-- Example application-level caching:
SELECT * FROM web3_wallets WHERE wallet_addr = $1
-- Cache result for 300 seconds
```

## Backup and Recovery

### Backup

```bash
# Backup specific tables
pg_dump $DATABASE_URL \
  --table=web3_wallets \
  --table=user_wallets \
  --data-only > web3_wallets_backup.sql
```

### Recovery

```bash
# Restore from backup
psql $DATABASE_URL < web3_wallets_backup.sql
```

## Data Retention

- **Active wallets**: Retained indefinitely
- **Unlinked wallets**: Removed automatically by CASCADE
- **Audit logs**: Retain for 2 years (configured separately)

## Security

### Data Protection

1. **No private keys stored**: Only public addresses
2. **Signature verification**: On-the-fly, not persisted
3. **Encrypted storage**: Use database encryption at rest
4. **Access control**: Role-based permissions

### Audit Trail

All wallet operations should be logged to `audit_logs` table:

```sql
-- Example audit logging
INSERT INTO audit_logs (
    id, user_id, action, ip_address, success, details
) VALUES (
    gen_random_uuid(),
    $1,
    'WALLET_LINK',
    $2,
    true,
    json_build_object('wallet_addr', $3, 'wallet_type', $4)
);
```

## Testing

### Test Data

```sql
-- Create test wallets
INSERT INTO web3_wallets (id, wallet_addr, wallet_type) VALUES
('w_test_1', '0x1111111111111111111111111111111111111111', 'metamask'),
('w_test_2', '0x2222222222222222222222222222222222222222', 'tp');

-- Link to test user
INSERT INTO user_wallets (id, user_id, wallet_addr, is_primary) VALUES
('uw_test_1', 'default', '0x1111111111111111111111111111111111111111', true);
```

### Test Queries

```sql
-- Test 1: Verify wallet lookup performance
EXPLAIN ANALYZE
SELECT * FROM web3_wallets
WHERE wallet_addr = '0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0';

-- Test 2: Verify user wallet list performance
EXPLAIN ANALYZE
SELECT * FROM user_wallets
WHERE user_id = 'user_123'
ORDER BY is_primary DESC, bound_at DESC;

-- Test 3: Verify primary wallet constraint
INSERT INTO user_wallets (id, user_id, wallet_addr, is_primary)
VALUES ('test', 'user_123', '0x2222222222222222222222222222222222222222', true);
-- Should fail if user_123 already has a primary wallet
```

## Maintenance

### Regular Tasks

1. **Weekly**: Analyze table statistics
2. **Monthly**: Reindex if needed
3. **Quarterly**: Review and optimize slow queries
4. **Annually**: Archive old audit logs

### Monitoring

- Track query performance
- Monitor index usage
- Watch for deadlocks
- Alert on constraint violations

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-12-01 | Initial schema |
