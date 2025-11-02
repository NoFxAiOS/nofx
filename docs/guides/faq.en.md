# Frequently Asked Questions

## Binance Position Mode Error (code=-4061)

**Error Message**: `Order's position side does not match user's setting`

**Cause**: The system requires Hedge Mode (dual position), but your Binance account is set to One-way Mode.

### Solution

1. Login to [Binance Futures Trading Platform](https://www.binance.com/en/futures/BTCUSDT)

2. Click **⚙️ Preferences** in the top right corner

3. Select **Position Mode**

4. Switch to **Hedge Mode** (Dual Position)

5. Confirm the change

**Note**: You must close all open positions before switching modes.

---

## How to Generate Binance API Keys

1. Visit [Binance API Management](https://www.binance.com/en/support/faq/detail/6b9a63f1e3384cf48a2eedb82767a69a)

2. Login to your Binance account

3. Go to **API Management** section

4. Click **Create API**

5. Set API key name (e.g., "NOFX Trading")

6. Enable **Enable Spot & Margin Trading** and **Enable Futures**

7. Configure IP Access Restriction (optional but recommended)

8. Click **Create**

9. Save your **API Key** and **Secret Key** securely

10. For enhanced security, Binance now supports multiple API key types:

    - **HMAC** (default) - Traditional signature method
    - **ED25519** - More secure elliptic curve cryptography
    - **RSA** - RSA public-key cryptography

    You can specify the key type in your NOFX configuration with the `binance_api_key_type` field.

---

## How to Convert Multi-line PEM Format to Single Line

When using ED25519 or RSA API keys, Binance provides keys in PEM format which spans multiple lines. To use these keys in NOFX, you need to convert them to single-line format:

### Method 1: Using awk (Linux/macOS)

```bash
awk '{printf "%s", $0} END {print ""}' private_key.pem
```

### Method 2: Using Python

```python
with open('private_key.pem', 'r') as f:
    content = f.read().replace('\n', '')
    print(content)
```

### Method 3: Using sed (Linux/macOS)

```bash
sed ':a;N;$!ba;s/\n//g' private_key.pem
```

**Note**: Be careful when handling private keys. Never share them with anyone or commit them to version control systems.

---

For more issues, check [GitHub Issues](https://github.com/tinkle-community/nofx/issues)
