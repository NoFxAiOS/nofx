# Hyperliquid Agent Wallet Setup Guide

This guide explains how to create and configure an Agent Wallet for secure trading on Hyperliquid.

## Why Use Agent Wallet?

- ✅ **More Secure**: Never expose your main wallet private key
- ✅ **Limited Access**: Agent only has trading permissions
- ✅ **Revocable**: Can be disabled anytime from Hyperliquid interface
- ✅ **Separate Funds**: Keep main holdings safe

## Prerequisites

- A wallet with funds on Hyperliquid
- Access to [Hyperliquid](https://app.hyperliquid.xyz/join/AITRADING)

## Step 1: Connect Your Main Wallet

1. Visit [Hyperliquid](https://app.hyperliquid.xyz/join/AITRADING)
2. Click **Connect Wallet** (top right)
3. Choose MetaMask, WalletConnect, or other Web3 wallet
4. Approve the connection

## Step 2: Create Agent Wallet

1. Click on your wallet address (top right)
2. Go to **Settings** → **API & Agents**
3. Or visit directly: [https://app.hyperliquid.xyz/agents](https://app.hyperliquid.xyz/agents)
4. Click **Create Agent** or **Add Agent**
5. System generates a new agent wallet automatically

## Step 3: Save Agent Credentials

After creation, save these immediately:

- **Agent Wallet Address**: `0x...` (starts with 0x)
- **Agent Private Key**: Shown only once!

⚠️ **Important**: The private key is only displayed once. Save it securely!

## Step 4: Configure in NOFX

Add your agent wallet through the NOFX web interface:

1. Open NOFX dashboard (http://localhost:3000)
2. Go to **Exchange Configuration**
3. Enable **Hyperliquid**
4. Enter:
   - **Wallet Address**: Your main wallet address (with `0x`)
   - **Private Key**: Agent private key (remove `0x` prefix)
5. Save configuration

## Agent Wallet Details

| Field | Description | Example |
|-------|-------------|---------|
| Main Wallet | Your connected wallet (holds funds) | `0xABC123...` |
| Agent Wallet | Sub-wallet for trading | `0xDEF456...` |
| Private Key | Only needed for NOFX | `abc123...` (no 0x) |

## Managing Your Agent

### Revoke Agent Access

1. Go to [Hyperliquid Agents](https://app.hyperliquid.xyz/agents)
2. Find your agent in the list
3. Click **Revoke** or **Delete**

### Create Multiple Agents

You can create multiple agents for different purposes:
- One for NOFX
- One for other trading bots
- One for manual API access

## Security Best Practices

- Use agent wallet instead of main wallet private key
- Store agent private key securely
- Revoke unused agents
- Monitor agent activity regularly
- Keep main wallet funds separate from trading funds

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Agent not working | Check if agent is still active in Hyperliquid settings |
| Invalid signature | Ensure private key doesn't have `0x` prefix |
| Insufficient funds | Transfer funds to your Hyperliquid account |
| Connection error | Check network (mainnet vs testnet) setting |
