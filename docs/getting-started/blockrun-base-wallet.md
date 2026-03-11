# BlockRun Base (EVM) Wallet Setup Guide

This guide explains how to use a Base network EVM wallet to pay for AI usage through BlockRun — no API key required.

**Language:** [English](blockrun-base-wallet.md) | [中文](blockrun-base-wallet.zh-CN.md)

## What is BlockRun?

[BlockRun](https://blockrun.ai) is a decentralized AI inference gateway that lets you access top AI models (Claude, GPT, Gemini, Grok, DeepSeek, etc.) by paying per request with USDC — no monthly subscriptions, no API key signups.

NOFX integrates BlockRun via the **x402 micropayment protocol**: each AI inference request automatically pays a small USDC fee directly from your wallet. You only pay for what you use.

## Why Use BlockRun?

| Feature | Traditional API Key | BlockRun Wallet |
|---------|-------------------|-----------------|
| Setup | Register + billing | Just a wallet address |
| Cost model | Monthly subscription | Pay-per-request |
| Models | One provider | All top models |
| Privacy | Account required | Pseudonymous |
| Control | Rate limits apply | Your wallet, your budget |

## Prerequisites

- An EVM wallet with USDC on **Base network** (chain ID 8453)
- The wallet private key (hex format: `0x...`)

### Getting USDC on Base

1. Buy USDC on Coinbase and withdraw to Base, **or**
2. Bridge USDC from Ethereum using [bridge.base.org](https://bridge.base.org), **or**
3. Swap on [Aerodrome](https://aerodrome.finance) or [Uniswap](https://app.uniswap.org) on Base

> **Tip:** A few dollars of USDC is enough to start — each AI call costs fractions of a cent.

## Step 1: Get Your Wallet Private Key

> ⚠️ **Security Warning:** Never share your private key with anyone. Use a dedicated trading wallet, not your main holdings wallet.

**Option A — Create a new wallet (recommended):**
1. Open MetaMask → Create New Account
2. Go to Account Details → Export Private Key
3. Copy the hex key (starts with `0x`)

**Option B — Use an existing wallet:**
1. MetaMask → Account Details → Export Private Key
2. Enter your MetaMask password to reveal the key

**Option C — Generate via CLI:**
```bash
# Using cast (foundry)
cast wallet new
# Output: Address: 0x... | Private key: 0x...
```

## Step 2: Fund the Wallet with USDC on Base

Send USDC to your wallet address on Base network:
- **USDC contract:** `0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913`
- **Network:** Base (chain ID 8453)
- **Recommended starting amount:** $5–$20 USDC

Check your balance at [basescan.org](https://basescan.org).

## Step 3: Configure in NOFX

1. Open NOFX at `http://localhost:3000`
2. Log in and go to **Config** tab
3. Click **+ Add AI Model**
4. In Step 0, scroll to **Via BlockRun Wallet** section
5. Select **BlockRun · Base Wallet**
6. In Step 1, configure:
   - **Wallet Private Key:** Your hex private key (`0x...`)
   - **Select Model:** Choose from Claude Opus, GPT-5.4, Gemini 3 Pro, Grok 3, DeepSeek R1, or leave as **Auto** for best available
7. Click **Save**

## How Payment Works

When NOFX sends an AI request:

1. Request goes to `https://blockrun.ai/api/v1/chat/completions`
2. Server responds with HTTP `402 Payment Required` + payment details
3. NOFX signs a **ERC-3009 TransferWithAuthorization** (EIP-712) with your private key
4. Payment signature is attached and request is retried
5. BlockRun verifies the signature, routes the request to the AI model, and charges USDC

> **Privacy:** Your private key never leaves your NOFX instance. Only the cryptographic signature is sent.

## Available Models via BlockRun

| Model ID | Provider | Use Case |
|----------|----------|----------|
| `gpt-5.4` | OpenAI | Flagship (default) |
| `claude-opus-4.6` | Anthropic | Flagship |
| `gemini-3.1-pro` | Google | Flagship |
| `grok-3` | xAI | Flagship |
| `deepseek-chat` | DeepSeek | Flagship |
| `minimax-m2.5` | MiniMax | Flagship |

## Security Best Practices

- ✅ Use a **dedicated wallet** with only trading budget, not your main wallet
- ✅ Keep only a small USDC balance (top up as needed)
- ✅ Your private key is encrypted at rest in NOFX's database
- ✅ Signatures are spend-limited — each signature authorizes only the exact amount for one request
- ❌ Never export or share your private key outside of NOFX

## Troubleshooting

| Issue | Solution |
|-------|----------|
| `no private key set` | Check your key was saved correctly; re-enter in Config |
| `payment retry failed` | Ensure you have USDC on **Base** (not Ethereum mainnet) |
| `invalid private key` | Key must be hex format with `0x` prefix, 66 chars total |
| Payment deducted but no response | Check BlockRun status at [blockrun.ai](https://blockrun.ai) |
| Slow responses | Try selecting a specific model instead of "Auto" |

## Monitoring Spend

Check your USDC balance and transaction history at:
- [Basescan](https://basescan.org) — search your wallet address
- [BlockRun dashboard](https://blockrun.ai) — usage history

---

[← Back to Getting Started](README.md)
