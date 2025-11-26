# Lighter Agent Wallet Setup Guide

This guide explains how to create and configure an Agent Wallet for secure trading on Lighter.

## Why Use Agent Wallet?

- ✅ **More Secure**: Never expose your main wallet private key
- ✅ **Limited Access**: Agent only has trading permissions
- ✅ **Revocable**: Can be disabled anytime
- ✅ **Separate Funds**: Keep main holdings safe

## Prerequisites

- A Web3 wallet (MetaMask, WalletConnect, etc.)
- Access to [Lighter](https://lighter.xyz)

## Step 1: Connect Your Main Wallet

1. Visit [Lighter](https://lighter.xyz)
2. Click **Connect Wallet**
3. Choose MetaMask, WalletConnect, or other Web3 wallet
4. Approve the connection

## Step 2: Create Agent Wallet

1. Navigate to **Settings** or **API** section
2. Look for **Agent Wallet** or **Trading Wallet** option
3. Click **Create Agent** or **Generate New Wallet**
4. Approve the transaction if required

## Step 3: Save Agent Credentials

After creation, save these immediately:

| Field | Description |
|-------|-------------|
| **Main Wallet Address** | Your connected wallet address |
| **Agent Wallet Address** | Generated agent wallet address |
| **Agent Private Key** | Private key for the agent wallet |

⚠️ **Important**: The private key is only shown once! Save it securely.

## Step 4: Configure in NOFX

Add your agent wallet through the NOFX web interface:

1. Open NOFX dashboard (http://localhost:3000)
2. Go to **Exchange Configuration**
3. Enable **Lighter**
4. Enter:
   - **Wallet Address**: Your main wallet address (with `0x`)
   - **Private Key**: Agent private key (remove `0x` prefix)
5. Save configuration

## Managing Your Agent

### Revoke Agent Access

1. Go to Lighter Settings
2. Find your agent in the list
3. Click **Revoke** or **Delete**

### Fund Your Account

1. Deposit supported assets to Lighter
2. Agent wallet will trade using deposited funds

## Security Best Practices

- Use agent wallet instead of main wallet private key
- Store agent private key securely
- Revoke unused agents
- Monitor agent activity regularly
- Keep main wallet funds separate from trading funds

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Agent not working | Check if agent is still active |
| Invalid signature | Ensure private key doesn't have `0x` prefix |
| Insufficient funds | Deposit funds to your Lighter account |
| Connection error | Check network settings |
