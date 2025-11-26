# Aster DEX API Wallet Setup Guide

This guide explains how to create and configure an API Wallet for secure trading on Aster DEX.

## Why Use API Wallet?

- ✅ **Binance-compatible API**: Easy migration from Binance
- ✅ **Separate Trading Wallet**: Enhanced security
- ✅ **Revocable Access**: Can be disabled anytime
- ✅ **Lower Fees**: Competitive trading fees

## Prerequisites

- A Web3 wallet (MetaMask, WalletConnect, etc.)
- Funds on supported EVM chain (Ethereum, BSC, Polygon, etc.)

## Step 1: Register on Aster DEX

1. Visit [Aster DEX](https://www.asterdex.com/en/referral/fdfc0e) (use referral link for fee discount)
2. Connect your Web3 wallet
3. Complete any required verification

## Step 2: Create API Wallet

1. Go to [Aster API Wallet](https://www.asterdex.com/en/api-wallet)
2. Connect your main wallet
3. Click **Create API Wallet**
4. Approve the transaction in your wallet

## Step 3: Save API Wallet Credentials

After creation, save these **immediately**:

| Field | Description |
|-------|-------------|
| **User Address** | Your main wallet address |
| **Signer Address** | API wallet address |
| **Private Key** | API wallet private key |

⚠️ **Important**: The private key is only shown once! Save it securely.

## Step 4: Configure in NOFX

Add your API wallet through the NOFX web interface:

1. Open NOFX dashboard (http://localhost:3000)
2. Go to **Exchange Configuration**
3. Enable **Aster DEX**
4. Enter:
   - **User**: Your main wallet address (with `0x`)
   - **Signer**: API wallet address (with `0x`)
   - **Private Key**: API wallet private key (remove `0x` prefix)
5. Save configuration

## Configuration Example

```
User:        0xYOUR_MAIN_WALLET_ADDRESS
Signer:      0xYOUR_API_WALLET_SIGNER_ADDRESS
Private Key: your_api_wallet_private_key_without_0x
```

## Fund Your Account

1. Deposit supported assets to Aster DEX
2. Transfer to your trading account
3. API wallet will trade using these funds

## Managing Your API Wallet

### Revoke Access

1. Go to [Aster API Wallet](https://www.asterdex.com/en/api-wallet)
2. Find your API wallet
3. Click **Revoke** or **Delete**

### Create New API Wallet

You can create multiple API wallets:
- Delete old wallet first (recommended)
- Or create additional wallet for different purposes

## Security Best Practices

- Never share your API wallet private key
- Store credentials in a secure password manager
- Revoke access when not in use
- Use separate wallets for different applications
- Monitor API wallet activity regularly

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Authentication failed | Verify User, Signer, and Private Key are correct |
| Invalid signature | Ensure private key doesn't have `0x` prefix |
| Insufficient balance | Deposit funds to Aster DEX |
| API wallet not found | Create new API wallet at asterdex.com |

## Supported Chains

Aster DEX supports multiple EVM chains:
- Ethereum Mainnet
- BNB Smart Chain (BSC)
- Polygon
- And more...

Select your preferred chain when depositing funds.
