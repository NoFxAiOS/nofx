# Monnaire Trading Agent OS User Manual

## 1. Registration and Login

### 1.1 Registration
To use Monnaire Trading Agent OS, you first need to register an account:

1. Open the web interface at `http://localhost:3000` or your deployed domain
2. Click on the "Register" button in the top navigation
3. Fill in your email address and password
4. Click "Register" to complete the registration

### 1.2 Login
After successful registration, you can log in:

1. Click on the "Login" button in the top navigation
2. Enter your registered email and password
3. Click "Login" to access your dashboard

## 2. Configuration

### 2.1 AI Model API Configuration
To use AI trading capabilities, you need to configure API keys for your preferred AI models:

1. Log in to your account and navigate to the "AI Models" tab
2. Select the AI model you want to use (DeepSeek, Qwen, etc.)
3. Enter your API key and any required parameters
4. Click "Save" to apply the configuration

### 2.2 Exchange Configuration
Configure your exchange API credentials:

1. Navigate to the "Exchanges" tab
2. Select your preferred exchange (Binance, Hyperliquid, OKX)
3. Enter your API Key and Secret Key
4. For OKX, also enter your Passphrase
5. Enable "Testnet" mode if you want to test without real funds
6. Click "Save" to apply the configuration

## 3. Trader Management

### 3.1 Create a New Trader
To create an AI trader:

1. Navigate to the "My Traders" tab
2. Click "Create Trader"
3. Fill in the trader name, select AI model and exchange
4. Set initial balance, leverage, and trading symbols
5. Configure custom prompts if needed
6. Click "Create" to save the trader configuration

### 3.2 Start/Stop a Trader
To control your trader:

1. On the "My Traders" page, find your trader
2. Click "Start" to activate the trading bot
3. Click "Stop" to pause trading
4. Monitor real-time status and performance on the dashboard

### 3.3 Optimize Trader Settings
Improve your trader's performance:

1. Click on the trader you want to optimize
2. Adjust AI model parameters, leverage, or trading symbols
3. Update custom prompts to refine trading strategies
4. Save changes and restart the trader

## 4. Risk Management

### 4.1 Initial Setup Recommendations
- Start with a small initial balance for testing
- Use Testnet mode before real trading
- Set conservative leverage (1-5x) for beginners

### 4.2 Monitor Performance
- Regularly check the "Statistics" and "Performance" tabs
- Analyze trading decisions and adjust strategies
- Set stop-loss and take-profit rules

## 5. Community and Support

- Join our Telegram community to share strategies
- Submit PRs on GitHub to improve the framework
- Check the documentation for advanced configurations
