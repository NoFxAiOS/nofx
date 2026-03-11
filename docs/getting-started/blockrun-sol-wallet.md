# BlockRun Solana Wallet Setup Guide

This guide explains how to use a Solana wallet to pay for AI usage through BlockRun — no API key required.

**Language:** [English](blockrun-sol-wallet.md) | [中文](blockrun-sol-wallet.zh-CN.md)

## What is BlockRun?

[BlockRun](https://blockrun.ai) is a decentralized AI inference gateway that lets you access top AI models (Claude, GPT, Gemini, Grok, DeepSeek, etc.) by paying per request with USDC — no monthly subscriptions, no API key signups.

NOFX integrates BlockRun via the **x402 micropayment protocol** on Solana: each AI inference request automatically pays a small USDC fee directly from your wallet.

## Prerequisites

- A Solana wallet with USDC on **Solana mainnet**
- The wallet private key (base58-encoded, 64 bytes — standard Solana keypair format)

### Getting USDC on Solana

1. Buy SOL on any exchange and withdraw to your Solana wallet, then swap to USDC on [Jupiter](https://jup.ag), **or**
2. Buy USDC directly on an exchange and withdraw to Solana, **or**
3. Bridge from other chains using [Wormhole](https://wormhole.com)

> **Tip:** A few dollars of USDC is plenty to start.

## Step 1: Export Your Solana Private Key

> ⚠️ **Security Warning:** Use a dedicated wallet for NOFX — not your main holdings wallet.

**From Phantom Wallet:**
1. Open Phantom → Settings (gear icon)
2. Security & Privacy → Export Private Key
3. Enter your password
4. Copy the base58 key (looks like: `5J...` — a long string of ~88 characters)

**From Solflare:**
1. Settings → Export Private Key
2. The key is displayed in base58 format

**From CLI (solana-keygen):**
```bash
# View existing keypair
cat ~/.config/solana/id.json
# This is a JSON array — convert to base58 using:
solana-keygen pubkey ~/.config/solana/id.json
```

> **Note:** NOFX accepts the **base58-encoded 64-byte keypair** (as exported by Phantom/Solflare). This is the standard format for Solana private keys.

## Step 2: Fund the Wallet with USDC on Solana

Send USDC to your Solana wallet:
- **USDC SPL token mint:** `EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v`
- **Network:** Solana Mainnet
- **Recommended starting amount:** $5–$20 USDC

Check your balance at [solscan.io](https://solscan.io) or in your wallet app.

## Step 3: Configure in NOFX

1. Open NOFX at `http://localhost:3000`
2. Log in and go to **Config** tab
3. Click **+ Add AI Model**
4. In Step 0, scroll to **Via BlockRun Wallet** section
5. Select **BlockRun · Solana Wallet**
6. In Step 1, configure:
   - **Wallet Private Key:** Your base58-encoded Solana private key
   - **Select Model:** Choose from Claude Opus, GPT-5.4, Gemini 3 Pro, Grok 3, DeepSeek R1, or leave as **Auto** for best available
7. Click **Save**

## How Payment Works

When NOFX sends an AI request:

1. Request goes to `https://sol.blockrun.ai/api/v1/chat/completions`
2. Server responds with HTTP `402 Payment Required` + payment details (nonce, recipient, amount)
3. NOFX signs the payment message `blockrun-payment:{nonce}:{recipient}:{amount}` with your **Ed25519** private key
4. Payment signature is attached and request is retried
5. BlockRun verifies the Ed25519 signature on-chain and routes to the AI model

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

- ✅ Use a **dedicated trading wallet** with only your AI budget
- ✅ Keep only a small USDC balance (top up as needed)
- ✅ Your private key is AES-256 encrypted at rest in NOFX's database
- ✅ Ed25519 signatures are one-time — each authorizes only one specific payment
- ❌ Never use your main SOL holdings wallet as the NOFX trading wallet

## Troubleshooting

| Issue | Solution |
|-------|----------|
| `unexpected key length` | Ensure you exported the full 64-byte keypair (not just the 32-byte seed) |
| `failed to decode base58` | Key must be base58 encoded (standard Phantom/Solflare export format) |
| `payment retry failed` | Ensure you have USDC on **Solana mainnet** (not devnet) |
| No response from server | Check `sol.blockrun.ai` is reachable from your server |
| Slow responses | Try selecting a specific model instead of "Auto" |

## Monitoring Spend

Check your USDC balance and transaction history at:
- [Solscan](https://solscan.io) — search your wallet address, filter by USDC token
- [BlockRun dashboard](https://blockrun.ai) — usage history

---

[← Back to Getting Started](README.md)
