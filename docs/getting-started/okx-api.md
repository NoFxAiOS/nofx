# OKX API Setup Guide

This guide explains how to create and configure OKX API keys for use with NOFX.

## Create API Key

1. Log in to your [OKX account](https://www.okx.com/join/1865360)
2. Go to **Account** → **API**
3. Click **Create API Key**
4. Select **Trade** as the purpose
5. Complete 2FA verification
6. Name your API key (e.g., "NOFX Trading")

## Configure API Permissions

Enable the following permissions:

- ✅ **Read** - Required
- ✅ **Trade** - Required for trading
- ❌ **Withdraw** - Keep disabled for security

## Passphrase

OKX requires a passphrase in addition to API Key and Secret:

1. Create a strong passphrase during API creation
2. Save it securely - you'll need it for configuration

## IP Whitelist (Recommended)

For enhanced security:

1. Click **Edit** on your API key
2. Enable **IP Whitelist**
3. Add your server's IP address
4. Save changes

## Save Your Keys

After creation, you'll have:
- **API Key**: `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
- **Secret Key**: `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`
- **Passphrase**: Your created passphrase

⚠️ **Important**: Save the Secret Key immediately - it's only shown once!

## Configure in NOFX

Add your API credentials through the NOFX web interface:

1. Open NOFX dashboard (http://localhost:3000)
2. Go to **Exchange Configuration**
3. Enable **OKX**
4. Enter:
   - **API Key**
   - **Secret Key**
   - **Passphrase**
5. Save configuration

## Troubleshooting

| Error | Solution |
|-------|----------|
| `Invalid API key` | Check if API key is correct |
| `Invalid signature` | Check if Secret key and Passphrase are correct |
| `IP not whitelisted` | Add your IP to whitelist or disable IP restriction |
| `Permission denied` | Enable Trade permission in API settings |

## Security Best Practices

- Never share your API keys or passphrase
- Use IP whitelisting
- Don't enable withdrawal permissions
- Create separate API keys for different applications
- Regularly rotate your API keys
