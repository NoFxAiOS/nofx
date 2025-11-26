# Binance API Setup Guide

This guide explains how to create and configure Binance API keys for use with NOFX.

## Create API Key

1. Log in to your [Binance account](https://www.binance.com)
2. Go to **Account** → **API Management**
3. Click **Create API**
4. Select **System Generated** API key type
5. Complete 2FA verification
6. Name your API key (e.g., "NOFX Trading")

## Configure API Permissions

Enable the following permissions:

- ✅ **Enable Reading** - Required
- ✅ **Enable Futures** - Required for trading
- ❌ **Enable Withdrawals** - Keep disabled for security

## IP Whitelist (Recommended)

For enhanced security:

1. Click **Edit restrictions**
2. Select **Restrict access to trusted IPs only**
3. Add your server's IP address
4. Save changes

## Save Your Keys

After creation, you'll see:
- **API Key**: `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`
- **Secret Key**: `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

⚠️ **Important**: Save the Secret Key immediately - it's only shown once!

## Configure in NOFX

Add your API credentials through the NOFX web interface:

1. Open NOFX dashboard (http://localhost:3000)
2. Go to **Exchange Configuration**
3. Enable **Binance**
4. Enter your API Key and Secret Key
5. Save configuration

## Troubleshooting

| Error | Solution |
|-------|----------|
| `Invalid API-key` | Check if API key is correct |
| `Signature verification failed` | Check if Secret key is correct |
| `IP not whitelisted` | Add your IP to whitelist or disable IP restriction |
| `Futures not enabled` | Enable Futures permission in API settings |

## Security Best Practices

- Never share your API keys
- Use IP whitelisting
- Don't enable withdrawal permissions
- Create separate API keys for different applications
- Regularly rotate your API keys
