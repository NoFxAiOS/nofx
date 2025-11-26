# Bybit API Setup Guide

This guide explains how to create and configure Bybit API keys for use with NOFX.

## Create API Key

1. Log in to your [Bybit account](https://partner.bybit.com/b/83856)
2. Go to **Account & Security** → **API Management**
3. Click **Create New Key**
4. Select **System-generated API Keys**
5. Complete 2FA verification
6. Name your API key (e.g., "NOFX Trading")

## Configure API Permissions

Enable the following permissions:

- ✅ **Read-Write** - Required for trading
- ✅ **Contract** - Required for futures/perpetual trading
- ❌ **Withdrawals** - Keep disabled for security

## IP Whitelist (Recommended)

For enhanced security:

1. Click **Edit** on your API key
2. Add your server's IP address to the whitelist
3. Save changes

## Save Your Keys

After creation, you'll see:
- **API Key**: `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`
- **API Secret**: `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

⚠️ **Important**: Save the API Secret immediately - it's only shown once!

## Configure in NOFX

Add your API credentials through the NOFX web interface:

1. Open NOFX dashboard (http://localhost:3000)
2. Go to **Exchange Configuration**
3. Enable **Bybit**
4. Enter your API Key and API Secret
5. Save configuration

## Troubleshooting

| Error | Solution |
|-------|----------|
| `Invalid API key` | Check if API key is correct |
| `Signature error` | Check if API Secret is correct |
| `IP not allowed` | Add your IP to whitelist |
| `Permission denied` | Enable Contract trading permission |

## Security Best Practices

- Never share your API keys
- Use IP whitelisting
- Don't enable withdrawal permissions
- Create separate API keys for different applications
- Regularly rotate your API keys
