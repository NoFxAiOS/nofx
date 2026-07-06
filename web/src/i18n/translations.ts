export type Language = 'en' | 'zh' | 'id'

export const translations = {
  en: {
    // Header
    appTitle: 'NOFX',
    subtitle: 'Multi-AI Model Trading Platform',
    aiTraders: 'AI Traders',
    details: 'Details',
    tradingPanel: 'Trading Panel',
    competition: 'Competition',
    running: 'RUNNING',
    stopped: 'STOPPED',
    adminMode: 'Admin Mode',
    logout: 'Logout',
    switchTrader: 'Switch Trader:',
    view: 'View',

    // Navigation
    realtimeNav: 'Leaderboard',
    configNav: 'Config',
    dashboardNav: 'Dashboard',
    strategyNav: 'Strategy',
    faqNav: 'FAQ',

    // Footer
    footerTitle: 'NOFX - AI Trading System',
    footerWarning: '⚠️ Trading involves risk. Use at your own discretion.',

    // Stats Cards
    totalEquity: 'Total Equity',
    availableBalance: 'Available Balance',
    totalPnL: 'Total P&L',
    positions: 'Positions',
    margin: 'Margin',
    free: 'Free',

    // Positions Table
    currentPositions: 'Current Positions',
    active: 'Active',
    symbol: 'Symbol',
    side: 'Side',
    entryPrice: 'Entry Price',
    stopLoss: 'Stop Loss',
    takeProfit: 'Take Profit',
    riskReward: 'Risk/Reward',
    markPrice: 'Mark Price',
    quantity: 'Quantity',
    positionValue: 'Position Value',
    leverage: 'Leverage',
    unrealizedPnL: 'Unrealized P&L',
    liqPrice: 'Liq. Price',
    long: 'LONG',
    short: 'SHORT',
    noPositions: 'No Positions',
    noActivePositions: 'No active trading positions',

    // Recent Decisions
    recentDecisions: 'Recent Decisions',
    lastCycles: 'Last {count} trading cycles',
    noDecisionsYet: 'No Decisions Yet',
    aiDecisionsWillAppear: 'AI trading decisions will appear here',
    cycle: 'Cycle',
    success: 'Success',
    failed: 'Failed',
    inputPrompt: 'Input Prompt',
    aiThinking: 'AI Chain of Thought',
    collapse: 'Collapse',
    expand: 'Expand',

    // Equity Chart
    accountEquityCurve: 'Account Equity Curve',
    noHistoricalData: 'No Historical Data',
    dataWillAppear: 'Equity curve will appear after running a few cycles',
    initialBalance: 'Initial Balance',
    currentEquity: 'Current Equity',
    historicalCycles: 'Historical Cycles',
    displayRange: 'Display Range',
    recent: 'Recent',
    allData: 'All Data',
    cycles: 'Cycles',

    // Comparison Chart
    comparisonMode: 'Comparison Mode',
    dataPoints: 'Data Points',
    currentGap: 'Current Gap',
    count: '{count} pts',

    // TradingView Chart
    marketChart: 'Market Chart',
    viewChart: 'Click to view chart',
    enterSymbol: 'Enter symbol...',
    popularSymbols: 'Popular Symbols',
    fullscreen: 'Fullscreen',
    exitFullscreen: 'Exit Fullscreen',

    // Competition Page
    aiCompetition: 'AI Competition',
    traders: 'traders',
    liveBattle: 'Live Battle',
    realTimeBattle: 'Real-time Battle',
    leader: 'Leader',
    leaderboard: 'Leaderboard',
    live: 'LIVE',
    realTime: 'LIVE',
    performanceComparison: 'Performance Comparison',
    realTimePnL: 'Real-time PnL %',
    realTimePnLPercent: 'Real-time PnL %',
    headToHead: 'Head-to-Head Battle',
    leadingBy: 'Leading by {gap}%',
    behindBy: 'Behind by {gap}%',
    equity: 'Equity',
    pnl: 'P&L',
    pos: 'Pos',

    // AI Traders Management
    manageAITraders: 'Manage your AI trading bots',
    aiModels: 'AI Models',
    exchanges: 'Exchanges',
    createTrader: 'Create Trader',
    modelConfiguration: 'Model Configuration',
    configured: 'Configured',
    notConfigured: 'Not Configured',
    currentTraders: 'Current Traders',
    noTraders: 'No AI Traders',
    createFirstTrader: 'Create your first AI trader to get started',
    dashboardEmptyTitle: "Let's Get Started!",
    dashboardEmptyDescription:
      'Create your first AI trader to automate your trading strategy. Connect an exchange, choose an AI model, and start trading in minutes!',
    goToTradersPage: 'Create Your First Trader',
    configureModelsFirst: 'Please configure AI models first',
    configureExchangesFirst: 'Please configure exchanges first',
    configureModelsAndExchangesFirst:
      'Please configure AI models and exchanges first',
    modelNotConfigured: 'Selected model is not configured',
    exchangeNotConfigured: 'Selected exchange is not configured',
    confirmDeleteTrader: 'Are you sure you want to delete this trader?',
    status: 'Status',
    start: 'Start',
    stop: 'Stop',
    createNewTrader: 'Create New AI Trader',
    selectAIModel: 'Select AI Model',
    selectExchange: 'Select Exchange',
    traderName: 'Trader Name',
    enterTraderName: 'Enter trader name',
    cancel: 'Cancel',
    create: 'Create',
    configureAIModels: 'Configure AI Models',
    configureExchanges: 'Configure Exchanges',
    aiScanInterval: 'AI Scan Decision Interval (minutes)',
    scanIntervalRecommend: 'Recommended: 15-30 minutes',
    useTestnet: 'Use Testnet',
    enabled: 'Enabled',
    save: 'Save',

    // TraderConfigModal - New keys for hardcoded Chinese strings
    fetchBalanceEditModeOnly: 'Only can fetch current balance in edit mode',
    balanceFetched: 'Current balance fetched',
    balanceFetchFailed: 'Failed to fetch balance',
    balanceFetchNetworkError:
      'Failed to fetch balance, please check network connection',
    saving: 'Saving...',
    saveSuccess: 'Saved successfully',
    saveFailed: 'Save failed',
    editTraderConfig: 'Edit Trader Configuration',
    selectStrategyAndConfigParams:
      'Select Strategy and Configure Basic Parameters',
    basicConfig: 'Basic Configuration',
    traderNameRequired: 'Trader Name *',
    enterTraderNamePlaceholder: 'Enter trader name',
    aiModelRequired: 'AI Model *',
    exchangeRequired: 'Exchange *',
    noExchangeAccount: "Don't have an exchange account? Click to register",
    discount: 'Discount',
    selectTradingStrategy: 'Select Trading Strategy',
    useStrategy: 'Use Strategy',
    noStrategyManual: '-- No Strategy (Manual Configuration) --',
    strategyActive: ' (Active)',
    strategyDefault: ' [Default]',
    noStrategyHint: 'No strategies yet, please create in Strategy Studio first',
    strategyDetails: 'Strategy Details',
    activating: 'Activating',
    coinSource: 'Coin Source',
    marginLimit: 'Margin Limit',
    tradingParams: 'Trading Parameters',
    marginMode: 'Margin Mode',
    crossMargin: 'Cross Margin',
    isolatedMargin: 'Isolated Margin',
    competitionDisplay: 'Show in Competition',
    show: 'Show',
    hide: 'Hide',
    hiddenInCompetition:
      'This trader will not be shown in the competition page when hidden',
    initialBalanceLabel: 'Initial Balance ($)',
    fetching: 'Fetching...',
    fetchCurrentBalance: 'Fetch Current Balance',
    balanceUpdateHint:
      'Used to manually update the initial balance baseline (e.g., after deposit/withdrawal)',
    autoFetchBalanceInfo:
      'The system will automatically fetch your account equity as the initial balance',
    fetchingBalance: 'Fetching balance...',
    editTrader: 'Save Changes',
    createTraderButton: 'Create Trader',

    // AI Model Configuration
    officialAPI: 'Official API',
    customAPI: 'Custom API',
    apiKey: 'API Key',
    customAPIURL: 'Custom API URL',
    enterAPIKey: 'Enter API Key',
    enterCustomAPIURL: 'Enter custom API endpoint URL',
    useOfficialAPI: 'Use official API service',
    useCustomAPI: 'Use custom API endpoint',

    // Exchange Configuration
    secretKey: 'Secret Key',
    privateKey: 'Private Key',
    walletAddress: 'Wallet Address',
    user: 'User',
    signer: 'Signer',
    passphrase: 'Passphrase',
    enterPrivateKey: 'Enter Private Key',
    enterWalletAddress: 'Enter Wallet Address',
    enterUser: 'Enter User',
    enterSigner: 'Enter Signer Address',
    enterSecretKey: 'Enter Secret Key',
    enterPassphrase: 'Enter Passphrase',
    hyperliquidPrivateKeyDesc:
      'Hyperliquid uses private key for trading authentication',
    hyperliquidWalletAddressDesc:
      'Wallet address corresponding to the private key',
    // Hyperliquid Agent Wallet (New Security Model)
    hyperliquidAgentWalletTitle: 'Hyperliquid Agent Wallet Configuration',
    hyperliquidAgentWalletDesc:
      'Use Agent Wallet for secure trading: Agent wallet signs transactions (balance ~0), Main wallet holds funds (never expose private key)',
    hyperliquidAgentPrivateKey: 'Agent Private Key',
    enterHyperliquidAgentPrivateKey: 'Enter Agent wallet private key',
    hyperliquidAgentPrivateKeyDesc:
      'Agent wallet private key for signing transactions (keep balance near 0 for security)',
    hyperliquidMainWalletAddress: 'Main Wallet Address',
    enterHyperliquidMainWalletAddress: 'Enter Main wallet address',
    hyperliquidMainWalletAddressDesc:
      'Main wallet address that holds your trading funds (never expose its private key)',
    // Aster API Pro Configuration
    asterApiProTitle: 'Aster API Pro Wallet Configuration',
    asterApiProDesc:
      'Use API Pro wallet for secure trading: API wallet signs transactions, main wallet holds funds (never expose main wallet private key)',
    asterUserDesc:
      'Main wallet address - The EVM wallet address you use to log in to Aster (Note: Only EVM wallets are supported)',
    asterSignerDesc:
      'API Pro wallet address (0x...) - Generate from https://www.asterdex.com/en/api-wallet',
    asterPrivateKeyDesc:
      'API Pro wallet private key - Get from https://www.asterdex.com/en/api-wallet (only used locally for signing, never transmitted)',
    asterUsdtWarning:
      'Important: Aster only tracks USDT balance. Please ensure you use USDT as margin currency to avoid P&L calculation errors caused by price fluctuations of other assets (BNB, ETH, etc.)',
    asterUserLabel: 'Main Wallet Address',
    asterSignerLabel: 'API Pro Wallet Address',
    asterPrivateKeyLabel: 'API Pro Wallet Private Key',
    enterAsterUser: 'Enter main wallet address (0x...)',
    enterAsterSigner: 'Enter API Pro wallet address (0x...)',
    enterAsterPrivateKey: 'Enter API Pro wallet private key',

    // LIGHTER Configuration
    lighterWalletAddress: 'L1 Wallet Address',
    lighterPrivateKey: 'L1 Private Key',
    lighterApiKeyPrivateKey: 'API Key Private Key',
    enterLighterWalletAddress: 'Enter Ethereum wallet address (0x...)',
    enterLighterPrivateKey: 'Enter L1 private key (32 bytes)',
    enterLighterApiKeyPrivateKey:
      'Enter API Key private key (40 bytes, optional)',
    lighterWalletAddressDesc:
      'Your Ethereum wallet address for account identification',
    lighterPrivateKeyDesc:
      'L1 private key for account identification (32-byte ECDSA key)',
    lighterApiKeyPrivateKeyDesc:
      'API Key private key for transaction signing (40-byte Poseidon2 key)',
    lighterApiKeyOptionalNote:
      'Without API Key, system will use limited V1 mode',
    lighterV1Description:
      'Basic Mode - Limited functionality, testing framework only',
    lighterV2Description:
      'Full Mode - Supports Poseidon2 signing and real trading',
    lighterPrivateKeyImported: 'LIGHTER private key imported',

    // Exchange names
    hyperliquidExchangeName: 'Hyperliquid',
    asterExchangeName: 'Aster DEX',

    // Secure input
    secureInputButton: 'Secure Input',
    secureInputReenter: 'Re-enter Securely',
    secureInputClear: 'Clear',
    secureInputHint:
      'Captured via secure two-step input. Use "Re-enter Securely" to update this value.',

    // Two Stage Key Modal
    twoStageModalTitle: 'Secure Key Input',
    twoStageModalDescription:
      'Use a two-step flow to enter your {length}-character private key safely.',
    twoStageStage1Title: 'Step 1 · Enter the first half',
    twoStageStage1Placeholder: 'First 32 characters (include 0x if present)',
    twoStageStage1Hint:
      'Continuing copies an obfuscation string to your clipboard as a diversion.',
    twoStageStage1Error: 'Please enter the first part before continuing.',
    twoStageNext: 'Next',
    twoStageProcessing: 'Processing…',
    twoStageCancel: 'Cancel',
    twoStageStage2Title: 'Step 2 · Enter the rest',
    twoStageStage2Placeholder: 'Remaining characters of your private key',
    twoStageStage2Hint:
      'Paste the obfuscation string somewhere neutral, then finish entering your key.',
    twoStageClipboardSuccess:
      'Obfuscation string copied. Paste it into any text field once before completing.',
    twoStageClipboardReminder:
      'Remember to paste the obfuscation string before submitting to avoid clipboard leaks.',
    twoStageClipboardManual:
      'Automatic copy failed. Copy the obfuscation string below manually.',
    twoStageBack: 'Back',
    twoStageSubmit: 'Confirm',
    twoStageInvalidFormat:
      'Invalid private key format. Expected {length} hexadecimal characters (optional 0x prefix).',
    testnetDescription:
      'Enable to connect to exchange test environment for simulated trading',
    securityWarning: 'Security Warning',
    saveConfiguration: 'Save Configuration',

    // Trader Configuration
    positionMode: 'Position Mode',
    crossMarginMode: 'Cross Margin',
    isolatedMarginMode: 'Isolated Margin',
    crossMarginDescription:
      'Cross margin: All positions share account balance as collateral',
    isolatedMarginDescription:
      'Isolated margin: Each position manages collateral independently, risk isolation',
    leverageConfiguration: 'Leverage Configuration',
    btcEthLeverage: 'BTC/ETH Leverage',
    altcoinLeverage: 'Altcoin Leverage',
    leverageRecommendation:
      'Recommended: BTC/ETH 5-10x, Altcoins 3-5x for risk control',
    tradingSymbols: 'Trading Symbols',
    tradingSymbolsPlaceholder:
      'Enter symbols, comma separated (e.g., BTCUSDT,ETHUSDT,SOLUSDT)',
    selectSymbols: 'Select Symbols',
    selectTradingSymbols: 'Select Trading Symbols',
    selectedSymbolsCount: 'Selected {count} symbols',
    clearSelection: 'Clear All',
    confirmSelection: 'Confirm',
    tradingSymbolsDescription:
      'Empty = use default symbols. Use USDT perps (e.g., BTCUSDT, ETHUSDT) or Hyperliquid XYZ USDC markets (e.g., TSLA-USDC)',
    btcEthLeverageValidation: 'BTC/ETH leverage must be between 1-50x',
    altcoinLeverageValidation: 'Altcoin leverage must be between 1-20x',
    invalidSymbolFormat:
      'Invalid symbol format: {symbol}, use USDT perps or SYMBOL-USDC',

    // System Prompt Templates
    systemPromptTemplate: 'System Prompt Template',
    promptTemplateDefault: 'Default Stable',
    promptTemplateAdaptive: 'Conservative Strategy',
    promptTemplateAdaptiveRelaxed: 'Aggressive Strategy',
    promptTemplateHansen: 'Hansen Strategy',
    promptTemplateNof1: 'NoF1 English Framework',
    promptTemplateTaroLong: 'Taro Long Position',
    promptDescDefault: '📊 Default Stable Strategy',
    promptDescDefaultContent:
      'Maximize Sharpe ratio, balanced risk-reward, suitable for beginners and stable long-term trading',
    promptDescAdaptive: '🛡️ Conservative Strategy (v6.0.0)',
    promptDescAdaptiveContent:
      'Strict risk control, BTC mandatory confirmation, high win rate priority, suitable for conservative traders',
    promptDescAdaptiveRelaxed: '⚡ Aggressive Strategy (v6.0.0)',
    promptDescAdaptiveRelaxedContent:
      'High-frequency trading, BTC optional confirmation, pursue trading opportunities, suitable for volatile markets',
    promptDescHansen: '🎯 Hansen Strategy',
    promptDescHansenContent:
      'Hansen custom strategy, maximize Sharpe ratio, for professional traders',
    promptDescNof1: '🌐 NoF1 English Framework',
    promptDescNof1Content:
      'Hyperliquid exchange specialist, English prompts, maximize risk-adjusted returns',
    promptDescTaroLong: '📈 Taro Long Position Strategy',
    promptDescTaroLongContent:
      'Data-driven decisions, multi-dimensional validation, continuous learning evolution, long position specialist',

    // Loading & Error
    loading: 'Loading...',

    // AI Traders Page - Additional
    inUse: 'In Use',
    noModelsConfigured: 'No configured AI models',
    noExchangesConfigured: 'No configured exchanges',
    signalSource: 'Signal Source',
    signalSourceConfig: 'Signal Source Configuration',
    ai500Description:
      'API endpoint for AI500 data provider, leave blank to disable this signal source',
    oiTopDescription:
      'API endpoint for open interest rankings, leave blank to disable this signal source',
    information: 'Information',
    signalSourceInfo1:
      '• Signal source configuration is per-user, each user can set their own URLs',
    signalSourceInfo2:
      '• When creating traders, you can choose whether to use these signal sources',
    signalSourceInfo3:
      '• Configured URLs will be used to fetch market data and trading signals',
    editAIModel: 'Edit AI Model',
    addAIModel: 'Add AI Model',
    confirmDeleteModel:
      'Are you sure you want to delete this AI model configuration?',
    cannotDeleteModelInUse:
      'Cannot delete this AI model because it is being used by traders',
    tradersUsing: 'Traders using this configuration',
    pleaseDeleteTradersFirst:
      'Please delete or reconfigure these traders first',
    selectModel: 'Select AI Model',
    pleaseSelectModel: 'Please select a model',
    customBaseURL: 'Base URL (Optional)',
    customBaseURLPlaceholder:
      'Custom API base URL, e.g.: https://api.openai.com/v1',
    leaveBlankForDefault: 'Leave blank to use default API address',
    modelConfigInfo1:
      '• For official API, only API Key is required, leave other fields blank',
    modelConfigInfo2:
      '• Custom Base URL and Model Name only needed for third-party proxies',
    modelConfigInfo3: '• API Key is encrypted and stored securely',
    defaultModel: 'Default model',
    applyApiKey: 'Apply API Key',
    kimiApiNote:
      'Kimi requires API Key from international site (moonshot.ai), China region keys are not compatible',
    leaveBlankForDefaultModel: 'Leave blank to use default model',
    customModelName: 'Model Name (Optional)',
    customModelNamePlaceholder: 'e.g.: deepseek-chat, qwen3-max, gpt-4o',
    saveConfig: 'Save Configuration',
    editExchange: 'Edit Exchange',
    addExchange: 'Add Exchange',
    confirmDeleteExchange:
      'Are you sure you want to delete this exchange configuration?',
    cannotDeleteExchangeInUse:
      'Cannot delete this exchange because it is being used by traders',
    pleaseSelectExchange: 'Please select an exchange',
    exchangeConfigWarning1:
      '• API keys will be encrypted, recommend using read-only or futures trading permissions',
    exchangeConfigWarning2:
      '• Do not grant withdrawal permissions to ensure fund security',
    exchangeConfigWarning3:
      '• After deleting configuration, related traders will not be able to trade',
    edit: 'Edit',
    viewGuide: 'View Guide',
    binanceSetupGuide: 'Binance Setup Guide',
    closeGuide: 'Close',
    whitelistIP: 'Whitelist IP',
    whitelistIPDesc: 'Binance requires adding server IP to API whitelist',
    serverIPAddresses: 'Server IP Addresses',
    copyIP: 'Copy',
    ipCopied: 'IP Copied',
    copyIPFailed: 'Failed to copy IP address. Please copy manually',
    loadingServerIP: 'Loading server IP...',

    // Error Messages
    createTraderFailed: 'Failed to create trader',
    getTraderConfigFailed: 'Failed to get trader configuration',
    modelConfigNotExist: 'Model configuration does not exist or is not enabled',
    exchangeConfigNotExist:
      'Exchange configuration does not exist or is not enabled',
    updateTraderFailed: 'Failed to update trader',
    deleteTraderFailed: 'Failed to delete trader',
    operationFailed: 'Operation failed',
    deleteConfigFailed: 'Failed to delete configuration',
    modelNotExist: 'Model does not exist',
    saveConfigFailed: 'Failed to save configuration',
    exchangeNotExist: 'Exchange does not exist',
    deleteExchangeConfigFailed: 'Failed to delete exchange configuration',
    saveSignalSourceFailed: 'Failed to save signal source configuration',
    encryptionFailed: 'Failed to encrypt sensitive data',

    // Login & Register
    login: 'Sign In',
    register: 'Sign Up',
    username: 'Username',
    email: 'Email',
    password: 'Password',
    confirmPassword: 'Confirm Password',
    usernamePlaceholder: 'your username',
    emailPlaceholder: 'your@email.com',
    passwordPlaceholder: 'Enter your password',
    confirmPasswordPlaceholder: 'Re-enter your password',
    passwordRequirements: 'Password requirements',
    passwordRuleMinLength: 'Minimum 8 characters',
    passwordRuleUppercase: 'At least 1 uppercase letter',
    passwordRuleLowercase: 'At least 1 lowercase letter',
    passwordRuleNumber: 'At least 1 number',
    passwordRuleSpecial: 'At least 1 special character (@#$%!&*?)',
    passwordRuleMatch: 'Passwords match',
    passwordNotMeetRequirements:
      'Password does not meet the security requirements',
    loginTitle: 'Sign in to your account',
    registerTitle: 'Create a new account',
    loginButton: 'Sign In',
    registerButton: 'Sign Up',
    back: 'Back',
    noAccount: "Don't have an account?",
    hasAccount: 'Already have an account?',
    registerNow: 'Sign up now',
    loginNow: 'Sign in now',
    forgotPassword: 'Forgot password?',
    forgotAccount: 'Forgot account?',
    forgotAccountConfirm:
      '⚠️ This will permanently delete EVERYTHING: users, traders, strategies, AI model API keys, exchange API keys, and your CLAW402 wallet. Export anything you need to keep (especially wallet private keys) BEFORE continuing. Re-registration will NOT restore them. Continue?',
    forgotAccountSuccess:
      'Account reset successful! You can now register a new account.',
    rememberMe: 'Remember me',
    resetPassword: 'Reset Password',
    resetPasswordTitle: 'Reset your password',
    newPassword: 'New Password',
    newPasswordPlaceholder: 'Enter new password (at least 6 characters)',
    resetPasswordButton: 'Reset Password',
    resetPasswordSuccess:
      'Password reset successful! Please login with your new password',
    resetPasswordFailed: 'Password reset failed',
    backToLogin: 'Back to Login',
    resetPasswordCliIntro:
      'For security, password recovery is no longer available from the browser. Run this command on the server where NOFX is installed:',
    resetPasswordCliSecurityNote:
      'This requires shell access to the server, which keeps your account safe even when NOFX is exposed to the internet.',
    resetAccountCliIntro:
      'To wipe everything and start over, run this command on the server where NOFX is installed:',
    copy: 'Copy',
    loginSuccess: 'Login successful',
    registrationSuccess: 'Registration successful',
    loginFailed: 'Login failed. Please check your email and password.',
    registrationFailed: 'Registration failed. Please try again.',
    sessionExpired: 'Session expired, please login again',
    invalidCredentials: 'Invalid email or password',
    weak: 'Weak',
    medium: 'Medium',
    strong: 'Strong',
    passwordStrength: 'Password strength',
    passwordStrengthHint:
      'Use at least 8 characters with mix of letters, numbers and symbols',
    passwordMismatch: 'Passwords do not match',
    emailRequired: 'Email is required',
    passwordRequired: 'Password is required',
    invalidEmail: 'Invalid email format',
    passwordTooShort: 'Password must be at least 6 characters',

    // Landing Page
    features: 'Features',
    howItWorks: 'How it Works',
    community: 'Community',
    language: 'Language',
    loggedInAs: 'Logged in as',
    exitLogin: 'Sign Out',
    signIn: 'Sign In',
    signUp: 'Sign Up',
    registrationClosed: 'Registration Closed',
    registrationClosedMessage:
      'User registration is currently disabled. Please contact the administrator for access.',

    // Hero Section
    githubStarsInDays: '2.5K+ GitHub Stars in 3 days',
    heroTitle1: 'Read the Market.',
    heroTitle2: 'Write the Trade.',
    heroDescription:
      'NOFX is the future standard for AI trading — an open, community-driven agentic trading OS. Supporting Binance, Aster DEX and other exchanges, self-hosted, multi-agent competition, let AI automatically make decisions, execute and optimize trades for you.',
    poweredBy: 'Powered by Aster DEX and Binance.',

    // Landing Page CTA
    readyToDefine: 'Ready to define the future of AI trading?',
    startWithCrypto:
      'Starting with crypto markets, expanding to TradFi. NOFX is the infrastructure of AgentFi.',
    getStartedNow: 'Get Started Now',
    viewSourceCode: 'View Source Code',

    // Features Section
    coreFeatures: 'Core Features',
    whyChooseNofx: 'Why Choose NOFX?',
    openCommunityDriven:
      'Open source, transparent, community-driven AI trading OS',
    openSourceSelfHosted: '100% Open Source & Self-Hosted',
    openSourceDesc:
      'Your framework, your rules. Non-black box, supports custom prompts and multi-models.',
    openSourceFeatures1: 'Fully open source code',
    openSourceFeatures2: 'Self-hosting deployment support',
    openSourceFeatures3: 'Custom AI prompts',
    openSourceFeatures4: 'Multi-model support (DeepSeek, Qwen)',
    multiAgentCompetition: 'Multi-Agent Intelligent Competition',
    multiAgentDesc:
      'AI strategies battle at high speed in sandbox, survival of the fittest, achieving strategy evolution.',
    multiAgentFeatures1: 'Multiple AI agents running in parallel',
    multiAgentFeatures2: 'Automatic strategy optimization',
    multiAgentFeatures3: 'Sandbox security testing',
    multiAgentFeatures4: 'Cross-market strategy porting',
    secureReliableTrading: 'Secure and Reliable Trading',
    secureDesc:
      'Enterprise-grade security, complete control over your funds and trading strategies.',
    secureFeatures1: 'Local private key management',
    secureFeatures2: 'Fine-grained API permission control',
    secureFeatures3: 'Real-time risk monitoring',
    secureFeatures4: 'Trading log auditing',

    // About Section
    aboutNofx: 'About NOFX',
    whatIsNofx: 'What is NOFX?',
    nofxNotAnotherBot:
      "NOFX is not another trading bot, but the 'Linux' of AI trading —",
    nofxDescription1:
      'a transparent, trustworthy open source OS that provides a unified',
    nofxDescription2:
      "'decision-risk-execution' layer, supporting all asset classes.",
    nofxDescription3:
      'Starting with crypto markets (24/7, high volatility perfect testing ground), future expansion to stocks, futures, forex. Core: open architecture, AI',
    nofxDescription4:
      'Darwinism (multi-agent self-competition, strategy evolution), CodeFi',
    nofxDescription5:
      'flywheel (developers get point rewards for PR contributions).',
    youFullControl: 'You 100% Control',
    fullControlDesc: 'Complete control over AI prompts and funds',
    startupMessages1: 'Starting automated trading system...',
    startupMessages2: 'API server started on port 8080',
    startupMessages3: 'Web console http://127.0.0.1:3000',

    // How It Works Section
    howToStart: 'How to Get Started with NOFX',
    fourSimpleSteps:
      'Four simple steps to start your AI automated trading journey',
    step1Title: 'Clone GitHub Repository',
    step1Desc:
      'git clone https://github.com/NoFxAiOS/nofx and switch to dev branch to test new features.',
    step2Title: 'Configure Environment',
    step2Desc:
      'Frontend setup for exchange APIs (like Binance, Hyperliquid), AI models and custom prompts.',
    step3Title: 'Deploy & Run',
    step3Desc:
      'One-click Docker deployment, start AI agents. Note: High-risk market, only test with money you can afford to lose.',
    step4Title: 'Optimize & Contribute',
    step4Desc:
      'Monitor trading, submit PRs to improve framework. Join Telegram to share strategies.',
    importantRiskWarning: 'Important Risk Warning',
    riskWarningText:
      'Dev branch is unstable, do not use funds you cannot afford to lose. NOFX is non-custodial, no official strategies. Trading involves risks, invest carefully.',

    // Community Section (testimonials are kept as-is since they are quotes)

    // Footer Section
    futureStandardAI: 'The future standard of AI trading',
    links: 'Links',
    resources: 'Resources',
    documentation: 'Documentation',
    supporters: 'Supporters',
    strategicInvestment: '(Strategic Investment)',

    // Login Modal
    accessNofxPlatform: 'Access NOFX Platform',
    loginRegisterPrompt:
      'Please login or register to access the full AI trading platform',
    registerNewAccount: 'Register New Account',

    // Candidate Coins Warnings
    candidateCoins: 'Candidate Coins',
    candidateCoinsZeroWarning: 'Candidate Coins Count is 0',
    possibleReasons: 'Possible Reasons:',
    ai500ApiNotConfigured:
      'AI500 data provider API not configured or inaccessible (check signal source settings)',
    apiConnectionTimeout: 'API connection timeout or returned empty data',
    noCustomCoinsAndApiFailed:
      'No custom coins configured and API fetch failed',
    solutions: 'Solutions:',
    setCustomCoinsInConfig: 'Set custom coin list in trader configuration',
    orConfigureCorrectApiUrl: 'Or configure correct data provider API address',
    orDisableAI500Options:
      'Or disable "Use AI500 Data Provider" and "Use OI Top" options',
    signalSourceNotConfigured: 'Signal Source Not Configured',
    signalSourceWarningMessage:
      'You have traders that enabled "Use AI500 Data Provider" or "Use OI Top", but signal source API address is not configured yet. This will cause candidate coins count to be 0, and traders cannot work properly.',
    configureSignalSourceNow: 'Configure Signal Source Now',

    // FAQ Page

    // FAQ Categories

    // ===== GETTING STARTED =====






    // ===== INSTALLATION =====






    // ===== CONFIGURATION =====






    // ===== TRADING =====








    // ===== TECHNICAL ISSUES =====








    // ===== SECURITY =====




    // ===== FEATURES =====



    // ===== AI MODELS =====




    // ===== CONTRIBUTING =====




    // Web Crypto Environment Check
    environmentCheck: {
      button: 'Check Secure Environment',
      checking: 'Checking...',
      description:
        'Automatically verifying whether this browser context allows Web Crypto before entering sensitive keys.',
      secureTitle: 'Secure context detected',
      secureDesc:
        'Web Crypto API is available. You can continue entering secrets with encryption enabled.',
      insecureTitle: 'Insecure context detected',
      insecureDesc:
        'This page is not running over HTTPS or a trusted localhost origin, so browsers block Web Crypto calls.',
      tipsTitle: 'How to fix:',
      tipHTTPS:
        'Serve the dashboard over HTTPS with a valid certificate (IP origins also need TLS).',
      tipLocalhost:
        'During development, open the app via http://localhost or 127.0.0.1.',
      tipIframe:
        'Avoid embedding the app in insecure HTTP iframes or reverse proxies that strip HTTPS.',
      unsupportedTitle: 'Browser does not expose Web Crypto',
      unsupportedDesc:
        'Open NOFX over HTTPS (or http://localhost during development) and avoid insecure iframes/reverse proxies so the browser can enable Web Crypto.',
      summary: 'Current origin: {origin} • Protocol: {protocol}',
      disabledTitle: 'Transport encryption disabled',
      disabledDesc:
        'Server-side transport encryption is disabled. API keys will be transmitted in plaintext. Enable TRANSPORT_ENCRYPTION=true for enhanced security.',
    },

    environmentSteps: {
      checkTitle: '1. Environment check',
      selectTitle: '2. Select exchange',
    },

    // Two-Stage Key Modal
    twoStageKey: {
      title: 'Two-Stage Private Key Input',
      stage1Description:
        'Enter the first {length} characters of your private key',
      stage2Description:
        'Enter the remaining {length} characters of your private key',
      stage1InputLabel: 'First Part',
      stage2InputLabel: 'Second Part',
      characters: 'characters',
      processing: 'Processing...',
      nextButton: 'Next',
      cancelButton: 'Cancel',
      backButton: 'Back',
      encryptButton: 'Encrypt & Submit',
      obfuscationCopied: 'Obfuscation data copied to clipboard',
      obfuscationInstruction:
        'Paste something else to clear clipboard, then continue',
      obfuscationManual: 'Manual obfuscation required',
    },

    // Error Messages
    errors: {
      privatekeyIncomplete: 'Please enter at least {expected} characters',
      privatekeyInvalidFormat:
        'Invalid private key format (should be 64 hex characters)',
      privatekeyObfuscationFailed: 'Clipboard obfuscation failed',
    },

    // Position History
    positionHistory: {
      title: 'Position History',
      loading: 'Loading position history...',
      noHistory: 'No Position History',
      noHistoryDesc: 'Closed positions will appear here after trading.',
      showingPositions: 'Showing {count} of {total} positions',
      totalPnL: 'Total P&L',
      // Stats
      totalTrades: 'Total Trades',
      winLoss: 'Win: {win} / Loss: {loss}',
      winRate: 'Win Rate',
      profitFactor: 'Profit Factor',
      profitFactorDesc: 'Total Profit / Total Loss',
      plRatio: 'P/L Ratio',
      plRatioDesc: 'Avg Win / Avg Loss',
      sharpeRatio: 'Sharpe Ratio',
      sharpeRatioDesc: 'Risk-adjusted Return',
      maxDrawdown: 'Max Drawdown',
      avgWin: 'Avg Win',
      avgLoss: 'Avg Loss',
      netPnL: 'Net P&L',
      netPnLDesc: 'After Fees',
      fee: 'Fee',
      // Direction Stats
      trades: 'Trades',
      avgPnL: 'Avg P&L',
      // Symbol Performance
      symbolPerformance: 'Symbol Performance',
      // Filters
      symbol: 'Symbol',
      allSymbols: 'All Symbols',
      side: 'Side',
      all: 'All',
      sort: 'Sort',
      latestFirst: 'Latest First',
      oldestFirst: 'Oldest First',
      highestPnL: 'Highest P&L',
      lowestPnL: 'Lowest P&L',
      // Table Headers
      entry: 'Entry',
      exit: 'Exit',
      qty: 'Qty',
      value: 'Value',
      lev: 'Lev',
      pnl: 'P&L',
      duration: 'Duration',
      closedAt: 'Closed At',
    },

    // Data Page
    dataCenter: 'Data Center',

    // Strategy Market Page
    strategyMarket: {
      title: 'STRATEGY MARKET',
      subtitle: 'GLOBAL STRATEGY DATABASE',
      description:
        'Discover, analyze, and clone high-performance trading algorithms',
      search: 'SEARCH PARAMETERS...',
      all: 'ALL PROTOCOLS',
      popular: 'TRENDING',
      recent: 'LATEST',
      myStrategies: 'MY LIBRARY',
      noStrategies: 'NO SIGNAL',
      noStrategiesDesc: 'No strategic signals detected in this frequency',
      author: 'OPERATOR',
      createdAt: 'TIMESTAMP',
      viewConfig: 'DECRYPT CONFIG',
      hideConfig: 'ENCRYPT',
      copyConfig: 'CLONE CONFIG',
      copied: 'COPIED',
      configHidden: 'ENCRYPTED',
      configHiddenDesc: 'Configuration parameters encrypted',
      indicators: 'INDICATORS',
      maxPositions: 'POS_LIMIT',
      maxLeverage: 'LEV_MAX',
      shareYours: 'UPLOAD_STRATEGY',
      makePublic: 'PUBLISH',
      loading: 'INITIALIZING...',
    },

    // Strategy Studio Page
    strategyStudio: {
      title: 'Strategy Studio',
      subtitle: 'Configure and test trading strategies',
      strategies: 'Strategies',
      newStrategy: 'New',
      strategyType: 'Strategy Type',
      aiTrading: 'AI Trading',
      aiTradingDesc: 'AI analyzes market and makes trading decisions',
      gridTrading: 'AI Grid Trading',
      gridTradingDesc: 'AI-controlled grid strategy for ranging markets',
      gridConfig: 'Grid Configuration',
      coinSource: 'Coin Source',
      indicators: 'Indicators',
      riskControl: 'Risk Control',
      promptSections: 'Prompt Editor',
      customPrompt: 'Extra Prompt',
      save: 'Save',
      saving: 'Saving...',
      activate: 'Activate',
      active: 'Active',
      default: 'Default',
      promptPreview: 'Prompt Preview',
      aiTestRun: 'AI Test',
      systemPrompt: 'System Prompt',
      userPrompt: 'User Prompt',
      loadPrompt: 'Generate Prompt',
      refreshPrompt: 'Refresh',
      promptVariant: 'Style',
      balanced: 'Balanced',
      aggressive: 'Aggressive',
      conservative: 'Conservative',
      selectModel: 'Select AI Model',
      runTest: 'Run AI Test',
      running: 'Running...',
      aiOutput: 'AI Output',
      reasoning: 'Reasoning',
      decisions: 'Decisions',
      duration: 'Duration',
      noModel: 'Please configure AI model first',
      testNote: 'Test with real AI, no trading',
      publishSettings: 'Publish',
      newStrategyName: 'New Strategy',
      strategyCopy: 'Strategy Copy',
      strategyDeleted: 'Strategy deleted',
      cannotDeleteActiveStrategy: 'Active strategy cannot be deleted',
      confirmDeleteStrategy: 'Delete this strategy?',
      confirmDelete: 'Confirm Delete',
      delete: 'Delete',
      cancel: 'Cancel',
      strategyExported: 'Strategy exported',
      invalidStrategyFile: 'Invalid strategy file',
      imported: 'Imported',
      strategyImported: 'Strategy imported',
      strategySaved: 'Strategy saved',
      importStrategy: 'Import Strategy',
      newStrategyTooltip: 'New Strategy',
      export: 'Export',
      duplicate: 'Duplicate',
      deleteTooltip: 'Delete',
      public: 'Public',
      addDescription: 'Add strategy description...',
      unsaved: 'Unsaved',
      discardChanges: 'Discard',
      selectOrCreate: 'Select or create a strategy',
      customPromptDesc:
        'Extra prompt appended to System Prompt for personalized trading style',
      customPromptPlaceholder: 'Enter custom prompt...',
      generatePromptPreview: 'Click to generate prompt preview',
      runAiTestHint: 'Click to run AI test',
      tokenEstimate: 'Token Estimate',
      tokenExceedWarning:
        'Token estimate exceeds 128K. AI requests may fail for some models.',
      tokenEstimating: 'Estimating...',
      tokenTooltip: 'Based on 200K context',
    },

    // Metric Tooltip
    metricTooltip: {
      formula: 'Formula',
    },

    // Login Required Overlay
    loginRequired: {
      title: 'SYSTEM ACCESS DENIED',
      accessDenied: 'ACCESS DENIED',
      subtitleWithFeature:
        'Module "{featureName}" requires elevated privileges',
      subtitleDefault: 'Authorization required for this module',
      description:
        'Initialize authentication protocol to unlock full system capabilities: AI Trader configuration and Strategy Market data streams.',
      benefit1: 'AI Trader Control',
      benefit2: 'HFT Strategy Market',
      benefit4: 'Full System Visualization',
      loginButton: 'EXECUTE LOGIN',
      registerButton: 'REGISTER NEW ID',
      abort: 'ABORT',
    },

    // Advanced Chart
    advancedChart: {
      updating: 'Updating...',
      indicators: 'Indicators',
      orderMarkers: 'Order Markers',
      technicalIndicators: 'Technical Indicators',
      clickToToggle: 'Click to toggle indicators',
      shares: 'shares',
      units: 'units',
    },

    // Chart With Orders
    chartWithOrders: {
      failedToLoad: 'Failed to load chart data',
      loading: 'Loading...',
      buy: 'BUY',
      sell: 'SELL',
    },

    // Comparison Chart
    comparisonChart: {
      '1d': '1D',
      '3d': '3D',
      '7d': '7D',
      '30d': '30D',
      all: 'All',
    },

    // TraderDashboardPage
    traderDashboard: {
      connectionFailed: 'Connection Failed',
      connectionFailedDesc: 'Please check if the backend service is running.',
      retry: 'Retry',
      confirmClosePosition:
        'Are you sure you want to close {symbol} {side} position?',
      confirmClose: 'Confirm Close',
      confirm: 'Confirm',
      cancel: 'Cancel',
      positionClosed: 'Position closed successfully',
      closeFailed: 'Failed to close position',
      hideAddress: 'Hide address',
      showFullAddress: 'Show full address',
      copyAddress: 'Copy address',
      noAddressConfigured: 'No address configured',
      action: 'Action',
      entry: 'Entry',
      mark: 'Mark',
      qty: 'Qty',
      value: 'Value',
      lev: 'Lev.',
      uPnL: 'uPnL',
      liq: 'Liq.',
      closePosition: 'Close Position',
      close: 'Close',
      showingPositions: 'Showing {shown} of {total} positions',
      perPage: 'Per page',
      accountFetchFailed:
        'DATA_FETCH::FAILED — Account data unavailable, check connection',
      positionsFetchFailed: 'Position data unavailable',
      decisionsFetchFailed: 'Decision data unavailable',
    },

    // AITradersPage toast messages
    aiTradersToast: {
      creating: 'Creating...',
      created: 'Created successfully',
      createFailed: 'Creation failed',
      saving: 'Saving...',
      saved: 'Saved successfully',
      saveFailed: 'Save failed',
      deleting: 'Deleting...',
      deleted: 'Deleted successfully',
      deleteFailed: 'Deletion failed',
      stopping: 'Stopping...',
      stopped: 'Stopped',
      stopFailed: 'Stop failed',
      starting: 'Starting...',
      started: 'Started',
      startFailed: 'Start failed',
      updating: 'Updating...',
      updatingConfig: 'Updating config...',
      configUpdated: 'Config updated',
      configUpdateFailed: 'Config update failed',
      showInCompetition: 'Shown in competition',
      hideInCompetition: 'Hidden from competition',
      updateFailed: 'Update failed',
      updatingModelConfig: 'Updating model config...',
      modelConfigUpdated: 'Model config updated',
      modelConfigUpdateFailed: 'Model config update failed',
      deletingExchange: 'Deleting exchange account...',
      exchangeDeleted: 'Exchange account deleted',
      exchangeDeleteFailed: 'Failed to delete exchange account',
      updatingExchangeConfig: 'Updating exchange config...',
      exchangeConfigUpdated: 'Exchange config updated',
      exchangeConfigUpdateFailed: 'Failed to update exchange config',
      creatingExchange: 'Creating exchange account...',
      exchangeCreated: 'Exchange account created',
      exchangeCreateFailed: 'Failed to create exchange account',
    },

    // ModelConfigModal
    modelConfig: {
      selectModel: 'Select Model',
      configure: 'Configure',
      configureApi: 'Configure API',
      configureWallet: 'Configure Wallet',
      chooseProvider: 'Choose Your AI Provider',
      claw402EntryDesc:
        'Recommended default path. Use Base USDC pay-per-call instead of managing API keys.',
      otherApiEntry: 'Other API Providers',
      otherApiEntryDesc:
        'Use your own API key for OpenAI, Claude, Gemini, DeepSeek, and more.',
      payPerCall: 'Pay-per-call USDC · All AI Models · No API Key',
      recommended: 'Best',
      allModelsClaw: 'Pay-per-call with USDC — supports all major AI models',
      selectAiModel: 'Choose AI Model',
      allModelsUnified:
        'All models unified via Claw402. Switch anytime after setup.',
      setupWallet: 'Setup Wallet',
      walletInfo: 'Claw402 uses USDC on Base chain. You need an EVM wallet.',
      exportKey: 'Export private key from MetaMask, Rabby, etc.',
      dedicatedWallet:
        'Recommended: create a dedicated wallet with a small USDC balance',
      walletPrivateKey: 'Wallet Private Key (Base Chain EVM)',
      privateKeyNote:
        'Private key is only used locally for signing. Never uploaded. No ETH or gas needed.',
      howToFundUsdc: 'How to Fund USDC',
      fundStep1:
        'Withdraw USDC from exchange (Binance/OKX/Coinbase) to your wallet',
      fundStep2: 'Select Base network (very low fees)',
      fundStep3: '$5-10 USDC lasts a long time (~$0.003/call)',
      back: 'Back',
      startTrading: 'Start Trading',
      modelsConfigured: 'Models with gold badge are already configured',
      getStarted: 'Get Started',
      getApiKey: 'Get API Key',
      walletPrivateKeyLabel: 'Wallet Private Key *',
      selectModelLabel: 'Select Model',
      validating: 'Validating...',
      walletAddress: 'Wallet Address',
      usdcBalance: 'Base USDC Balance',
      claw402Connected: 'claw402 Connected',
      claw402Unreachable: 'claw402 Unreachable',
      depositUsdc: 'Deposit USDC to this address on Base chain',
      invalidKeyPrefix: 'Please add 0x at the beginning',
      invalidKeyLength: 'Should be 66 characters, currently',
      invalidKeyChars: 'Contains invalid characters',
      testConnection: 'Test Connection',
      testingConnection: 'Testing...',
    },

    // ExchangeConfigModal
    exchangeConfig: {
      selectExchange: 'Select Exchange',
      configure: 'Configure',
      chooseExchange: 'Choose Your Exchange',
      centralizedExchanges: 'Centralized Exchanges',
      decentralizedExchanges: 'Decentralized Exchanges',
      register: 'Register',
      bonus: 'Bonus',
      accountName: 'Account Name',
      accountNamePlaceholder: 'e.g., Main Account',
      pleaseEnterAccountName: 'Please enter account name',
      useBinanceFuturesApi: 'Use "Spot & Futures Trading" API',
      viewTutorial: 'View Tutorial',
      lighterApiKeySetup: 'Lighter API Key Setup',
      lighterApiKeyDesc: 'Generate an API Key on Lighter website',
      apiKeyIndex: 'API Key Index',
      apiKeyIndexTooltip: 'API Key index starts from 0',
      back: 'Back',
    },

    // TelegramConfigModal
    telegram: {
      botSetup: 'Telegram Bot Setup',
      createBot: 'Create Bot',
      bindAccount: 'Bind Account',
      done: 'Done',
      invalidTokenFormat:
        'Invalid Bot Token format. Expected "numbers:alphanumeric"',
      tokenSaved: 'Bot Token saved, waiting for binding',
      saveFailed: 'Save failed, please verify the token',
      unbound: 'Telegram account unbound',
      unbindFailed: 'Unbind failed',
      step1Title: 'Step 1: Create your Bot in Telegram',
      step1Desc1: 'Open Telegram, search for',
      step1Desc2: 'Send',
      step1Desc2Suffix: 'command',
      step1Desc3: 'Follow prompts to set bot name and username',
      step1Desc4: 'BotFather will return a Token, copy it',
      openBotFather: 'Open @BotFather',
      pasteToken: 'Paste Bot Token',
      tokenFormat: 'Format: numbers:alphanumeric, e.g. 123456789:ABCdef...',
      selectAiModel: 'Select AI Model (optional)',
      noEnabledModels: 'No enabled models. Configure one in AI Models first.',
      autoSelect: '— Auto-select (recommended)',
      autoUseEnabled: 'Leave blank to auto-use any enabled model',
      savingToken: 'Saving...',
      saveAndContinue: 'Save & Continue',
      step2Title: 'Step 2: Send /start to your Bot',
      step2Desc1: 'Search for your newly created Bot in Telegram',
      step2Desc2: 'Click Start or send',
      step2Desc3: 'Bot will automatically bind to your account',
      currentToken: 'Current Token',
      waitingForStart:
        'Waiting for you to send /start... Refresh page after sending',
      reconfigureToken: 'Reconfigure Token',
      bindSuccess: 'Bound successfully!',
      noStartReceived:
        'No /start received yet. Please send /start to your Bot first',
      checkFailed: 'Check failed',
      checkStatus: 'Check Status',
      botActive: 'Telegram Bot is Active!',
      botActiveDesc:
        'You can now control the trading system via natural language in Telegram',
      supportedCommands: 'Supported Commands',
      cmdHelp: 'Show all commands',
      cmdStatus: 'Show trader status',
      cmdNaturalLang: 'Natural language',
      cmdStartStop: 'Start/stop trader',
      cmdControl: 'Natural language control',
      cmdPositions: 'View positions',
      cmdPositionsDesc: 'Real-time position query',
      cmdStrategy: 'Configure strategy',
      cmdStrategyDesc: 'Modify trading strategy',
      unbinding: 'Unbinding...',
      unbindAccount: 'Unbind Account',
      aiModelLabel: 'AI Model (for natural language)',
      aiModelAutoSelect: '— Auto-select',
      modelUpdated: 'AI model updated',
      modelUpdateFailed: 'Update failed',
      save: 'Save',
      loading: 'Loading...',
    },

    // TraderConfigViewModal
    traderConfigView: {
      traderConfig: 'Trader Configuration',
      configInfo: '{name} configuration details',
      running: 'Running',
      stopped: 'Stopped',
      basicInfo: 'Basic Information',
      traderName: 'Trader Name',
      aiModel: 'AI Model',
      exchange: 'Exchange',
      initialBalance: 'Initial Balance',
      marginMode: 'Margin Mode',
      crossMargin: 'Cross',
      isolatedMargin: 'Isolated',
      scanInterval: '{minutes} minutes',
      scanIntervalLabel: 'Scan Interval',
      strategyUsed: 'Strategy Used',
      strategyName: 'Strategy Name',
      close: 'Close',
      yes: 'Yes',
      no: 'No',
    },
  },
  zh: {
    // Header
    appTitle: 'NOFX',
    subtitle: '多AI模型交易平台',
    aiTraders: 'AI交易员',
    details: '详情',
    tradingPanel: '交易面板',
    competition: '竞赛',
    running: '运行中',
    stopped: '已停止',
    adminMode: '管理员模式',
    logout: '退出',
    switchTrader: '切换交易员:',
    view: '查看',

    // Navigation
    realtimeNav: '排行榜',
    configNav: '配置',
    dashboardNav: '看板',
    strategyNav: '策略',
    faqNav: '常见问题',

    // Footer
    footerTitle: 'NOFX - AI交易系统',
    footerWarning: '⚠️ 交易有风险，请谨慎使用。',

    // Stats Cards
    totalEquity: '总净值',
    availableBalance: '可用余额',
    totalPnL: '总盈亏',
    positions: '持仓',
    margin: '保证金',
    free: '空闲',

    // Positions Table
    currentPositions: '当前持仓',
    active: '活跃',
    symbol: '币种',
    side: '方向',
    entryPrice: '入场价',
    stopLoss: '止损',
    takeProfit: '止盈',
    riskReward: '风险回报比',
    markPrice: '标记价',
    quantity: '数量',
    positionValue: '仓位价值',
    leverage: '杠杆',
    unrealizedPnL: '未实现盈亏',
    liqPrice: '强平价',
    long: '多头',
    short: '空头',
    noPositions: '无持仓',
    noActivePositions: '当前没有活跃的交易持仓',

    // Recent Decisions
    recentDecisions: '最近决策',
    lastCycles: '最近 {count} 个交易周期',
    noDecisionsYet: '暂无决策',
    aiDecisionsWillAppear: 'AI交易决策将显示在这里',
    cycle: '周期',
    success: '成功',
    failed: '失败',
    inputPrompt: '输入提示',
    aiThinking: '💭 AI思维链分析',
    collapse: '▼ 收起',
    expand: '▶ 展开',

    // Equity Chart
    accountEquityCurve: '账户净值曲线',
    noHistoricalData: '暂无历史数据',
    dataWillAppear: '运行几个周期后将显示收益率曲线',
    initialBalance: '初始余额',
    currentEquity: '当前净值',
    historicalCycles: '历史周期',
    displayRange: '显示范围',
    recent: '最近',
    allData: '全部数据',
    cycles: '个',

    // Comparison Chart
    comparisonMode: '对比模式',
    dataPoints: '数据点数',
    currentGap: '当前差距',
    count: '{count} 个',

    // TradingView Chart
    marketChart: '行情图表',
    viewChart: '点击查看图表',
    enterSymbol: '输入币种...',
    popularSymbols: '热门币种',
    fullscreen: '全屏',
    exitFullscreen: '退出全屏',

    // Competition Page
    aiCompetition: 'AI竞赛',
    traders: '交易员',
    liveBattle: '实时对战',
    realTimeBattle: '实时对战',
    leader: '领先者',
    leaderboard: '排行榜',
    live: '实时',
    realTime: '实时',
    performanceComparison: '表现对比',
    realTimePnL: '实时收益率',
    realTimePnLPercent: '实时收益率',
    headToHead: '正面对决',
    leadingBy: '领先 {gap}%',
    behindBy: '落后 {gap}%',
    equity: '权益',
    pnl: '收益',
    pos: '持仓',

    // AI Traders Management
    manageAITraders: '管理您的AI交易机器人',
    aiModels: 'AI模型',
    exchanges: '交易所',
    createTrader: '创建交易员',
    modelConfiguration: '模型配置',
    configured: '已配置',
    notConfigured: '未配置',
    currentTraders: '当前交易员',
    noTraders: '暂无AI交易员',
    createFirstTrader: '创建您的第一个AI交易员开始使用',
    dashboardEmptyTitle: '开始使用吧！',
    dashboardEmptyDescription:
      '创建您的第一个 AI 交易员，自动化您的交易策略。连接交易所、选择 AI 模型，几分钟内即可开始交易！',
    goToTradersPage: '创建您的第一个交易员',
    configureModelsFirst: '请先配置AI模型',
    configureExchangesFirst: '请先配置交易所',
    configureModelsAndExchangesFirst: '请先配置AI模型和交易所',
    modelNotConfigured: '所选模型未配置',
    exchangeNotConfigured: '所选交易所未配置',
    confirmDeleteTrader: '确定要删除这个交易员吗？',
    status: '状态',
    start: '启动',
    stop: '停止',
    createNewTrader: '创建新的AI交易员',
    selectAIModel: '选择AI模型',
    selectExchange: '选择交易所',
    traderName: '交易员名称',
    enterTraderName: '输入交易员名称',
    cancel: '取消',
    create: '创建',
    configureAIModels: '配置AI模型',
    configureExchanges: '配置交易所',
    aiScanInterval: 'AI 扫描决策间隔 (分钟)',
    scanIntervalRecommend: '建议: 15-30分钟',
    useTestnet: '使用测试网',
    enabled: '启用',
    save: '保存',

    // TraderConfigModal - New keys for hardcoded Chinese strings
    fetchBalanceEditModeOnly: '只有在编辑模式下才能获取当前余额',
    balanceFetched: '已获取当前余额',
    balanceFetchFailed: '获取余额失败',
    balanceFetchNetworkError: '获取余额失败，请检查网络连接',
    saving: '正在保存…',
    saveSuccess: '保存成功',
    saveFailed: '保存失败',
    editTraderConfig: '修改交易员配置',
    selectStrategyAndConfigParams: '选择策略并配置基础参数',
    basicConfig: '基础配置',
    traderNameRequired: '交易员名称 *',
    enterTraderNamePlaceholder: '请输入交易员名称',
    aiModelRequired: 'AI模型 *',
    exchangeRequired: '交易所 *',
    noExchangeAccount: '还没有交易所账号？点击注册',
    discount: '折扣优惠',
    selectTradingStrategy: '选择交易策略',
    useStrategy: '使用策略',
    noStrategyManual: '-- 不使用策略（手动配置） --',
    strategyActive: ' (当前激活)',
    strategyDefault: ' [默认]',
    noStrategyHint: '暂无策略，请先在策略工作室创建策略',
    strategyDetails: '策略详情',
    activating: '激活中',
    coinSource: '币种来源',
    marginLimit: '保证金上限',
    tradingParams: '交易参数',
    marginMode: '保证金模式',
    crossMargin: '全仓',
    isolatedMargin: '逐仓',
    competitionDisplay: '竞技场显示',
    show: '显示',
    hide: '隐藏',
    hiddenInCompetition: '隐藏后将不在竞技场页面显示此交易员',
    initialBalanceLabel: '初始余额 ($)',
    fetching: '获取中...',
    fetchCurrentBalance: '获取当前余额',
    balanceUpdateHint: '用于手动更新初始余额基准（例如充值/提现后）',
    autoFetchBalanceInfo: '系统将自动获取您的账户净值作为初始余额',
    fetchingBalance: '正在获取余额…',
    editTrader: '保存修改',
    createTraderButton: '创建交易员',

    // AI Model Configuration
    officialAPI: '官方API',
    customAPI: '自定义API',
    apiKey: 'API密钥',
    customAPIURL: '自定义API地址',
    enterAPIKey: '请输入API密钥',
    enterCustomAPIURL: '请输入自定义API端点地址',
    useOfficialAPI: '使用官方API服务',
    useCustomAPI: '使用自定义API端点',

    // Exchange Configuration
    secretKey: '密钥',
    privateKey: '私钥',
    walletAddress: '钱包地址',
    user: '用户名',
    signer: '签名者',
    passphrase: '口令',
    enterSecretKey: '输入密钥',
    enterPrivateKey: '输入私钥',
    enterWalletAddress: '输入钱包地址',
    enterUser: '输入用户名',
    enterSigner: '输入签名者地址',
    enterPassphrase: '输入Passphrase',
    hyperliquidPrivateKeyDesc: 'Hyperliquid 使用私钥进行交易认证',
    hyperliquidWalletAddressDesc: '与私钥对应的钱包地址',
    // Hyperliquid 代理钱包 (新安全模型)
    hyperliquidAgentWalletTitle: 'Hyperliquid 代理钱包配置',
    hyperliquidAgentWalletDesc:
      '使用代理钱包安全交易：代理钱包用于签名（餘額~0），主钱包持有资金（永不暴露私钥）',
    hyperliquidAgentPrivateKey: '代理私钥',
    enterHyperliquidAgentPrivateKey: '输入代理钱包私钥',
    hyperliquidAgentPrivateKeyDesc: '代理钱包仅有交易权限，无法提现',
    hyperliquidMainWalletAddress: '主钱包地址',
    enterHyperliquidMainWalletAddress: '输入主钱包地址',
    hyperliquidMainWalletAddressDesc:
      '持有交易资金的主钱包地址（永不暴露其私钥）',
    // Aster API Pro 配置
    asterApiProTitle: 'Aster API Pro 代理钱包配置',
    asterApiProDesc:
      '使用 API Pro 代理钱包安全交易：代理钱包用于签名交易，主钱包持有资金（永不暴露主钱包私钥）',
    asterUserDesc:
      '主钱包地址 - 您用于登录 Aster 的 EVM 钱包地址（仅支持 EVM 钱包）',
    asterSignerDesc:
      'API Pro 代理钱包地址 (0x...) - 从 https://www.asterdex.com/zh-CN/api-wallet 生成',
    asterPrivateKeyDesc:
      'API Pro 代理钱包私钥 - 从 https://www.asterdex.com/zh-CN/api-wallet 获取（仅在本地用于签名，不会被传输）',
    asterUsdtWarning:
      '重要提示：Aster 仅统计 USDT 余额。请确保您使用 USDT 作为保证金币种，避免其他资产（BNB、ETH等）的价格波动导致盈亏统计错误',
    asterUserLabel: '主钱包地址',
    asterSignerLabel: 'API Pro 代理钱包地址',
    asterPrivateKeyLabel: 'API Pro 代理钱包私钥',
    enterAsterUser: '输入主钱包地址 (0x...)',
    enterAsterSigner: '输入 API Pro 代理钱包地址 (0x...)',
    enterAsterPrivateKey: '输入 API Pro 代理钱包私钥',

    // LIGHTER 配置
    lighterWalletAddress: 'L1 錢包地址',
    lighterPrivateKey: 'L1 私鑰',
    lighterApiKeyPrivateKey: 'API Key 私鑰',
    enterLighterWalletAddress: '請輸入以太坊錢包地址（0x...）',
    enterLighterPrivateKey: '請輸入 L1 私鑰（32 字節）',
    enterLighterApiKeyPrivateKey: '請輸入 API Key 私鑰（40 字節，可選）',
    lighterWalletAddressDesc: '您的以太坊錢包地址，用於識別賬戶',
    lighterPrivateKeyDesc: 'L1 私鑰用於賬戶識別（32 字節 ECDSA 私鑰）',
    lighterApiKeyPrivateKeyDesc:
      'API Key 私鑰用於簽名交易（40 字節 Poseidon2 私鑰）',
    lighterApiKeyOptionalNote:
      '如果不提供 API Key，系統將使用功能受限的 V1 模式',
    lighterV1Description: '基本模式 - 功能受限，僅用於測試框架',
    lighterV2Description: '完整模式 - 支持 Poseidon2 簽名和真實交易',
    lighterPrivateKeyImported: 'LIGHTER 私鑰已導入',

    // Exchange names
    hyperliquidExchangeName: 'Hyperliquid',
    asterExchangeName: 'Aster DEX',

    // Secure input
    secureInputButton: '安全输入',
    secureInputReenter: '重新安全输入',
    secureInputClear: '清除',
    secureInputHint:
      '已通过安全双阶段输入设置。若需修改，请点击"重新安全输入"。',

    // Two Stage Key Modal
    twoStageModalTitle: '安全私钥输入',
    twoStageModalDescription: '使用双阶段流程安全输入长度为 {length} 的私钥。',
    twoStageStage1Title: '步骤一 · 输入前半段',
    twoStageStage1Placeholder: '前 32 位字符（若有 0x 前缀请保留）',
    twoStageStage1Hint:
      '继续后会将扰动字符串复制到剪贴板，用于迷惑剪贴板监控。',
    twoStageStage1Error: '请先输入第一段私钥。',
    twoStageNext: '下一步',
    twoStageProcessing: '处理中…',
    twoStageCancel: '取消',
    twoStageStage2Title: '步骤二 · 输入剩余部分',
    twoStageStage2Placeholder: '剩余的私钥字符',
    twoStageStage2Hint: '将扰动字符串粘贴到任意位置后，再完成私钥输入。',
    twoStageClipboardSuccess:
      '扰动字符串已复制。请在完成前在任意文本处粘贴一次以迷惑剪贴板记录。',
    twoStageClipboardReminder:
      '记得在提交前粘贴一次扰动字符串，降低剪贴板泄漏风险。',
    twoStageClipboardManual: '自动复制失败，请手动复制下面的扰动字符串。',
    twoStageBack: '返回',
    twoStageSubmit: '确认',
    twoStageInvalidFormat:
      '私钥格式不正确，应为 {length} 位十六进制字符（可选 0x 前缀）。',
    testnetDescription: '启用后将连接到交易所测试环境,用于模拟交易',
    securityWarning: '安全提示',
    saveConfiguration: '保存配置',

    // Trader Configuration
    positionMode: '仓位模式',
    crossMarginMode: '全仓模式',
    isolatedMarginMode: '逐仓模式',
    crossMarginDescription: '全仓模式：所有仓位共享账户余额作为保证金',
    isolatedMarginDescription: '逐仓模式：每个仓位独立管理保证金，风险隔离',
    leverageConfiguration: '杠杆配置',
    btcEthLeverage: 'BTC/ETH杠杆',
    altcoinLeverage: '山寨币杠杆',
    leverageRecommendation: '推荐：BTC/ETH 5-10倍，山寨币 3-5倍，控制风险',
    tradingSymbols: '交易币种',
    tradingSymbolsPlaceholder:
      '输入币种，逗号分隔（如：BTCUSDT,ETHUSDT,SOLUSDT）',
    selectSymbols: '选择币种',
    selectTradingSymbols: '选择交易币种',
    selectedSymbolsCount: '已选择 {count} 个币种',
    clearSelection: '清空选择',
    confirmSelection: '确认选择',
    tradingSymbolsDescription:
      '留空 = 使用默认币种。支持 USDT 合约（如：BTCUSDT, ETHUSDT）或 Hyperliquid XYZ USDC 标的（如：TSLA-USDC）',
    btcEthLeverageValidation: 'BTC/ETH杠杆必须在1-50倍之间',
    altcoinLeverageValidation: '山寨币杠杆必须在1-20倍之间',
    invalidSymbolFormat:
      '无效的币种格式：{symbol}，请使用 USDT 合约或 SYMBOL-USDC',

    // System Prompt Templates
    systemPromptTemplate: '系统提示词模板',
    promptTemplateDefault: '默认稳健',
    promptTemplateAdaptive: '保守策略',
    promptTemplateAdaptiveRelaxed: '激进策略',
    promptTemplateHansen: 'Hansen 策略',
    promptTemplateNof1: 'NoF1 英文框架',
    promptTemplateTaroLong: 'Taro 长仓',
    promptDescDefault: '📊 默认稳健策略',
    promptDescDefaultContent:
      '最大化夏普比率，平衡风险收益，适合新手和长期稳定交易',
    promptDescAdaptive: '🛡️ 保守策略 (v6.0.0)',
    promptDescAdaptiveContent:
      '严格风控，BTC 强制确认，高胜率优先，适合保守型交易者',
    promptDescAdaptiveRelaxed: '⚡ 激进策略 (v6.0.0)',
    promptDescAdaptiveRelaxedContent:
      '高频交易，BTC 可选确认，追求交易机会，适合波动市场',
    promptDescHansen: '🎯 Hansen 策略',
    promptDescHansenContent: 'Hansen 定制策略，最大化夏普比率，专业交易者专用',
    promptDescNof1: '🌐 NoF1 英文框架',
    promptDescNof1Content:
      'Hyperliquid 交易所专用，英文提示词，风险调整回报最大化',
    promptDescTaroLong: '📈 Taro 长仓策略',
    promptDescTaroLongContent:
      '数据驱动决策，多维度验证，持续学习进化，长仓专用',

    // Loading & Error
    loading: '加载中...',

    // AI Traders Page - Additional
    inUse: '正在使用',
    noModelsConfigured: '暂无已配置的AI模型',
    noExchangesConfigured: '暂无已配置的交易所',
    signalSource: '信号源',
    signalSourceConfig: '信号源配置',
    ai500Description: '用于获取 AI500 数据源的 API 地址，留空则不使用此数据源',
    oiTopDescription: '用于获取持仓量排行数据的API地址，留空则不使用此信号源',
    information: '说明',
    signalSourceInfo1:
      '• 信号源配置为用户级别，每个用户可以设置自己的信号源URL',
    signalSourceInfo2: '• 在创建交易员时可以选择是否使用这些信号源',
    signalSourceInfo3: '• 配置的URL将用于获取市场数据和交易信号',
    editAIModel: '编辑AI模型',
    addAIModel: '添加AI模型',
    confirmDeleteModel: '确定要删除此AI模型配置吗？',
    cannotDeleteModelInUse: '无法删除此AI模型，因为有交易员正在使用',
    tradersUsing: '正在使用此配置的交易员',
    pleaseDeleteTradersFirst: '请先删除或重新配置这些交易员',
    selectModel: '选择AI模型',
    pleaseSelectModel: '请选择模型',
    customBaseURL: 'Base URL (可选)',
    customBaseURLPlaceholder: '自定义API基础URL，如: https://api.openai.com/v1',
    leaveBlankForDefault: '留空则使用默认API地址',
    modelConfigInfo1: '• 使用官方 API 时，只需填写 API Key，其他字段留空即可',
    modelConfigInfo2:
      '• 自定义 Base URL 和 Model Name 仅在使用第三方代理时需要填写',
    modelConfigInfo3: '• API Key 加密存储，不会明文展示',
    defaultModel: '默认模型',
    applyApiKey: '申请 API Key',
    kimiApiNote:
      'Kimi 需要从国际站申请 API Key (moonshot.ai)，中国区 Key 不通用',
    leaveBlankForDefaultModel: '留空使用默认模型名称',
    customModelName: 'Model Name (可选)',
    customModelNamePlaceholder: '例如: deepseek-chat, qwen3-max, gpt-4o',
    saveConfig: '保存配置',
    editExchange: '编辑交易所',
    addExchange: '添加交易所',
    confirmDeleteExchange: '确定要删除此交易所配置吗？',
    cannotDeleteExchangeInUse: '无法删除此交易所，因为有交易员正在使用',
    pleaseSelectExchange: '请选择交易所',
    exchangeConfigWarning1: '• API密钥将被加密存储，建议使用只读或期货交易权限',
    exchangeConfigWarning2: '• 不要授予提现权限，确保资金安全',
    exchangeConfigWarning3: '• 删除配置后，相关交易员将无法正常交易',
    edit: '编辑',
    viewGuide: '查看教程',
    binanceSetupGuide: '币安配置教程',
    closeGuide: '关闭',
    whitelistIP: '白名单IP',
    whitelistIPDesc: '币安交易所需要填写白名单IP',
    serverIPAddresses: '服务器IP地址',
    copyIP: '复制',
    ipCopied: 'IP已复制',
    copyIPFailed: 'IP地址复制失败，请手动复制',
    loadingServerIP: '正在加载服务器IP...',

    // Error Messages
    createTraderFailed: '创建交易员失败',
    getTraderConfigFailed: '获取交易员配置失败',
    modelConfigNotExist: 'AI模型配置不存在或未启用',
    exchangeConfigNotExist: '交易所配置不存在或未启用',
    updateTraderFailed: '更新交易员失败',
    deleteTraderFailed: '删除交易员失败',
    operationFailed: '操作失败',
    deleteConfigFailed: '删除配置失败',
    modelNotExist: '模型不存在',
    saveConfigFailed: '保存配置失败',
    exchangeNotExist: '交易所不存在',
    deleteExchangeConfigFailed: '删除交易所配置失败',
    saveSignalSourceFailed: '保存信号源配置失败',
    encryptionFailed: '加密敏感数据失败',

    // Login & Register
    login: '登录',
    register: '注册',
    username: '用户名',
    email: '邮箱',
    password: '密码',
    confirmPassword: '确认密码',
    usernamePlaceholder: '请输入用户名',
    emailPlaceholder: '请输入邮箱地址',
    passwordPlaceholder: '请输入密码（至少6位）',
    confirmPasswordPlaceholder: '请再次输入密码',
    passwordRequirements: '密码要求',
    passwordRuleMinLength: '至少 8 位',
    passwordRuleUppercase: '至少 1 个大写字母',
    passwordRuleLowercase: '至少 1 个小写字母',
    passwordRuleNumber: '至少 1 个数字',
    passwordRuleSpecial: '至少 1 个特殊字符（@#$%!&*?）',
    passwordRuleMatch: '两次密码一致',
    passwordNotMeetRequirements: '密码不符合安全要求',
    loginTitle: '登录到您的账户',
    registerTitle: '创建新账户',
    loginButton: '登录',
    registerButton: '注册',
    back: '返回',
    noAccount: '还没有账户？',
    hasAccount: '已有账户？',
    registerNow: '立即注册',
    loginNow: '立即登录',
    forgotPassword: '忘记密码？',
    forgotAccount: '忘记账户？',
    forgotAccountConfirm:
      '⚠️ 这将永久删除全部数据：用户、Trader、策略、AI 模型 API Key、交易所 API Key，以及您的 CLAW402 钱包。请务必在继续前导出需要保留的内容（尤其是钱包私钥）。重新注册不会恢复任何数据。确定要继续吗？',
    forgotAccountSuccess: '账户已重置！现在可以注册新账户了。',
    rememberMe: '记住我',
    resetPassword: '重置密码',
    resetPasswordTitle: '重置您的密码',
    newPassword: '新密码',
    newPasswordPlaceholder: '请输入新密码（至少6位）',
    resetPasswordButton: '重置密码',
    resetPasswordSuccess: '密码重置成功！请使用新密码登录',
    resetPasswordFailed: '密码重置失败',
    backToLogin: '返回登录',
    resetPasswordCliIntro:
      '出于安全考虑，密码找回不再通过浏览器进行。请在部署 NOFX 的服务器上运行以下命令：',
    resetPasswordCliSecurityNote:
      '该操作需要服务器的 shell 访问权限，因此即使 NOFX 暴露在公网上，你的账户依然安全。',
    resetAccountCliIntro:
      '如需清空所有数据并重新开始，请在部署 NOFX 的服务器上运行以下命令：',
    copy: '复制',
    loginSuccess: '登录成功',
    registrationSuccess: '注册成功',
    loginFailed: '登录失败，请检查您的邮箱和密码。',
    registrationFailed: '注册失败，请重试。',
    sessionExpired: '登录已过期，请重新登录',
    invalidCredentials: '邮箱或密码错误',
    weak: '弱',
    medium: '中',
    strong: '强',
    passwordStrength: '密码强度',
    passwordStrengthHint: '建议至少8位，包含大小写、数字和符号',
    passwordMismatch: '两次输入的密码不一致',
    emailRequired: '请输入邮箱',
    passwordRequired: '请输入密码',
    invalidEmail: '邮箱格式不正确',
    passwordTooShort: '密码至少需要6个字符',

    // Landing Page
    features: '功能',
    howItWorks: '如何运作',
    community: '社区',
    language: '语言',
    loggedInAs: '已登录为',
    exitLogin: '退出登录',
    signIn: '登录',
    signUp: '注册',
    registrationClosed: '注册已关闭',
    registrationClosedMessage:
      '平台当前不开放新用户注册，如需访问请联系管理员获取账号。',

    // Hero Section
    githubStarsInDays: '3 天内 2.5K+ GitHub Stars',
    heroTitle1: 'Read the Market.',
    heroTitle2: 'Write the Trade.',
    heroDescription:
      'NOFX 是 AI 交易的未来标准——一个开放、社区驱动的代理式交易操作系统。支持 Binance、Aster DEX 等交易所，自托管、多代理竞争，让 AI 为你自动决策、执行和优化交易。',
    poweredBy: '由 Aster DEX 和 Binance 提供支持。',

    // Landing Page CTA
    readyToDefine: '准备好定义 AI 交易的未来吗？',
    startWithCrypto:
      '从加密市场起步，扩展到 TradFi。NOFX 是 AgentFi 的基础架构。',
    getStartedNow: '立即开始',
    viewSourceCode: '查看源码',

    // Features Section
    coreFeatures: '核心功能',
    whyChooseNofx: '为什么选择 NOFX？',
    openCommunityDriven: '开源、透明、社区驱动的 AI 交易操作系统',
    openSourceSelfHosted: '100% 开源与自托管',
    openSourceDesc: '你的框架，你的规则。非黑箱，支持自定义提示词和多模型。',
    openSourceFeatures1: '完全开源代码',
    openSourceFeatures2: '支持自托管部署',
    openSourceFeatures3: '自定义 AI 提示词',
    openSourceFeatures4: '多模型支持（DeepSeek、Qwen）',
    multiAgentCompetition: '多代理智能竞争',
    multiAgentDesc: 'AI 策略在沙盒中高速战斗，最优者生存，实现策略进化。',
    multiAgentFeatures1: '多 AI 代理并行运行',
    multiAgentFeatures2: '策略自动优化',
    multiAgentFeatures3: '沙盒安全测试',
    multiAgentFeatures4: '跨市场策略移植',
    secureReliableTrading: '安全可靠交易',
    secureDesc: '企业级安全保障，完全掌控你的资金和交易策略。',
    secureFeatures1: '本地私钥管理',
    secureFeatures2: 'API 权限精细控制',
    secureFeatures3: '实时风险监控',
    secureFeatures4: '交易日志审计',

    // About Section
    aboutNofx: '关于 NOFX',
    whatIsNofx: '什么是 NOFX？',
    nofxNotAnotherBot: "NOFX 不是另一个交易机器人，而是 AI 交易的 'Linux' ——",
    nofxDescription1: "一个透明、可信任的开源 OS，提供统一的 '决策-风险-执行'",
    nofxDescription2: '层，支持所有资产类别。',
    nofxDescription3:
      '从加密市场起步（24/7、高波动性完美测试场），未来扩展到股票、期货、外汇。核心：开放架构、AI',
    nofxDescription4:
      '达尔文主义（多代理自竞争、策略进化）、CodeFi 飞轮（开发者 PR',
    nofxDescription5: '贡献获积分奖励）。',
    youFullControl: '你 100% 掌控',
    fullControlDesc: '完全掌控 AI 提示词和资金',
    startupMessages1: '启动自动交易系统...',
    startupMessages2: 'API服务器启动在端口 8080',
    startupMessages3: 'Web 控制台 http://127.0.0.1:3000',

    // How It Works Section
    howToStart: '如何开始使用 NOFX',
    fourSimpleSteps: '四个简单步骤，开启 AI 自动交易之旅',
    step1Title: '拉取 GitHub 仓库',
    step1Desc:
      'git clone https://github.com/NoFxAiOS/nofx 并切换到 dev 分支测试新功能。',
    step2Title: '配置环境',
    step2Desc:
      '前端设置交易所 API（如 Binance、Hyperliquid）、AI 模型和自定义提示词。',
    step3Title: '部署与运行',
    step3Desc:
      '一键 Docker 部署，启动 AI 代理。注意：高风险市场，仅用闲钱测试。',
    step4Title: '优化与贡献',
    step4Desc: '监控交易，提交 PR 改进框架。加入 Telegram 分享策略。',
    importantRiskWarning: '重要风险提示',
    riskWarningText:
      'dev 分支不稳定，勿用无法承受损失的资金。NOFX 非托管，无官方策略。交易有风险，投资需谨慎。',

    // Community Section (testimonials are kept as-is since they are quotes)

    // Footer Section
    futureStandardAI: 'AI 交易的未来标准',
    links: '链接',
    resources: '资源',
    documentation: '文档',
    supporters: '支持方',
    strategicInvestment: '(战略投资)',

    // Login Modal
    accessNofxPlatform: '访问 NOFX 平台',
    loginRegisterPrompt: '请选择登录或注册以访问完整的 AI 交易平台',
    registerNewAccount: '注册新账号',

    // Candidate Coins Warnings
    candidateCoins: '候选币种',
    candidateCoinsZeroWarning: '候选币种数量为 0',
    possibleReasons: '可能原因：',
    ai500ApiNotConfigured:
      'AI500 数据源 API 未配置或无法访问（请检查信号源设置）',
    apiConnectionTimeout: 'API连接超时或返回数据为空',
    noCustomCoinsAndApiFailed: '未配置自定义币种且API获取失败',
    solutions: '解决方案：',
    setCustomCoinsInConfig: '在交易员配置中设置自定义币种列表',
    orConfigureCorrectApiUrl: '或者配置正确的数据源 API 地址',
    orDisableAI500Options: '或者禁用"使用 AI500 数据源"和"使用 OI Top"选项',
    signalSourceNotConfigured: '信号源未配置',
    signalSourceWarningMessage:
      '您有交易员启用了"使用 AI500 数据源"或"使用 OI Top"，但尚未配置信号源 API 地址。这将导致候选币种数量为 0，交易员无法正常工作。',
    configureSignalSourceNow: '立即配置信号源',

    // FAQ Page

    // FAQ Categories

    // ===== 入门指南 =====






    // ===== 安装部署 =====






    // ===== 配置设置 =====






    // ===== 交易相关 =====








    // ===== 技术问题 =====








    // ===== 安全相关 =====




    // ===== 功能介绍 =====



    // ===== AI 模型 =====




    // ===== 参与贡献 =====




    // Web Crypto Environment Check
    environmentCheck: {
      button: '一键检测环境',
      checking: '正在检测...',
      description: '系统将自动检测当前浏览器是否允许使用 Web Crypto。',
      secureTitle: '环境安全，已启用 Web Crypto',
      secureDesc: '页面处于安全上下文，可继续输入敏感信息并使用加密传输。',
      insecureTitle: '检测到非安全环境',
      insecureDesc:
        '当前访问未通过 HTTPS 或可信 localhost，浏览器会阻止 Web Crypto 调用。',
      tipsTitle: '修改建议：',
      tipHTTPS:
        '通过 HTTPS 访问（即使是 IP 也需证书），或部署到支持 TLS 的域名。',
      tipLocalhost: '开发阶段请使用 http://localhost 或 127.0.0.1。',
      tipIframe:
        '避免把应用嵌入在不安全的 HTTP iframe 或会降级协议的反向代理中。',
      unsupportedTitle: '浏览器未提供 Web Crypto',
      unsupportedDesc:
        '请通过 HTTPS 或本机 localhost 访问 NOFX，并避免嵌入不安全 iframe/反向代理，以符合浏览器的 Web Crypto 规则。',
      summary: '当前来源：{origin} · 协议：{protocol}',
      disabledTitle: '传输加密已禁用',
      disabledDesc:
        '服务端传输加密已关闭，API 密钥将以明文传输。如需增强安全性，请设置 TRANSPORT_ENCRYPTION=true。',
    },

    environmentSteps: {
      checkTitle: '1. 环境检测',
      selectTitle: '2. 选择交易所',
    },

    // Two-Stage Key Modal
    twoStageKey: {
      title: '两阶段私钥输入',
      stage1Description: '请输入私钥的前 {length} 位字符',
      stage2Description: '请输入私钥的后 {length} 位字符',
      stage1InputLabel: '第一部分',
      stage2InputLabel: '第二部分',
      characters: '位字符',
      processing: '处理中...',
      nextButton: '下一步',
      cancelButton: '取消',
      backButton: '返回',
      encryptButton: '加密并提交',
      obfuscationCopied: '混淆数据已复制到剪贴板',
      obfuscationInstruction: '请粘贴其他内容清空剪贴板，然后继续',
      obfuscationManual: '需要手动混淆',
    },

    // Error Messages
    errors: {
      privatekeyIncomplete: '请输入至少 {expected} 位字符',
      privatekeyInvalidFormat: '私钥格式无效（应为64位十六进制字符）',
      privatekeyObfuscationFailed: '剪贴板混淆失败',
    },

    // Position History
    positionHistory: {
      title: '历史仓位',
      loading: '加载历史仓位...',
      noHistory: '暂无历史仓位',
      noHistoryDesc: '平仓后的仓位记录将显示在此处',
      showingPositions: '显示 {count} / {total} 条记录',
      totalPnL: '总盈亏',
      // Stats
      totalTrades: '总交易次数',
      winLoss: '盈利: {win} / 亏损: {loss}',
      winRate: '胜率',
      profitFactor: '盈利因子',
      profitFactorDesc: '总盈利 / 总亏损',
      plRatio: '盈亏比',
      plRatioDesc: '平均盈利 / 平均亏损',
      sharpeRatio: '夏普比率',
      sharpeRatioDesc: '风险调整收益',
      maxDrawdown: '最大回撤',
      avgWin: '平均盈利',
      avgLoss: '平均亏损',
      netPnL: '净盈亏',
      netPnLDesc: '扣除手续费后',
      fee: '手续费',
      // Direction Stats
      trades: '交易次数',
      avgPnL: '平均盈亏',
      // Symbol Performance
      symbolPerformance: '品种表现',
      // Filters
      symbol: '交易对',
      allSymbols: '全部交易对',
      side: '方向',
      all: '全部',
      sort: '排序',
      latestFirst: '最新优先',
      oldestFirst: '最早优先',
      highestPnL: '盈利最高',
      lowestPnL: '亏损最多',
      // Table Headers
      entry: '开仓价',
      exit: '平仓价',
      qty: '数量',
      value: '仓位价值',
      lev: '杠杆',
      pnl: '盈亏',
      duration: '持仓时长',
      closedAt: '平仓时间',
    },

    // Data Page
    dataCenter: '数据中心',

    // Strategy Market Page
    strategyMarket: {
      title: '策略市场',
      subtitle: 'STRATEGY MARKETPLACE',
      description: '发现、学习并复用社区精英交易员的策略配置',
      search: '搜索参数...',
      all: '全部协议',
      popular: '热门配置',
      recent: '最新提交',
      myStrategies: '我的库',
      noStrategies: '无信号',
      noStrategiesDesc: '当前频段未检测到策略信号',
      author: 'OPERATOR',
      createdAt: 'TIMESTAMP',
      viewConfig: 'DECRYPT CONFIG',
      hideConfig: 'ENCRYPT',
      copyConfig: 'CLONE CONFIG',
      copied: 'COPIED',
      configHidden: 'ENCRYPTED',
      configHiddenDesc: '配置参数已加密',
      indicators: 'INDICATORS',
      maxPositions: 'POS_LIMIT',
      maxLeverage: 'LEV_MAX',
      shareYours: 'UPLOAD_STRATEGY',
      makePublic: 'PUBLISH',
      loading: 'INITIALIZING...',
    },

    // Strategy Studio Page
    strategyStudio: {
      title: '策略工作室',
      subtitle: '可视化配置和测试交易策略',
      strategies: '策略',
      newStrategy: '新建',
      strategyType: '策略类型',
      aiTrading: 'AI 智能交易',
      aiTradingDesc: 'AI 分析市场并自主决策买卖',
      gridTrading: 'AI 网格交易',
      gridTradingDesc: 'AI 控制网格策略，在震荡市场获利',
      gridConfig: '网格配置',
      coinSource: '币种来源',
      indicators: '技术指标',
      riskControl: '风控参数',
      promptSections: 'Prompt 编辑',
      customPrompt: '附加提示',
      save: '保存',
      saving: '保存中...',
      activate: '激活',
      active: '激活中',
      default: '默认',
      promptPreview: 'Prompt 预览',
      aiTestRun: 'AI 测试',
      systemPrompt: 'System Prompt',
      userPrompt: 'User Prompt',
      loadPrompt: '生成 Prompt',
      refreshPrompt: '刷新',
      promptVariant: '风格',
      balanced: '平衡',
      aggressive: '激进',
      conservative: '保守',
      selectModel: '选择 AI 模型',
      runTest: '运行 AI 测试',
      running: '运行中...',
      aiOutput: 'AI 输出',
      reasoning: '思维链',
      decisions: '决策',
      duration: '耗时',
      noModel: '请先配置 AI 模型',
      testNote: '使用真实 AI 模型测试，不执行交易',
      publishSettings: '发布设置',
      newStrategyName: '新策略',
      strategyCopy: '策略副本',
      strategyDeleted: '策略已删除',
      cannotDeleteActiveStrategy: '激活中的策略不能删除',
      confirmDeleteStrategy: '确定删除此策略？',
      confirmDelete: '确认删除',
      delete: '删除',
      cancel: '取消',
      strategyExported: '策略已导出',
      invalidStrategyFile: '无效的策略文件',
      imported: '导入',
      strategyImported: '策略已导入',
      strategySaved: '策略已保存',
      importStrategy: '导入策略',
      newStrategyTooltip: '新建策略',
      export: '导出',
      duplicate: '复制',
      deleteTooltip: '删除',
      public: '公开',
      addDescription: '添加策略简介...',
      unsaved: '未保存',
      discardChanges: '撤销',
      selectOrCreate: '选择或创建策略',
      customPromptDesc:
        '附加在 System Prompt 末尾的额外提示，用于补充个性化交易风格',
      customPromptPlaceholder: '输入自定义提示词...',
      generatePromptPreview: '点击生成 Prompt 预览',
      runAiTestHint: '点击运行 AI 测试',
      tokenEstimate: 'Token 预估',
      tokenExceedWarning: 'Token 估算超过 128K，部分模型请求可能失败',
      tokenEstimating: '预估中...',
      tokenTooltip: '基于 200K 上下文计算',
    },

    // Metric Tooltip
    metricTooltip: {
      formula: '计算公式',
    },

    // Login Required Overlay
    loginRequired: {
      title: '系统访问受限',
      accessDenied: '访问被拒绝',
      subtitleWithFeature: '访问「{featureName}」需要更高权限',
      subtitleDefault: '此模块需要授权访问',
      description:
        '初始化身份验证协议以解锁完整系统功能：AI 交易员配置、策略市场数据流。',
      benefit1: 'AI 交易员控制权',
      benefit2: '高频策略核心市场',
      benefit4: '全系统数据可视化',
      loginButton: '执行登录指令',
      registerButton: '注册新用户 ID',
      abort: '中止操作',
    },

    // Advanced Chart
    advancedChart: {
      updating: '更新中...',
      indicators: '指标',
      orderMarkers: '订单标记',
      technicalIndicators: '技术指标',
      clickToToggle: '点击选择需要显示的指标',
      shares: '股',
      units: '个',
    },

    // Chart With Orders
    chartWithOrders: {
      failedToLoad: '加载图表数据失败',
      loading: '加载中...',
      buy: 'BUY (买入)',
      sell: 'SELL (卖出)',
    },

    // Comparison Chart
    comparisonChart: {
      '1d': '1天',
      '3d': '3天',
      '7d': '7天',
      '30d': '30天',
      all: '全部',
    },

    traderDashboard: {
      connectionFailed: '无法连接到服务器',
      connectionFailedDesc: '请确认后端服务已启动。',
      retry: '重试',
      confirmClosePosition: '确定要平仓 {symbol} {side} 吗？',
      confirmClose: '确认平仓',
      confirm: '确认',
      cancel: '取消',
      positionClosed: '平仓成功',
      closeFailed: '平仓失败',
      hideAddress: '隐藏地址',
      showFullAddress: '显示完整地址',
      copyAddress: '复制地址',
      noAddressConfigured: '未配置地址',
      action: '操作',
      entry: '入场价',
      mark: '标记价',
      qty: '数量',
      value: '价值',
      lev: '杠杆',
      uPnL: '未实现盈亏',
      liq: '强平价',
      closePosition: '平仓',
      close: '平仓',
      showingPositions: '显示 {shown} / {total} 个持仓',
      perPage: '每页',
      accountFetchFailed: 'DATA_FETCH::FAILED — 账户数据请求失败，请检查连接',
      positionsFetchFailed: '持仓数据请求失败',
      decisionsFetchFailed: '决策记录请求失败',
    },

    aiTradersToast: {
      creating: '正在创建…',
      created: '创建成功',
      createFailed: '创建失败',
      saving: '正在保存…',
      saved: '保存成功',
      saveFailed: '保存失败',
      deleting: '正在删除…',
      deleted: '删除成功',
      deleteFailed: '删除失败',
      stopping: '正在停止…',
      stopped: '已停止',
      stopFailed: '停止失败',
      starting: '正在启动…',
      started: '已启动',
      startFailed: '启动失败',
      updating: '正在更新…',
      updatingConfig: '正在更新配置…',
      configUpdated: '配置已更新',
      configUpdateFailed: '更新配置失败',
      showInCompetition: '已在竞技场显示',
      hideInCompetition: '已在竞技场隐藏',
      updateFailed: '更新失败',
      updatingModelConfig: '正在更新模型配置…',
      modelConfigUpdated: '模型配置已更新',
      modelConfigUpdateFailed: '更新模型配置失败',
      deletingExchange: '正在删除交易所账户…',
      exchangeDeleted: '交易所账户已删除',
      exchangeDeleteFailed: '删除交易所账户失败',
      updatingExchangeConfig: '正在更新交易所配置…',
      exchangeConfigUpdated: '交易所配置已更新',
      exchangeConfigUpdateFailed: '更新交易所配置失败',
      creatingExchange: '正在创建交易所账户…',
      exchangeCreated: '交易所账户已创建',
      exchangeCreateFailed: '创建交易所账户失败',
    },

    modelConfig: {
      selectModel: '选择模型',
      configure: '配置',
      configureApi: '配置 API',
      configureWallet: '配置钱包',
      chooseProvider: '选择 AI 模型提供商',
      claw402EntryDesc:
        '默认推荐走这条路。直接用 Base USDC 按次付费，不需要自己管理 API Key。',
      otherApiEntry: '其他 API 模型',
      otherApiEntryDesc:
        '如果你已经有自己的 OpenAI、Claude、Gemini、DeepSeek 等 API Key，再从这里进入。',
      payPerCall: 'USDC 按次付费 · 支持全部 AI 模型 · 无需 API Key',
      recommended: '推荐',
      allModelsClaw: '用 USDC 按次付费，支持所有主流 AI 模型',
      selectAiModel: '① 选择 AI 模型',
      allModelsUnified: '所有模型通过 Claw402 统一调用，创建后可随时切换',
      setupWallet: '② 设置钱包',
      walletInfo: '💡 Claw402 使用 Base 链上的 USDC 付费，你需要一个 EVM 钱包',
      exportKey: '可以用 MetaMask、Rabby 等钱包导出私钥',
      dedicatedWallet: '建议新建一个专用钱包，充入少量 USDC 即可',
      walletPrivateKey: '钱包私钥（Base 链 EVM）',
      privateKeyNote:
        '私钥仅在本地签名使用，不会上传或发送交易。无需 ETH，无 Gas 费用。',
      howToFundUsdc: '如何充值 USDC',
      fundStep1: '从交易所（Binance / OKX / Coinbase）提 USDC 到你的钱包地址',
      fundStep2: '选择 Base 网络（手续费极低）',
      fundStep3: '充入 $5-10 USDC 即可使用很长时间（约 $0.003/次调用）',
      back: '返回',
      startTrading: '开始交易',
      modelsConfigured: '带金色标记的模型已配置',
      getStarted: '开始使用',
      getApiKey: '获取 API Key',
      walletPrivateKeyLabel: '钱包私钥 *',
      selectModelLabel: '选择模型',
      validating: '验证中...',
      walletAddress: '钱包地址',
      usdcBalance: 'Base USDC 余额',
      claw402Connected: 'claw402 已连接',
      claw402Unreachable: 'claw402 不可达',
      depositUsdc: '请往此地址充值 Base 链 USDC',
      invalidKeyPrefix: '请在开头加 0x',
      invalidKeyLength: '应为 66 个字符，当前',
      invalidKeyChars: '包含非法字符',
      testConnection: '测试连接',
      testingConnection: '测试中...',
    },

    exchangeConfig: {
      selectExchange: '选择交易所',
      configure: '配置账户',
      chooseExchange: '选择您的交易所',
      centralizedExchanges: '中心化交易所 (CEX)',
      decentralizedExchanges: '去中心化交易所 (DEX)',
      register: '注册',
      bonus: '优惠',
      accountName: '账户名称',
      accountNamePlaceholder: '例如：主账户、套利账户',
      pleaseEnterAccountName: '请输入账户名称',
      useBinanceFuturesApi: '币安用户必读：使用「现货与合约交易」API',
      viewTutorial: '查看官方教程',
      lighterApiKeySetup: 'Lighter API Key 配置',
      lighterApiKeyDesc: '请在 Lighter 网站生成 API Key',
      apiKeyIndex: 'API Key 索引',
      apiKeyIndexTooltip: 'API Key 索引从0开始',
      back: '返回',
    },

    telegram: {
      botSetup: 'Telegram Bot 配置',
      createBot: '创建 Bot',
      bindAccount: '绑定账号',
      done: '完成',
      invalidTokenFormat: 'Bot Token 格式不正确，应为 "数字:字母数字串"',
      tokenSaved: 'Bot Token 已保存，等待绑定',
      saveFailed: '保存失败，请检查 Token 是否正确',
      unbound: '已解绑 Telegram 账号',
      unbindFailed: '解绑失败',
      step1Title: '第一步：在 Telegram 创建你的 Bot',
      step1Desc1: '打开 Telegram，搜索',
      step1Desc2: '发送',
      step1Desc2Suffix: '命令',
      step1Desc3: '按提示输入 Bot 名称和用户名',
      step1Desc4: 'BotFather 会返回一个 Token，复制它',
      openBotFather: '打开 @BotFather',
      pasteToken: '粘贴 Bot Token',
      tokenFormat: 'Token 格式：数字:字母数字串，如 123456789:ABCdef...',
      selectAiModel: '选择 AI 模型（可选）',
      noEnabledModels: '暂无启用的模型，请先在「AI 模型」中配置',
      autoSelect: '— 自动选择（推荐）',
      autoUseEnabled: '不选则自动使用已启用的模型',
      savingToken: '保存中...',
      saveAndContinue: '保存并继续',
      step2Title: '第二步：向你的 Bot 发送 /start',
      step2Desc1: '在 Telegram 中搜索你刚创建的 Bot',
      step2Desc2: '点击 Start 或发送',
      step2Desc3: 'Bot 会自动绑定到你的账号',
      currentToken: '当前 Token',
      waitingForStart: '⏳ 等待你发送 /start... 发送后刷新页面查看状态',
      reconfigureToken: '重新配置 Token',
      bindSuccess: '绑定成功！',
      noStartReceived: '尚未收到 /start，请先向 Bot 发送 /start',
      checkFailed: '检查失败',
      checkStatus: '检查绑定状态',
      botActive: 'Telegram Bot 已绑定！',
      botActiveDesc: '你现在可以通过 Telegram 用自然语言控制交易系统',
      supportedCommands: '支持的命令',
      cmdHelp: '查看所有命令',
      cmdStatus: '查看交易员状态',
      cmdNaturalLang: '自然语言查询',
      cmdStartStop: '启动/停止交易员',
      cmdControl: '自然语言控制',
      cmdPositions: '查看持仓',
      cmdPositionsDesc: '实时持仓查询',
      cmdStrategy: '配置策略',
      cmdStrategyDesc: '修改交易策略',
      unbinding: '解绑中...',
      unbindAccount: '解绑账号',
      aiModelLabel: 'AI 模型（用于自然语言解析）',
      aiModelAutoSelect: '— 自动选择',
      modelUpdated: 'AI 模型已更新',
      modelUpdateFailed: '更新失败',
      save: '保存',
      loading: '加载中...',
    },

    traderConfigView: {
      traderConfig: '交易员配置',
      configInfo: '{name} 的配置信息',
      running: '运行中',
      stopped: '已停止',
      basicInfo: '基础信息',
      traderName: '交易员名称',
      aiModel: 'AI模型',
      exchange: '交易所',
      initialBalance: '初始余额',
      marginMode: '保证金模式',
      crossMargin: '全仓',
      isolatedMargin: '逐仓',
      scanInterval: '{minutes} 分钟',
      scanIntervalLabel: '扫描间隔',
      strategyUsed: '使用策略',
      strategyName: '策略名称',
      close: '关闭',
      yes: '是',
      no: '否',
    },
  },
  id: {
    // Header
    appTitle: 'NOFX',
    subtitle: 'Platform Trading Multi-AI',
    aiTraders: 'Trader AI',
    details: 'Detail',
    tradingPanel: 'Panel Trading',
    competition: 'Kompetisi',
    running: 'BERJALAN',
    stopped: 'BERHENTI',
    adminMode: 'Mode Admin',
    logout: 'Keluar',
    switchTrader: 'Ganti Trader:',
    view: 'Lihat',

    // Navigation
    realtimeNav: 'Papan Peringkat',
    configNav: 'Konfigurasi',
    dashboardNav: 'Dasbor',
    strategyNav: 'Strategi',
    faqNav: 'FAQ',

    // Footer
    footerTitle: 'NOFX - Sistem Trading AI',
    footerWarning: '⚠️ Trading memiliki risiko. Gunakan dengan bijak.',

    // Stats Cards
    totalEquity: 'Total Ekuitas',
    availableBalance: 'Saldo Tersedia',
    totalPnL: 'Total L/R',
    positions: 'Posisi',
    margin: 'Margin',
    free: 'Bebas',

    // Positions Table
    currentPositions: 'Posisi Saat Ini',
    active: 'Aktif',
    symbol: 'Simbol',
    side: 'Arah',
    entryPrice: 'Harga Masuk',
    stopLoss: 'Stop Loss',
    takeProfit: 'Take Profit',
    riskReward: 'Risiko/Imbalan',
    markPrice: 'Harga Tanda',
    quantity: 'Jumlah',
    positionValue: 'Nilai Posisi',
    leverage: 'Leverage',
    unrealizedPnL: 'L/R Belum Terealisasi',
    liqPrice: 'Harga Likuidasi',
    long: 'LONG',
    short: 'SHORT',
    noPositions: 'Tidak Ada Posisi',
    noActivePositions: 'Tidak ada posisi trading yang aktif',

    // Recent Decisions
    recentDecisions: 'Keputusan Terbaru',
    lastCycles: '{count} siklus trading terakhir',
    noDecisionsYet: 'Belum Ada Keputusan',
    aiDecisionsWillAppear: 'Keputusan trading AI akan muncul di sini',
    cycle: 'Siklus',
    success: 'Berhasil',
    failed: 'Gagal',
    inputPrompt: 'Prompt Input',
    aiThinking: 'Rantai Pemikiran AI',
    collapse: 'Tutup',
    expand: 'Buka',

    // Equity Chart
    accountEquityCurve: 'Kurva Ekuitas Akun',
    noHistoricalData: 'Tidak Ada Data Historis',
    dataWillAppear:
      'Kurva ekuitas akan muncul setelah beberapa siklus berjalan',
    initialBalance: 'Saldo Awal',
    currentEquity: 'Ekuitas Saat Ini',
    historicalCycles: 'Siklus Historis',
    displayRange: 'Rentang Tampilan',
    recent: 'Terbaru',
    allData: 'Semua Data',
    cycles: 'Siklus',

    // Comparison Chart
    comparisonMode: 'Mode Perbandingan',
    dataPoints: 'Titik Data',
    currentGap: 'Selisih Saat Ini',
    count: '{count} poin',

    // TradingView Chart
    marketChart: 'Grafik Pasar',
    viewChart: 'Klik untuk melihat grafik',
    enterSymbol: 'Masukkan simbol...',
    popularSymbols: 'Simbol Populer',
    fullscreen: 'Layar Penuh',
    exitFullscreen: 'Keluar Layar Penuh',

    // Competition Page
    aiCompetition: 'Kompetisi AI',
    traders: 'trader',
    liveBattle: 'Pertarungan Langsung',
    realTimeBattle: 'Pertarungan Realtime',
    leader: 'Pemimpin',
    leaderboard: 'Papan Peringkat',
    live: 'LIVE',
    realTime: 'LIVE',
    performanceComparison: 'Perbandingan Performa',
    realTimePnL: 'L/R Realtime %',
    realTimePnLPercent: 'L/R Realtime %',
    headToHead: 'Pertarungan Langsung',
    leadingBy: 'Unggul {gap}%',
    behindBy: 'Tertinggal {gap}%',
    equity: 'Ekuitas',
    pnl: 'L/R',
    pos: 'Pos',

    // AI Traders Management
    manageAITraders: 'Kelola bot trading AI Anda',
    aiModels: 'Model AI',
    exchanges: 'Bursa',
    createTrader: 'Buat Trader',
    modelConfiguration: 'Konfigurasi Model',
    configured: 'Terkonfigurasi',
    notConfigured: 'Belum Dikonfigurasi',
    currentTraders: 'Trader Saat Ini',
    noTraders: 'Tidak Ada Trader AI',
    createFirstTrader: 'Buat trader AI pertama Anda untuk memulai',
    dashboardEmptyTitle: 'Mari Mulai!',
    dashboardEmptyDescription:
      'Buat trader AI pertama Anda untuk mengotomatisasi strategi trading. Hubungkan bursa, pilih model AI, dan mulai trading dalam hitungan menit!',
    goToTradersPage: 'Buat Trader Pertama Anda',
    configureModelsFirst: 'Silakan konfigurasi model AI terlebih dahulu',
    configureExchangesFirst: 'Silakan konfigurasi bursa terlebih dahulu',
    configureModelsAndExchangesFirst:
      'Silakan konfigurasi model AI dan bursa terlebih dahulu',
    modelNotConfigured: 'Model yang dipilih belum dikonfigurasi',
    exchangeNotConfigured: 'Bursa yang dipilih belum dikonfigurasi',
    confirmDeleteTrader: 'Apakah Anda yakin ingin menghapus trader ini?',
    status: 'Status',
    start: 'Mulai',
    stop: 'Berhenti',
    createNewTrader: 'Buat Trader AI Baru',
    selectAIModel: 'Pilih Model AI',
    selectExchange: 'Pilih Bursa',
    traderName: 'Nama Trader',
    enterTraderName: 'Masukkan nama trader',
    cancel: 'Batal',
    create: 'Buat',
    configureAIModels: 'Konfigurasi Model AI',
    configureExchanges: 'Konfigurasi Bursa',
    aiScanInterval: 'Interval Keputusan AI (menit)',
    scanIntervalRecommend: 'Disarankan: 15-30 menit',
    useTestnet: 'Gunakan Testnet',
    enabled: 'Aktif',
    save: 'Simpan',

    // TraderConfigModal
    fetchBalanceEditModeOnly:
      'Hanya bisa mengambil saldo saat ini dalam mode edit',
    balanceFetched: 'Saldo saat ini berhasil diambil',
    balanceFetchFailed: 'Gagal mengambil saldo',
    balanceFetchNetworkError: 'Gagal mengambil saldo, periksa koneksi jaringan',
    saving: 'Menyimpan...',
    saveSuccess: 'Berhasil disimpan',
    saveFailed: 'Gagal menyimpan',
    editTraderConfig: 'Edit Konfigurasi Trader',
    selectStrategyAndConfigParams:
      'Pilih Strategi dan Konfigurasi Parameter Dasar',
    basicConfig: 'Konfigurasi Dasar',
    traderNameRequired: 'Nama Trader *',
    enterTraderNamePlaceholder: 'Masukkan nama trader',
    aiModelRequired: 'Model AI *',
    exchangeRequired: 'Bursa *',
    noExchangeAccount: 'Belum punya akun bursa? Klik untuk mendaftar',
    discount: 'Diskon',
    selectTradingStrategy: 'Pilih Strategi Trading',
    useStrategy: 'Gunakan Strategi',
    noStrategyManual: '-- Tanpa Strategi (Konfigurasi Manual) --',
    strategyActive: ' (Aktif)',
    strategyDefault: ' [Default]',
    noStrategyHint:
      'Belum ada strategi, buat di Strategy Studio terlebih dahulu',
    strategyDetails: 'Detail Strategi',
    activating: 'Mengaktifkan',
    coinSource: 'Sumber Koin',
    marginLimit: 'Batas Margin',
    tradingParams: 'Parameter Trading',
    marginMode: 'Mode Margin',
    crossMargin: 'Cross Margin',
    isolatedMargin: 'Isolated Margin',
    competitionDisplay: 'Tampilkan di Kompetisi',
    show: 'Tampilkan',
    hide: 'Sembunyikan',
    hiddenInCompetition:
      'Trader ini tidak akan ditampilkan di halaman kompetisi saat disembunyikan',
    initialBalanceLabel: 'Saldo Awal ($)',
    fetching: 'Mengambil...',
    fetchCurrentBalance: 'Ambil Saldo Saat Ini',
    balanceUpdateHint:
      'Digunakan untuk memperbarui saldo awal secara manual (misal setelah deposit/withdraw)',
    autoFetchBalanceInfo:
      'Sistem akan otomatis mengambil ekuitas akun Anda sebagai saldo awal',
    fetchingBalance: 'Mengambil saldo...',
    editTrader: 'Simpan Perubahan',
    createTraderButton: 'Buat Trader',

    // AI Model Configuration
    officialAPI: 'API Resmi',
    customAPI: 'API Kustom',
    apiKey: 'API Key',
    customAPIURL: 'URL API Kustom',
    enterAPIKey: 'Masukkan API Key',
    enterCustomAPIURL: 'Masukkan URL endpoint API kustom',
    useOfficialAPI: 'Gunakan layanan API resmi',
    useCustomAPI: 'Gunakan endpoint API kustom',

    // Exchange Configuration
    secretKey: 'Secret Key',
    privateKey: 'Private Key',
    walletAddress: 'Alamat Wallet',
    user: 'Pengguna',
    signer: 'Penandatangan',
    passphrase: 'Passphrase',
    enterPrivateKey: 'Masukkan Private Key',
    enterWalletAddress: 'Masukkan Alamat Wallet',
    enterUser: 'Masukkan Pengguna',
    enterSigner: 'Masukkan Alamat Penandatangan',
    enterSecretKey: 'Masukkan Secret Key',
    enterPassphrase: 'Masukkan Passphrase',
    hyperliquidPrivateKeyDesc:
      'Hyperliquid menggunakan private key untuk autentikasi trading',
    hyperliquidWalletAddressDesc:
      'Alamat wallet yang sesuai dengan private key',
    hyperliquidAgentWalletTitle: 'Konfigurasi Agent Wallet Hyperliquid',
    hyperliquidAgentWalletDesc:
      'Gunakan Agent Wallet untuk trading aman: Agent wallet menandatangani transaksi (saldo ~0), Wallet utama menyimpan dana (jangan pernah ekspos private key)',
    hyperliquidAgentPrivateKey: 'Agent Private Key',
    enterHyperliquidAgentPrivateKey: 'Masukkan private key agent wallet',
    hyperliquidAgentPrivateKeyDesc:
      'Private key agent wallet untuk menandatangani transaksi (jaga saldo mendekati 0 untuk keamanan)',
    hyperliquidMainWalletAddress: 'Alamat Wallet Utama',
    enterHyperliquidMainWalletAddress: 'Masukkan alamat wallet utama',
    hyperliquidMainWalletAddressDesc:
      'Alamat wallet utama yang menyimpan dana trading Anda (jangan pernah ekspos private key-nya)',
    asterApiProTitle: 'Konfigurasi Wallet API Pro Aster',
    asterApiProDesc:
      'Gunakan wallet API Pro untuk trading aman: Wallet API menandatangani transaksi, wallet utama menyimpan dana (jangan pernah ekspos private key wallet utama)',
    asterUserDesc:
      'Alamat wallet utama - Alamat wallet EVM yang Anda gunakan untuk login ke Aster (Catatan: Hanya wallet EVM yang didukung)',
    asterSignerDesc:
      'Alamat wallet API Pro (0x...) - Buat dari https://www.asterdex.com/en/api-wallet',
    asterPrivateKeyDesc:
      'Private key wallet API Pro - Dapatkan dari https://www.asterdex.com/en/api-wallet (hanya digunakan lokal untuk penandatanganan, tidak pernah ditransmisikan)',
    asterUsdtWarning:
      'Penting: Aster hanya melacak saldo USDT. Pastikan Anda menggunakan USDT sebagai mata uang margin untuk menghindari kesalahan perhitungan L/R akibat fluktuasi harga aset lain (BNB, ETH, dll.)',
    asterUserLabel: 'Alamat Wallet Utama',
    asterSignerLabel: 'Alamat Wallet API Pro',
    asterPrivateKeyLabel: 'Private Key Wallet API Pro',
    enterAsterUser: 'Masukkan alamat wallet utama (0x...)',
    enterAsterSigner: 'Masukkan alamat wallet API Pro (0x...)',
    enterAsterPrivateKey: 'Masukkan private key wallet API Pro',
    lighterWalletAddress: 'Alamat Wallet L1',
    lighterPrivateKey: 'Private Key L1',
    lighterApiKeyPrivateKey: 'Private Key API Key',
    enterLighterWalletAddress: 'Masukkan alamat wallet Ethereum (0x...)',
    enterLighterPrivateKey: 'Masukkan private key L1 (32 byte)',
    enterLighterApiKeyPrivateKey:
      'Masukkan private key API Key (40 byte, opsional)',
    lighterWalletAddressDesc:
      'Alamat wallet Ethereum Anda untuk identifikasi akun',
    lighterPrivateKeyDesc:
      'Private key L1 untuk identifikasi akun (kunci ECDSA 32 byte)',
    lighterApiKeyPrivateKeyDesc:
      'Private key API Key untuk penandatanganan transaksi (kunci Poseidon2 40 byte)',
    lighterApiKeyOptionalNote:
      'Tanpa API Key, sistem akan menggunakan mode V1 terbatas',
    lighterV1Description:
      'Mode Dasar - Fungsionalitas terbatas, hanya framework pengujian',
    lighterV2Description:
      'Mode Lengkap - Mendukung penandatanganan Poseidon2 dan trading nyata',
    lighterPrivateKeyImported: 'Private key LIGHTER telah diimpor',
    hyperliquidExchangeName: 'Hyperliquid',
    asterExchangeName: 'Aster DEX',
    secureInputButton: 'Input Aman',
    secureInputReenter: 'Masukkan Ulang dengan Aman',
    secureInputClear: 'Hapus',
    secureInputHint:
      'Diambil melalui input aman dua tahap. Gunakan "Masukkan Ulang dengan Aman" untuk memperbarui nilai ini.',
    twoStageModalTitle: 'Input Kunci Aman',
    twoStageModalDescription:
      'Gunakan alur dua tahap untuk memasukkan private key {length} karakter Anda dengan aman.',
    twoStageStage1Title: 'Tahap 1 · Masukkan bagian pertama',
    twoStageStage1Placeholder: '32 karakter pertama (sertakan 0x jika ada)',
    twoStageStage1Hint:
      'Melanjutkan akan menyalin string pengacak ke clipboard sebagai pengalih.',
    twoStageStage1Error: 'Silakan masukkan bagian pertama terlebih dahulu.',
    twoStageNext: 'Lanjut',
    twoStageProcessing: 'Memproses…',
    twoStageCancel: 'Batal',
    twoStageStage2Title: 'Tahap 2 · Masukkan sisanya',
    twoStageStage2Placeholder: 'Karakter sisa dari private key Anda',
    twoStageStage2Hint:
      'Tempelkan string pengacak di tempat netral, lalu selesaikan memasukkan kunci Anda.',
    twoStageClipboardSuccess:
      'String pengacak disalin. Tempelkan di kolom teks mana pun sebelum menyelesaikan.',
    twoStageClipboardReminder:
      'Ingat tempelkan string pengacak sebelum mengirim untuk menghindari kebocoran clipboard.',
    twoStageClipboardManual:
      'Salin otomatis gagal. Salin string pengacak di bawah secara manual.',
    twoStageBack: 'Kembali',
    twoStageSubmit: 'Konfirmasi',
    twoStageInvalidFormat:
      'Format private key tidak valid. Diharapkan {length} karakter heksadesimal (awalan 0x opsional).',
    testnetDescription:
      'Aktifkan untuk terhubung ke lingkungan uji coba bursa untuk trading simulasi',
    securityWarning: 'Peringatan Keamanan',
    saveConfiguration: 'Simpan Konfigurasi',

    // Trader Configuration
    positionMode: 'Mode Posisi',
    crossMarginMode: 'Cross Margin',
    isolatedMarginMode: 'Isolated Margin',
    crossMarginDescription:
      'Cross margin: Semua posisi berbagi saldo akun sebagai jaminan',
    isolatedMarginDescription:
      'Isolated margin: Setiap posisi mengelola jaminan secara independen, isolasi risiko',
    leverageConfiguration: 'Konfigurasi Leverage',
    btcEthLeverage: 'Leverage BTC/ETH',
    altcoinLeverage: 'Leverage Altcoin',
    leverageRecommendation:
      'Disarankan: BTC/ETH 5-10x, Altcoin 3-5x untuk kontrol risiko',
    tradingSymbols: 'Simbol Trading',
    tradingSymbolsPlaceholder:
      'Masukkan simbol, pisahkan dengan koma (misal BTCUSDT,ETHUSDT,SOLUSDT)',
    selectSymbols: 'Pilih Simbol',
    selectTradingSymbols: 'Pilih Simbol Trading',
    selectedSymbolsCount: '{count} simbol dipilih',
    clearSelection: 'Hapus Semua',
    confirmSelection: 'Konfirmasi',
    tradingSymbolsDescription:
      'Kosong = gunakan simbol default. Gunakan perp USDT (misal BTCUSDT, ETHUSDT) atau market Hyperliquid XYZ USDC (misal TSLA-USDC)',
    btcEthLeverageValidation: 'Leverage BTC/ETH harus antara 1-50x',
    altcoinLeverageValidation: 'Leverage Altcoin harus antara 1-20x',
    invalidSymbolFormat:
      'Format simbol tidak valid: {symbol}, gunakan perp USDT atau SYMBOL-USDC',
    systemPromptTemplate: 'Template Prompt Sistem',
    promptTemplateDefault: 'Default Stabil',
    promptTemplateAdaptive: 'Strategi Konservatif',
    promptTemplateAdaptiveRelaxed: 'Strategi Agresif',
    promptTemplateHansen: 'Strategi Hansen',
    promptTemplateNof1: 'Framework NoF1 English',
    promptTemplateTaroLong: 'Taro Long Position',
    promptDescDefault: '📊 Strategi Default Stabil',
    promptDescDefaultContent:
      'Maksimalkan rasio Sharpe, risiko-imbalan seimbang, cocok untuk pemula dan trading jangka panjang stabil',
    promptDescAdaptive: '🛡️ Strategi Konservatif (v6.0.0)',
    promptDescAdaptiveContent:
      'Kontrol risiko ketat, konfirmasi BTC wajib, prioritas win rate tinggi, cocok untuk trader konservatif',
    promptDescAdaptiveRelaxed: '⚡ Strategi Agresif (v6.0.0)',
    promptDescAdaptiveRelaxedContent:
      'Trading frekuensi tinggi, konfirmasi BTC opsional, mengejar peluang trading, cocok untuk pasar volatil',
    promptDescHansen: '🎯 Strategi Hansen',
    promptDescHansenContent:
      'Strategi kustom Hansen, maksimalkan rasio Sharpe, untuk trader profesional',
    promptDescNof1: '🌐 Framework NoF1 English',
    promptDescNof1Content:
      'Spesialis bursa Hyperliquid, prompt bahasa Inggris, maksimalkan return yang disesuaikan risiko',
    promptDescTaroLong: '📈 Strategi Taro Long Position',
    promptDescTaroLongContent:
      'Keputusan berbasis data, validasi multi-dimensi, evolusi pembelajaran berkelanjutan, spesialis posisi long',
    loading: 'Memuat...',

    // AI Traders Page - Additional
    inUse: 'Digunakan',
    noModelsConfigured: 'Belum ada model AI yang dikonfigurasi',
    noExchangesConfigured: 'Belum ada bursa yang dikonfigurasi',
    signalSource: 'Sumber Sinyal',
    signalSourceConfig: 'Konfigurasi Sumber Sinyal',
    ai500Description:
      'Endpoint API untuk penyedia data AI500, kosongkan untuk menonaktifkan sumber sinyal ini',
    oiTopDescription:
      'Endpoint API untuk peringkat open interest, kosongkan untuk menonaktifkan sumber sinyal ini',
    information: 'Informasi',
    signalSourceInfo1:
      '• Konfigurasi sumber sinyal per-pengguna, setiap pengguna dapat mengatur URL sendiri',
    signalSourceInfo2:
      '• Saat membuat trader, Anda dapat memilih apakah akan menggunakan sumber sinyal ini',
    signalSourceInfo3:
      '• URL yang dikonfigurasi akan digunakan untuk mengambil data pasar dan sinyal trading',
    editAIModel: 'Edit Model AI',
    addAIModel: 'Tambah Model AI',
    confirmDeleteModel:
      'Apakah Anda yakin ingin menghapus konfigurasi model AI ini?',
    cannotDeleteModelInUse:
      'Tidak dapat menghapus model AI ini karena sedang digunakan oleh trader',
    tradersUsing: 'Trader yang menggunakan konfigurasi ini',
    pleaseDeleteTradersFirst:
      'Silakan hapus atau konfigurasi ulang trader ini terlebih dahulu',
    selectModel: 'Pilih Model AI',
    pleaseSelectModel: 'Silakan pilih model',
    customBaseURL: 'Base URL (Opsional)',
    customBaseURLPlaceholder:
      'URL base API kustom, misal: https://api.openai.com/v1',
    leaveBlankForDefault: 'Kosongkan untuk menggunakan alamat API default',
    modelConfigInfo1:
      '• Untuk API resmi, hanya API Key yang diperlukan, biarkan kolom lain kosong',
    modelConfigInfo2:
      '• Base URL dan Nama Model kustom hanya diperlukan untuk proxy pihak ketiga',
    modelConfigInfo3: '• API Key dienkripsi dan disimpan dengan aman',
    defaultModel: 'Model default',
    applyApiKey: 'Dapatkan API Key',
    kimiApiNote:
      'Kimi memerlukan API Key dari situs internasional (moonshot.ai), key region China tidak kompatibel',
    leaveBlankForDefaultModel: 'Kosongkan untuk menggunakan model default',
    customModelName: 'Nama Model (Opsional)',
    customModelNamePlaceholder: 'misal: deepseek-chat, qwen3-max, gpt-4o',
    saveConfig: 'Simpan Konfigurasi',
    editExchange: 'Edit Bursa',
    addExchange: 'Tambah Bursa',
    confirmDeleteExchange:
      'Apakah Anda yakin ingin menghapus konfigurasi bursa ini?',
    cannotDeleteExchangeInUse:
      'Tidak dapat menghapus bursa ini karena sedang digunakan oleh trader',
    pleaseSelectExchange: 'Silakan pilih bursa',
    exchangeConfigWarning1:
      '• API key akan dienkripsi, disarankan menggunakan izin baca-saja atau trading futures',
    exchangeConfigWarning2:
      '• Jangan berikan izin penarikan untuk memastikan keamanan dana',
    exchangeConfigWarning3:
      '• Setelah menghapus konfigurasi, trader terkait tidak akan dapat trading',
    edit: 'Edit',
    viewGuide: 'Lihat Panduan',
    binanceSetupGuide: 'Panduan Pengaturan Binance',
    closeGuide: 'Tutup',
    whitelistIP: 'Whitelist IP',
    whitelistIPDesc: 'Binance memerlukan penambahan IP server ke whitelist API',
    serverIPAddresses: 'Alamat IP Server',
    copyIP: 'Salin',
    ipCopied: 'IP Disalin',
    copyIPFailed: 'Gagal menyalin alamat IP. Silakan salin secara manual',
    loadingServerIP: 'Memuat IP server...',

    // Error Messages
    createTraderFailed: 'Gagal membuat trader',
    getTraderConfigFailed: 'Gagal mendapatkan konfigurasi trader',
    modelConfigNotExist: 'Konfigurasi model tidak ada atau tidak diaktifkan',
    exchangeConfigNotExist: 'Konfigurasi bursa tidak ada atau tidak diaktifkan',
    updateTraderFailed: 'Gagal memperbarui trader',
    deleteTraderFailed: 'Gagal menghapus trader',
    operationFailed: 'Operasi gagal',
    deleteConfigFailed: 'Gagal menghapus konfigurasi',
    modelNotExist: 'Model tidak ada',
    saveConfigFailed: 'Gagal menyimpan konfigurasi',
    exchangeNotExist: 'Bursa tidak ada',
    deleteExchangeConfigFailed: 'Gagal menghapus konfigurasi bursa',
    saveSignalSourceFailed: 'Gagal menyimpan konfigurasi sumber sinyal',
    encryptionFailed: 'Gagal mengenkripsi data sensitif',

    // Login & Register
    login: 'Masuk',
    register: 'Daftar',
    username: 'Nama Pengguna',
    email: 'Email',
    password: 'Kata Sandi',
    confirmPassword: 'Konfirmasi Kata Sandi',
    usernamePlaceholder: 'nama pengguna anda',
    emailPlaceholder: 'email@anda.com',
    passwordPlaceholder: 'Masukkan kata sandi',
    confirmPasswordPlaceholder: 'Masukkan ulang kata sandi',
    passwordRequirements: 'Persyaratan kata sandi',
    passwordRuleMinLength: 'Minimal 8 karakter',
    passwordRuleUppercase: 'Minimal 1 huruf besar',
    passwordRuleLowercase: 'Minimal 1 huruf kecil',
    passwordRuleNumber: 'Minimal 1 angka',
    passwordRuleSpecial: 'Minimal 1 karakter khusus (@#$%!&*?)',
    passwordRuleMatch: 'Kata sandi cocok',
    passwordNotMeetRequirements:
      'Kata sandi tidak memenuhi persyaratan keamanan',
    loginTitle: 'Masuk ke akun Anda',
    registerTitle: 'Buat akun baru',
    loginButton: 'Masuk',
    registerButton: 'Daftar',
    back: 'Kembali',
    noAccount: 'Belum punya akun?',
    hasAccount: 'Sudah punya akun?',
    registerNow: 'Daftar sekarang',
    loginNow: 'Masuk sekarang',
    forgotPassword: 'Lupa kata sandi?',
    forgotAccount: 'Lupa akun?',
    forgotAccountConfirm:
      '⚠️ Ini akan MENGHAPUS PERMANEN semua data: pengguna, trader, strategi, kunci API model AI, kunci API bursa, dan dompet CLAW402 Anda. Ekspor apa pun yang ingin Anda simpan (terutama kunci privat dompet) SEBELUM melanjutkan. Pendaftaran ulang TIDAK akan memulihkannya. Lanjutkan?',
    forgotAccountSuccess:
      'Akun berhasil direset! Anda sekarang dapat mendaftar akun baru.',
    rememberMe: 'Ingat saya',
    resetPassword: 'Reset Kata Sandi',
    resetPasswordTitle: 'Reset kata sandi Anda',
    newPassword: 'Kata Sandi Baru',
    newPasswordPlaceholder: 'Masukkan kata sandi baru (minimal 6 karakter)',
    resetPasswordButton: 'Reset Kata Sandi',
    resetPasswordSuccess:
      'Kata sandi berhasil direset! Silakan masuk dengan kata sandi baru',
    resetPasswordFailed: 'Gagal mereset kata sandi',
    backToLogin: 'Kembali ke Login',
    resetPasswordCliIntro:
      'Demi keamanan, pemulihan kata sandi tidak lagi tersedia dari browser. Jalankan perintah ini di server tempat NOFX dipasang:',
    resetPasswordCliSecurityNote:
      'Ini memerlukan akses shell ke server, sehingga akun Anda tetap aman bahkan saat NOFX terekspos ke internet.',
    resetAccountCliIntro:
      'Untuk menghapus semua data dan memulai dari awal, jalankan perintah ini di server tempat NOFX dipasang:',
    copy: 'Salin',
    loginSuccess: 'Berhasil masuk',
    registrationSuccess: 'Berhasil mendaftar',
    loginFailed: 'Gagal masuk. Periksa email dan kata sandi Anda.',
    registrationFailed: 'Gagal mendaftar. Silakan coba lagi.',
    sessionExpired: 'Sesi berakhir, silakan masuk kembali',
    invalidCredentials: 'Email atau kata sandi salah',
    weak: 'Lemah',
    medium: 'Sedang',
    strong: 'Kuat',
    passwordStrength: 'Kekuatan kata sandi',
    passwordStrengthHint:
      'Gunakan minimal 8 karakter dengan campuran huruf, angka dan simbol',
    passwordMismatch: 'Kata sandi tidak cocok',
    emailRequired: 'Email diperlukan',
    passwordRequired: 'Kata sandi diperlukan',
    invalidEmail: 'Format email tidak valid',
    passwordTooShort: 'Kata sandi minimal 6 karakter',

    // Landing Page
    features: 'Fitur',
    howItWorks: 'Cara Kerja',
    community: 'Komunitas',
    language: 'Bahasa',
    loggedInAs: 'Masuk sebagai',
    exitLogin: 'Keluar',
    signIn: 'Masuk',
    signUp: 'Daftar',
    registrationClosed: 'Pendaftaran Ditutup',
    registrationClosedMessage:
      'Pendaftaran pengguna saat ini dinonaktifkan. Silakan hubungi administrator untuk akses.',
    githubStarsInDays: '2.5K+ GitHub Stars dalam 3 hari',
    heroTitle1: 'Read the Market.',
    heroTitle2: 'Write the Trade.',
    heroDescription:
      'NOFX adalah standar masa depan untuk trading AI — OS trading agensi yang terbuka dan didorong komunitas. Mendukung Binance, Aster DEX dan bursa lainnya, self-hosted, kompetisi multi-agen, biarkan AI secara otomatis membuat keputusan, mengeksekusi dan mengoptimalkan trading untuk Anda.',
    poweredBy: 'Didukung oleh Aster DEX dan Binance.',
    readyToDefine: 'Siap mendefinisikan masa depan trading AI?',
    startWithCrypto:
      'Dimulai dari pasar kripto, berkembang ke TradFi. NOFX adalah infrastruktur AgentFi.',
    getStartedNow: 'Mulai Sekarang',
    viewSourceCode: 'Lihat Kode Sumber',
    coreFeatures: 'Fitur Inti',
    whyChooseNofx: 'Mengapa Memilih NOFX?',
    openCommunityDriven:
      'Open source, transparan, OS trading AI yang didorong komunitas',
    openSourceSelfHosted: '100% Open Source & Self-Hosted',
    openSourceDesc:
      'Framework Anda, aturan Anda. Non-black box, mendukung prompt kustom dan multi-model.',
    openSourceFeatures1: 'Kode sumber sepenuhnya terbuka',
    openSourceFeatures2: 'Dukungan deployment self-hosting',
    openSourceFeatures3: 'Prompt AI kustom',
    openSourceFeatures4: 'Dukungan multi-model (DeepSeek, Qwen)',
    multiAgentCompetition: 'Kompetisi Multi-Agen Cerdas',
    multiAgentDesc:
      'Strategi AI bertarung kecepatan tinggi di sandbox, yang terkuat bertahan, mencapai evolusi strategi.',
    multiAgentFeatures1: 'Beberapa agen AI berjalan paralel',
    multiAgentFeatures2: 'Optimasi strategi otomatis',
    multiAgentFeatures3: 'Pengujian keamanan sandbox',
    multiAgentFeatures4: 'Portabilitas strategi lintas pasar',
    secureReliableTrading: 'Trading Aman dan Andal',
    secureDesc:
      'Keamanan tingkat enterprise, kontrol penuh atas dana dan strategi trading Anda.',
    secureFeatures1: 'Manajemen private key lokal',
    secureFeatures2: 'Kontrol izin API granular',
    secureFeatures3: 'Pemantauan risiko realtime',
    secureFeatures4: 'Audit log trading',
    aboutNofx: 'Tentang NOFX',
    whatIsNofx: 'Apa itu NOFX?',
    nofxNotAnotherBot:
      "NOFX bukan bot trading biasa, melainkan 'Linux' dari trading AI —",
    nofxDescription1:
      'OS open source yang transparan dan terpercaya yang menyediakan lapisan',
    nofxDescription2:
      "'keputusan-risiko-eksekusi' terpadu, mendukung semua kelas aset.",
    nofxDescription3:
      'Dimulai dari pasar kripto (24/7, volatilitas tinggi sebagai tempat uji sempurna), ekspansi masa depan ke saham, futures, forex. Inti: arsitektur terbuka, AI',
    nofxDescription4:
      'Darwinisme (kompetisi mandiri multi-agen, evolusi strategi), flywheel CodeFi',
    nofxDescription5: '(pengembang mendapat reward poin untuk kontribusi PR).',
    youFullControl: 'Anda 100% Mengendalikan',
    fullControlDesc: 'Kontrol penuh atas prompt AI dan dana',
    startupMessages1: 'Memulai sistem trading otomatis...',
    startupMessages2: 'Server API dimulai di port 8080',
    startupMessages3: 'Konsol Web http://127.0.0.1:3000',
    howToStart: 'Cara Memulai NOFX',
    fourSimpleSteps:
      'Empat langkah sederhana untuk memulai perjalanan trading AI otomatis Anda',
    step1Title: 'Clone Repository GitHub',
    step1Desc:
      'git clone https://github.com/NoFxAiOS/nofx dan beralih ke branch dev untuk menguji fitur baru.',
    step2Title: 'Konfigurasi Lingkungan',
    step2Desc:
      'Setup frontend untuk API bursa (seperti Binance, Hyperliquid), model AI dan prompt kustom.',
    step3Title: 'Deploy & Jalankan',
    step3Desc:
      'Deployment Docker satu klik, mulai agen AI. Catatan: Pasar berisiko tinggi, hanya uji dengan uang yang bisa Anda rugi.',
    step4Title: 'Optimalkan & Kontribusi',
    step4Desc:
      'Pantau trading, kirim PR untuk meningkatkan framework. Bergabung ke Telegram untuk berbagi strategi.',
    importantRiskWarning: 'Peringatan Risiko Penting',
    riskWarningText:
      'Branch dev tidak stabil, jangan gunakan dana yang tidak sanggup Anda rugi. NOFX non-custodial, tanpa strategi resmi. Trading memiliki risiko, investasi dengan hati-hati.',
    futureStandardAI: 'Standar masa depan trading AI',
    links: 'Tautan',
    resources: 'Sumber Daya',
    documentation: 'Dokumentasi',
    supporters: 'Pendukung',
    strategicInvestment: '(Investasi Strategis)',
    accessNofxPlatform: 'Akses Platform NOFX',
    loginRegisterPrompt:
      'Silakan masuk atau daftar untuk mengakses platform trading AI lengkap',
    registerNewAccount: 'Daftar Akun Baru',
    candidateCoins: 'Koin Kandidat',
    candidateCoinsZeroWarning: 'Jumlah Koin Kandidat adalah 0',
    possibleReasons: 'Kemungkinan Penyebab:',
    ai500ApiNotConfigured:
      'API penyedia data AI500 tidak dikonfigurasi atau tidak dapat diakses (periksa pengaturan sumber sinyal)',
    apiConnectionTimeout: 'Koneksi API timeout atau mengembalikan data kosong',
    noCustomCoinsAndApiFailed:
      'Tidak ada koin kustom yang dikonfigurasi dan pengambilan API gagal',
    solutions: 'Solusi:',
    setCustomCoinsInConfig: 'Atur daftar koin kustom di konfigurasi trader',
    orConfigureCorrectApiUrl:
      'Atau konfigurasi alamat API penyedia data yang benar',
    orDisableAI500Options:
      'Atau nonaktifkan opsi "Gunakan Penyedia Data AI500" dan "Gunakan OI Top"',
    signalSourceNotConfigured: 'Sumber Sinyal Belum Dikonfigurasi',
    signalSourceWarningMessage:
      'Anda memiliki trader yang mengaktifkan "Gunakan Penyedia Data AI500" atau "Gunakan OI Top", tetapi alamat API sumber sinyal belum dikonfigurasi. Ini akan menyebabkan jumlah koin kandidat menjadi 0, dan trader tidak dapat bekerja dengan baik.',
    configureSignalSourceNow: 'Konfigurasi Sumber Sinyal Sekarang',

    // FAQ Page

    // Web Crypto Environment Check
    environmentCheck: {
      button: 'Periksa Lingkungan Aman',
      checking: 'Memeriksa...',
      description:
        'Memverifikasi otomatis apakah konteks browser ini memungkinkan Web Crypto sebelum memasukkan kunci sensitif.',
      secureTitle: 'Konteks aman terdeteksi',
      secureDesc:
        'API Web Crypto tersedia. Anda dapat melanjutkan memasukkan rahasia dengan enkripsi diaktifkan.',
      insecureTitle: 'Konteks tidak aman terdeteksi',
      insecureDesc:
        'Halaman ini tidak berjalan melalui HTTPS atau origin localhost tepercaya.',
      tipsTitle: 'Cara memperbaiki:',
      tipHTTPS: 'Sajikan dasbor melalui HTTPS dengan sertifikat valid.',
      tipLocalhost:
        'Selama pengembangan, buka aplikasi via http://localhost atau 127.0.0.1.',
      tipIframe:
        'Hindari menyematkan aplikasi dalam iframe HTTP yang tidak aman.',
      unsupportedTitle: 'Browser tidak mengekspos Web Crypto',
      unsupportedDesc:
        'Buka NOFX melalui HTTPS (atau http://localhost saat pengembangan).',
      summary: 'Origin saat ini: {origin} · Protokol: {protocol}',
      disabledTitle: 'Enkripsi transport dinonaktifkan',
      disabledDesc:
        'Enkripsi transport sisi server dinonaktifkan. API key akan ditransmisikan dalam plaintext. Aktifkan TRANSPORT_ENCRYPTION=true untuk keamanan yang lebih baik.',
    },
    environmentSteps: {
      checkTitle: '1. Pemeriksaan lingkungan',
      selectTitle: '2. Pilih bursa',
    },
    twoStageKey: {
      title: 'Input Private Key Dua Tahap',
      stage1Description: 'Masukkan {length} karakter pertama private key Anda',
      stage2Description: 'Masukkan {length} karakter sisa private key Anda',
      stage1InputLabel: 'Bagian Pertama',
      stage2InputLabel: 'Bagian Kedua',
      characters: 'karakter',
      processing: 'Memproses...',
      nextButton: 'Lanjut',
      cancelButton: 'Batal',
      backButton: 'Kembali',
      encryptButton: 'Enkripsi & Kirim',
      obfuscationCopied: 'Data pengacak disalin ke clipboard',
      obfuscationInstruction:
        'Tempelkan sesuatu yang lain untuk membersihkan clipboard, lalu lanjutkan',
      obfuscationManual: 'Diperlukan pengacakan manual',
    },
    errors: {
      privatekeyIncomplete: 'Masukkan minimal {expected} karakter',
      privatekeyInvalidFormat:
        'Format private key tidak valid (harus 64 karakter heksadesimal)',
      privatekeyObfuscationFailed: 'Pengacakan clipboard gagal',
    },
    positionHistory: {
      title: 'Riwayat Posisi',
      loading: 'Memuat riwayat posisi...',
      noHistory: 'Tidak Ada Riwayat Posisi',
      noHistoryDesc: 'Posisi yang ditutup akan muncul di sini setelah trading.',
      showingPositions: 'Menampilkan {count} dari {total} posisi',
      totalPnL: 'Total L/R',
      totalTrades: 'Total Trading',
      winLoss: 'Menang: {win} / Kalah: {loss}',
      winRate: 'Win Rate',
      profitFactor: 'Profit Factor',
      profitFactorDesc: 'Total Profit / Total Loss',
      plRatio: 'Rasio L/R',
      plRatioDesc: 'Rata-rata Menang / Rata-rata Kalah',
      sharpeRatio: 'Rasio Sharpe',
      sharpeRatioDesc: 'Return yang Disesuaikan Risiko',
      maxDrawdown: 'Drawdown Maksimum',
      avgWin: 'Rata-rata Menang',
      avgLoss: 'Rata-rata Kalah',
      netPnL: 'L/R Bersih',
      netPnLDesc: 'Setelah Biaya',
      fee: 'Biaya',
      trades: 'Trading',
      avgPnL: 'Rata-rata L/R',
      symbolPerformance: 'Performa Simbol',
      symbol: 'Simbol',
      allSymbols: 'Semua Simbol',
      side: 'Arah',
      all: 'Semua',
      sort: 'Urutkan',
      latestFirst: 'Terbaru Dulu',
      oldestFirst: 'Terlama Dulu',
      highestPnL: 'L/R Tertinggi',
      lowestPnL: 'L/R Terendah',
      entry: 'Masuk',
      exit: 'Keluar',
      qty: 'Jml',
      value: 'Nilai',
      lev: 'Lev',
      pnl: 'L/R',
      duration: 'Durasi',
      closedAt: 'Ditutup Pada',
    },

    // Data Page
    dataCenter: 'Data Center',

    // Strategy Market Page
    strategyMarket: {
      title: 'PASAR STRATEGI',
      subtitle: 'DATABASE STRATEGI GLOBAL',
      description:
        'Temukan, analisis, dan kloning algoritma trading berperforma tinggi',
      search: 'CARI PARAMETER...',
      all: 'SEMUA PROTOKOL',
      popular: 'TREN',
      recent: 'TERBARU',
      myStrategies: 'PERPUSTAKAAN SAYA',
      noStrategies: 'TIDAK ADA SINYAL',
      noStrategiesDesc:
        'Tidak ada sinyal strategis terdeteksi pada frekuensi ini',
      author: 'OPERATOR',
      createdAt: 'TIMESTAMP',
      viewConfig: 'DEKRIPSI CONFIG',
      hideConfig: 'ENKRIPSI',
      copyConfig: 'KLON CONFIG',
      copied: 'DISALIN',
      configHidden: 'TERENKRIPSI',
      configHiddenDesc: 'Parameter konfigurasi terenkripsi',
      indicators: 'INDIKATOR',
      maxPositions: 'BATAS_POS',
      maxLeverage: 'LEV_MAKS',
      shareYours: 'UNGGAH_STRATEGI',
      makePublic: 'PUBLIKASI',
      loading: 'MENGINISIALISASI...',
    },

    // Strategy Studio Page
    strategyStudio: {
      title: 'Studio Strategi',
      subtitle: 'Konfigurasi dan uji strategi trading',
      strategies: 'Strategi',
      newStrategy: 'Baru',
      strategyType: 'Jenis Strategi',
      aiTrading: 'AI Trading',
      aiTradingDesc: 'AI menganalisis pasar dan membuat keputusan trading',
      gridTrading: 'AI Grid Trading',
      gridTradingDesc: 'Strategi grid yang dikontrol AI untuk pasar ranging',
      gridConfig: 'Konfigurasi Grid',
      coinSource: 'Sumber Koin',
      indicators: 'Indikator',
      riskControl: 'Kontrol Risiko',
      promptSections: 'Editor Prompt',
      customPrompt: 'Prompt Ekstra',
      save: 'Simpan',
      saving: 'Menyimpan...',
      activate: 'Aktifkan',
      active: 'Aktif',
      default: 'Default',
      promptPreview: 'Pratinjau Prompt',
      aiTestRun: 'Uji AI',
      systemPrompt: 'System Prompt',
      userPrompt: 'User Prompt',
      loadPrompt: 'Generate Prompt',
      refreshPrompt: 'Refresh',
      promptVariant: 'Gaya',
      balanced: 'Seimbang',
      aggressive: 'Agresif',
      conservative: 'Konservatif',
      selectModel: 'Pilih Model AI',
      runTest: 'Jalankan Uji AI',
      running: 'Berjalan...',
      aiOutput: 'Output AI',
      reasoning: 'Penalaran',
      decisions: 'Keputusan',
      duration: 'Durasi',
      noModel: 'Silakan konfigurasi model AI terlebih dahulu',
      testNote: 'Uji dengan AI nyata, tanpa trading',
      publishSettings: 'Publikasi',
      newStrategyName: 'Strategi Baru',
      strategyCopy: 'Salinan Strategi',
      strategyDeleted: 'Strategi dihapus',
      cannotDeleteActiveStrategy: 'Strategi aktif tidak bisa dihapus',
      confirmDeleteStrategy: 'Hapus strategi ini?',
      confirmDelete: 'Konfirmasi Hapus',
      delete: 'Hapus',
      cancel: 'Batal',
      strategyExported: 'Strategi diekspor',
      invalidStrategyFile: 'File strategi tidak valid',
      imported: 'Diimpor',
      strategyImported: 'Strategi diimpor',
      strategySaved: 'Strategi disimpan',
      importStrategy: 'Impor Strategi',
      newStrategyTooltip: 'Strategi Baru',
      export: 'Ekspor',
      duplicate: 'Duplikat',
      deleteTooltip: 'Hapus',
      public: 'Publik',
      addDescription: 'Tambah deskripsi strategi...',
      unsaved: 'Belum Disimpan',
      discardChanges: 'Buang',
      selectOrCreate: 'Pilih atau buat strategi',
      customPromptDesc:
        'Prompt tambahan di akhir System Prompt untuk gaya trading personal',
      customPromptPlaceholder: 'Masukkan prompt kustom...',
      generatePromptPreview: 'Klik untuk generate pratinjau prompt',
      runAiTestHint: 'Klik untuk menjalankan uji AI',
      tokenEstimate: 'Estimasi Token',
      tokenExceedWarning:
        'Estimasi token melebihi 128K. Permintaan AI mungkin gagal untuk beberapa model.',
      tokenEstimating: 'Mengestimasi...',
      tokenTooltip: 'Berdasarkan konteks 200K',
    },

    // Metric Tooltip
    metricTooltip: {
      formula: 'Formula',
    },

    // Login Required Overlay
    loginRequired: {
      title: 'AKSES SISTEM DITOLAK',
      accessDenied: 'AKSES DITOLAK',
      subtitleWithFeature:
        'Modul "{featureName}" memerlukan hak akses lebih tinggi',
      subtitleDefault: 'Otorisasi diperlukan untuk modul ini',
      description:
        'Inisialisasi protokol autentikasi untuk membuka kemampuan sistem penuh: konfigurasi Trader AI dan aliran data Pasar Strategi.',
      benefit1: 'Kontrol Trader AI',
      benefit2: 'Pasar Strategi HFT',
      benefit4: 'Visualisasi Sistem Penuh',
      loginButton: 'JALANKAN LOGIN',
      registerButton: 'DAFTAR ID BARU',
      abort: 'BATALKAN',
    },

    // Advanced Chart
    advancedChart: {
      updating: 'Memperbarui...',
      indicators: 'Indikator',
      orderMarkers: 'Penanda Order',
      technicalIndicators: 'Indikator Teknikal',
      clickToToggle: 'Klik untuk beralih indikator',
      shares: 'lembar',
      units: 'unit',
    },

    // Chart With Orders
    chartWithOrders: {
      failedToLoad: 'Gagal memuat data grafik',
      loading: 'Memuat...',
      buy: 'BELI',
      sell: 'JUAL',
    },

    // Comparison Chart
    comparisonChart: {
      '1d': '1H',
      '3d': '3H',
      '7d': '7H',
      '30d': '30H',
      all: 'Semua',
    },

    traderDashboard: {
      connectionFailed: 'Koneksi Gagal',
      connectionFailedDesc: 'Silakan periksa apakah layanan backend berjalan.',
      retry: 'Coba Lagi',
      confirmClosePosition: 'Yakin ingin menutup posisi {symbol} {side}?',
      confirmClose: 'Konfirmasi Tutup',
      confirm: 'Konfirmasi',
      cancel: 'Batal',
      positionClosed: 'Posisi berhasil ditutup',
      closeFailed: 'Gagal menutup posisi',
      hideAddress: 'Sembunyikan alamat',
      showFullAddress: 'Tampilkan alamat lengkap',
      copyAddress: 'Salin alamat',
      noAddressConfigured: 'Alamat belum dikonfigurasi',
      action: 'Aksi',
      entry: 'Entry',
      mark: 'Mark',
      qty: 'Qty',
      value: 'Nilai',
      lev: 'Lev.',
      uPnL: 'uPnL',
      liq: 'Liq.',
      closePosition: 'Tutup Posisi',
      close: 'Tutup',
      showingPositions: 'Menampilkan {shown} dari {total} posisi',
      perPage: 'Per halaman',
      accountFetchFailed:
        'DATA_FETCH::FAILED — Data akun tidak tersedia, periksa koneksi',
      positionsFetchFailed: 'Data posisi tidak tersedia',
      decisionsFetchFailed: 'Data keputusan tidak tersedia',
    },

    aiTradersToast: {
      creating: 'Membuat...',
      created: 'Berhasil dibuat',
      createFailed: 'Gagal membuat',
      saving: 'Menyimpan...',
      saved: 'Berhasil disimpan',
      saveFailed: 'Gagal menyimpan',
      deleting: 'Menghapus...',
      deleted: 'Berhasil dihapus',
      deleteFailed: 'Gagal menghapus',
      stopping: 'Menghentikan...',
      stopped: 'Dihentikan',
      stopFailed: 'Gagal menghentikan',
      starting: 'Memulai...',
      started: 'Dimulai',
      startFailed: 'Gagal memulai',
      updating: 'Memperbarui...',
      updatingConfig: 'Memperbarui konfigurasi...',
      configUpdated: 'Konfigurasi diperbarui',
      configUpdateFailed: 'Gagal memperbarui konfigurasi',
      showInCompetition: 'Ditampilkan di kompetisi',
      hideInCompetition: 'Disembunyikan dari kompetisi',
      updateFailed: 'Gagal memperbarui',
      updatingModelConfig: 'Memperbarui konfigurasi model...',
      modelConfigUpdated: 'Konfigurasi model diperbarui',
      modelConfigUpdateFailed: 'Gagal memperbarui konfigurasi model',
      deletingExchange: 'Menghapus akun exchange...',
      exchangeDeleted: 'Akun exchange dihapus',
      exchangeDeleteFailed: 'Gagal menghapus akun exchange',
      updatingExchangeConfig: 'Memperbarui konfigurasi exchange...',
      exchangeConfigUpdated: 'Konfigurasi exchange diperbarui',
      exchangeConfigUpdateFailed: 'Gagal memperbarui konfigurasi exchange',
      creatingExchange: 'Membuat akun exchange...',
      exchangeCreated: 'Akun exchange dibuat',
      exchangeCreateFailed: 'Gagal membuat akun exchange',
    },

    modelConfig: {
      selectModel: 'Pilih Model',
      configure: 'Konfigurasi',
      configureApi: 'Konfigurasi API',
      configureWallet: 'Konfigurasi Wallet',
      chooseProvider: 'Pilih Penyedia AI Anda',
      claw402EntryDesc:
        'Jalur default yang direkomendasikan. Gunakan Base USDC bayar per panggilan tanpa mengelola API key.',
      otherApiEntry: 'Penyedia API Lain',
      otherApiEntryDesc:
        'Gunakan API key Anda sendiri untuk OpenAI, Claude, Gemini, DeepSeek, dan lainnya.',
      payPerCall: 'Bayar per panggilan USDC · Semua Model AI · Tanpa API Key',
      recommended: 'Terbaik',
      allModelsClaw:
        'Bayar per panggilan dengan USDC — mendukung semua model AI utama',
      selectAiModel: 'Pilih Model AI',
      allModelsUnified:
        'Semua model terpadu via Claw402. Ganti kapan saja setelah setup.',
      setupWallet: 'Setup Wallet',
      walletInfo:
        'Claw402 menggunakan USDC di Base chain. Anda memerlukan wallet EVM.',
      exportKey: 'Ekspor private key dari MetaMask, Rabby, dll.',
      dedicatedWallet: 'Disarankan: buat wallet khusus dengan saldo USDC kecil',
      walletPrivateKey: 'Private Key Wallet (Base Chain EVM)',
      privateKeyNote:
        'Private key hanya digunakan untuk signing lokal. Tidak pernah diunggah. Tidak perlu ETH atau gas.',
      howToFundUsdc: 'Cara Mengisi USDC',
      fundStep1:
        'Tarik USDC dari exchange (Binance/OKX/Coinbase) ke wallet Anda',
      fundStep2: 'Pilih jaringan Base (biaya sangat rendah)',
      fundStep3: '$5-10 USDC cukup untuk waktu lama (~$0.003/panggilan)',
      back: 'Kembali',
      startTrading: 'Mulai Trading',
      modelsConfigured: 'Model dengan lencana emas sudah dikonfigurasi',
      getStarted: 'Mulai',
      getApiKey: 'Dapatkan API Key',
      walletPrivateKeyLabel: 'Private Key Wallet *',
      selectModelLabel: 'Pilih Model',
      validating: 'Memvalidasi...',
      walletAddress: 'Alamat Wallet',
      usdcBalance: 'Saldo Base USDC',
      claw402Connected: 'claw402 Terhubung',
      claw402Unreachable: 'claw402 Tidak Dapat Dijangkau',
      depositUsdc: 'Deposit USDC ke alamat ini di Base chain',
      invalidKeyPrefix: 'Tambahkan 0x di awal',
      invalidKeyLength: 'Harus 66 karakter, saat ini',
      invalidKeyChars: 'Mengandung karakter tidak valid',
      testConnection: 'Tes Koneksi',
      testingConnection: 'Menguji...',
    },

    exchangeConfig: {
      selectExchange: 'Pilih Exchange',
      configure: 'Konfigurasi',
      chooseExchange: 'Pilih Exchange Anda',
      centralizedExchanges: 'Exchange Tersentralisasi',
      decentralizedExchanges: 'Exchange Terdesentralisasi',
      register: 'Daftar',
      bonus: 'Bonus',
      accountName: 'Nama Akun',
      accountNamePlaceholder: 'mis., Akun Utama',
      pleaseEnterAccountName: 'Silakan masukkan nama akun',
      useBinanceFuturesApi: 'Gunakan API "Spot & Futures Trading"',
      viewTutorial: 'Lihat Tutorial',
      lighterApiKeySetup: 'Setup API Key Lighter',
      lighterApiKeyDesc: 'Buat API Key di situs Lighter',
      apiKeyIndex: 'Indeks API Key',
      apiKeyIndexTooltip: 'Indeks API Key dimulai dari 0',
      back: 'Kembali',
    },

    telegram: {
      botSetup: 'Setup Telegram Bot',
      createBot: 'Buat Bot',
      bindAccount: 'Hubungkan Akun',
      done: 'Selesai',
      invalidTokenFormat:
        'Format Bot Token tidak valid. Seharusnya "angka:alfanumerik"',
      tokenSaved: 'Bot Token tersimpan, menunggu binding',
      saveFailed: 'Gagal menyimpan, silakan periksa token',
      unbound: 'Akun Telegram terputus',
      unbindFailed: 'Gagal memutuskan',
      step1Title: 'Langkah 1: Buat Bot di Telegram',
      step1Desc1: 'Buka Telegram, cari',
      step1Desc2: 'Kirim',
      step1Desc2Suffix: 'perintah',
      step1Desc3: 'Ikuti petunjuk untuk mengatur nama dan username bot',
      step1Desc4: 'BotFather akan mengembalikan Token, salin itu',
      openBotFather: 'Buka @BotFather',
      pasteToken: 'Tempel Bot Token',
      tokenFormat: 'Format: angka:alfanumerik, mis. 123456789:ABCdef...',
      selectAiModel: 'Pilih Model AI (opsional)',
      noEnabledModels:
        'Belum ada model aktif. Konfigurasi di AI Models terlebih dahulu.',
      autoSelect: '— Pilih otomatis (disarankan)',
      autoUseEnabled: 'Kosongkan untuk otomatis menggunakan model aktif',
      savingToken: 'Menyimpan...',
      saveAndContinue: 'Simpan & Lanjut',
      step2Title: 'Langkah 2: Kirim /start ke Bot Anda',
      step2Desc1: 'Cari Bot yang baru dibuat di Telegram',
      step2Desc2: 'Klik Start atau kirim',
      step2Desc3: 'Bot akan otomatis terhubung ke akun Anda',
      currentToken: 'Token Saat Ini',
      waitingForStart:
        'Menunggu Anda mengirim /start... Refresh halaman setelah mengirim',
      reconfigureToken: 'Konfigurasi Ulang Token',
      bindSuccess: 'Berhasil terhubung!',
      noStartReceived:
        'Belum menerima /start. Silakan kirim /start ke Bot Anda terlebih dahulu',
      checkFailed: 'Pemeriksaan gagal',
      checkStatus: 'Periksa Status',
      botActive: 'Telegram Bot Aktif!',
      botActiveDesc:
        'Anda sekarang dapat mengontrol sistem trading melalui bahasa alami di Telegram',
      supportedCommands: 'Perintah yang Didukung',
      cmdHelp: 'Tampilkan semua perintah',
      cmdStatus: 'Tampilkan status trader',
      cmdNaturalLang: 'Bahasa alami',
      cmdStartStop: 'Mulai/hentikan trader',
      cmdControl: 'Kontrol bahasa alami',
      cmdPositions: 'Lihat posisi',
      cmdPositionsDesc: 'Kueri posisi real-time',
      cmdStrategy: 'Konfigurasi strategi',
      cmdStrategyDesc: 'Ubah strategi trading',
      unbinding: 'Memutuskan...',
      unbindAccount: 'Putuskan Akun',
      aiModelLabel: 'Model AI (untuk bahasa alami)',
      aiModelAutoSelect: '— Pilih otomatis',
      modelUpdated: 'Model AI diperbarui',
      modelUpdateFailed: 'Gagal memperbarui',
      save: 'Simpan',
      loading: 'Memuat...',
    },

    traderConfigView: {
      traderConfig: 'Konfigurasi Trader',
      configInfo: 'Detail konfigurasi {name}',
      running: 'Berjalan',
      stopped: 'Berhenti',
      basicInfo: 'Informasi Dasar',
      traderName: 'Nama Trader',
      aiModel: 'Model AI',
      exchange: 'Exchange',
      initialBalance: 'Saldo Awal',
      marginMode: 'Mode Margin',
      crossMargin: 'Cross',
      isolatedMargin: 'Isolated',
      scanInterval: '{minutes} menit',
      scanIntervalLabel: 'Interval Scan',
      strategyUsed: 'Strategi Digunakan',
      strategyName: 'Nama Strategi',
      close: 'Tutup',
      yes: 'Ya',
      no: 'Tidak',
    },
  },
}

export function t(
  key: string,
  lang: Language,
  params?: Record<string, string | number>
): string {
  // Handle nested keys like 'twoStageKey.title'
  const keys = key.split('.')
  let value: any = translations[lang]

  for (const k of keys) {
    value = value?.[k]
  }

  let text = typeof value === 'string' ? value : key

  // Replace parameters like {count}, {gap}, etc.
  if (params) {
    Object.entries(params).forEach(([param, value]) => {
      text = text.replace(`{${param}}`, String(value))
    })
  }

  return text
}
