# Web3 Authentication Specification

## Overview

This document specifies the Web3 wallet authentication system for Monnaire Trading Agent OS, following EIP-191 signature standard.

## Authentication Flow

### 1. Wallet Connection Flow

```
User Action → Wallet Detection → Address Retrieval → Nonce Generation
     ↓
Signature Request → User Signature → Server Verification → Token Generation
     ↓
Wallet Link → User Account Link → Success Response
```

### 2. Message Format

All signature messages follow this template:

```
Monnaire Trading Agent OS - Web3 Authentication

Wallet Address: {address}
Nonce: {nonce}
Timestamp: {timestamp}

This request will not trigger a blockchain transaction or cost any gas fees.

Signature Expires: 10 minutes
```

## API Endpoints

### POST /api/web3/auth/generate-nonce

Generate a nonce for signature verification.

**Request:**
```json
{
  "address": "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0",
  "wallet_type": "metamask"
}
```

**Response:**
```json
{
  "nonce": "a1b2c3d4e5f6...",
  "timestamp": "1640995200",
  "message": "Monnaire Trading Agent OS - Web3 Authentication\n\n..."
}
```

### POST /api/web3/auth/authenticate

Verify wallet signature and authenticate user.

**Request:**
```json
{
  "address": "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0",
  "signature": "0x1234abcd...",
  "nonce": "a1b2c3d4e5f6...",
  "wallet_type": "metamask"
}
```

**Response:**
```json
{
  "success": true,
  "message": "钱包验证成功",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "wallet_addr": "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0",
  "bound_wallets": [
    "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0",
    "0x1234567890abcdef1234567890abcdef12345678"
  ]
}
```

### POST /api/web3/wallet/link

Link wallet to authenticated user.

**Headers:**
```
Authorization: Bearer {jwt_token}
```

**Request:**
```json
{
  "address": "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0",
  "wallet_type": "metamask",
  "is_primary": true
}
```

**Response:**
```json
{
  "success": true,
  "message": "钱包绑定成功",
  "address": "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0"
}
```

### DELETE /api/web3/wallet/{address}

Unlink wallet from authenticated user.

**Headers:**
```
Authorization: Bearer {jwt_token}
```

**Response:**
```json
{
  "success": true,
  "message": "钱包解绑成功",
  "address": "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0"
}
```

### GET /api/web3/wallet/list

List all wallets linked to authenticated user.

**Headers:**
```
Authorization: Bearer {jwt_token}
```

**Response:**
```json
{
  "success": true,
  "wallets": [
    {
      "id": "uw_001",
      "user_id": "user_123",
      "wallet_addr": "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0",
      "is_primary": true,
      "bound_at": "2025-12-01T10:00:00Z",
      "last_used_at": "2025-12-01T10:00:00Z"
    }
  ]
}
```

## Security Requirements

1. **Nonce Expiration**: Nonces expire after 10 minutes
2. **Signature Validation**: Must use EIP-191 standard
3. **Address Format**: Strict validation of Ethereum address format
4. **Rate Limiting**: Max 10 requests per minute per IP
5. **Audit Logging**: All wallet operations logged to audit_logs table

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| WEB3_001 | 400 | Invalid address format |
| WEB3_002 | 401 | Invalid signature |
| WEB3_003 | 400 | Nonce expired |
| WEB3_004 | 401 | Address mismatch |
| WEB3_005 | 409 | Wallet already bound |
| WEB3_006 | 404 | Wallet not bound |
| WEB3_007 | 400 | Cannot unbind primary wallet |
| WEB3_008 | 400 | Invalid wallet type |

## Testing Requirements

- Unit tests for all signature verification functions
- Integration tests for all API endpoints
- E2E tests for complete wallet connection flow
- Performance tests: <100ms signature verification
- Security tests: signature replay attack prevention
