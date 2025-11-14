export type Language = 'en' | 'zh'

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
    realtimeNav: 'Live',
    configNav: 'Config',
    dashboardNav: 'Dashboard',
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

    // AI Learning
    aiLearning: 'AI Learning & Reflection',
    tradesAnalyzed: '{count} trades analyzed · Real-time evolution',
    latestReflection: 'Latest Reflection',
    fullCoT: 'Full Chain of Thought',
    totalTrades: 'Total Trades',
    winRate: 'Win Rate',
    avgWin: 'Avg Win',
    avgLoss: 'Avg Loss',
    profitFactor: 'Profit Factor',
    avgWinDivLoss: 'Avg Win ÷ Avg Loss',
    excellent: '🔥 Excellent - Strong profitability',
    good: '✓ Good - Stable profits',
    fair: '⚠️ Fair - Needs optimization',
    poor: '❌ Poor - Losses exceed gains',
    bestPerformer: 'Best Performer',
    worstPerformer: 'Worst Performer',
    symbolPerformance: 'Symbol Performance',
    tradeHistory: 'Trade History',
    completedTrades: 'Recent {count} completed trades',
    noCompletedTrades: 'No completed trades yet',
    completedTradesWillAppear: 'Completed trades will appear here',
    entry: 'Entry',
    exit: 'Exit',
    stopLoss: 'Stop Loss',
    latest: 'Latest',

    // AI Learning Description
    howAILearns: 'How AI Learns & Evolves',
    aiLearningPoint1: 'Analyzes last 20 trading cycles before each decision',
    aiLearningPoint2: 'Identifies best & worst performing symbols',
    aiLearningPoint3: 'Optimizes position sizing based on win rate',
    aiLearningPoint4: 'Avoids repeating past mistakes',

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
    scanIntervalRecommend: 'Recommended: 3-10 minutes',
    useTestnet: 'Use Testnet',
    enabled: 'Enabled',
    save: 'Save',

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
    enterPassphrase: 'Enter Passphrase (Required for OKX)',
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
    asterUserDesc:
      'Main wallet address - The EVM wallet address you use to log in to Aster (Note: Only EVM wallets are supported, Solana wallets are not supported)',
    asterSignerDesc:
      'API wallet address - Generate from https://www.asterdex.com/en/api-wallet',
    asterPrivateKeyDesc:
      'API wallet private key - Get from https://www.asterdex.com/en/api-wallet (only used locally for signing, never transmitted)',
    asterUsdtWarning:
      'Important: Aster only tracks USDT balance. Please ensure you use USDT as margin currency to avoid P&L calculation errors caused by price fluctuations of other assets (BNB, ETH, etc.)',

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
      'Empty = use default symbols. Must end with USDT (e.g., BTCUSDT, ETHUSDT)',
    btcEthLeverageValidation: 'BTC/ETH leverage must be between 1-50x',
    altcoinLeverageValidation: 'Altcoin leverage must be between 1-20x',
    invalidSymbolFormat: 'Invalid symbol format: {symbol}, must end with USDT',

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
    loadingError: '⚠️ Failed to load AI learning data',
    noCompleteData:
      'No complete trading data (needs to complete open → close cycle)',

    // AI Traders Page - Additional
    inUse: 'In Use',
    noModelsConfigured: 'No configured AI models',
    noExchangesConfigured: 'No configured exchanges',
    signalSource: 'Signal Source',
    signalSourceConfig: 'Signal Source Configuration',
    coinPoolDescription:
      'API endpoint for coin pool data, leave blank to disable this signal source',
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
      '• API Key will be encrypted and stored, please ensure it is valid',
    modelConfigInfo2: '• Base URL is used for custom API server address',
    modelConfigInfo3:
      '• After deleting configuration, traders using this model will not work properly',
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
    otpPlaceholder: '000000',
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
    rememberMe: 'Remember me',
    otpCode: 'OTP Code',
    resetPassword: 'Reset Password',
    resetPasswordTitle: 'Reset your password',
    newPassword: 'New Password',
    newPasswordPlaceholder: 'Enter new password (at least 6 characters)',
    resetPasswordButton: 'Reset Password',
    resetPasswordSuccess:
      'Password reset successful! Please login with your new password',
    resetPasswordFailed: 'Password reset failed',
    backToLogin: 'Back to Login',
    scanQRCode: 'Scan QR Code',
    enterOTPCode: 'Enter 6-digit OTP code',
    verifyOTP: 'Verify OTP',
    setupTwoFactor: 'Set up two-factor authentication',
    setupTwoFactorDesc:
      'Follow the steps below to secure your account with Google Authenticator',
    scanQRCodeInstructions:
      'Scan this QR code with Google Authenticator or Authy',
    otpSecret: 'Or enter this secret manually:',
    qrCodeHint: 'QR code (if scanning fails, use the secret below):',
    authStep1Title: 'Step 1: Install Google Authenticator',
    authStep1Desc:
      'Download and install Google Authenticator from your app store',
    authStep2Title: 'Step 2: Add account',
    authStep2Desc: 'Tap "+", then choose "Scan QR code" or "Enter a setup key"',
    authStep3Title: 'Step 3: Verify setup',
    authStep3Desc: 'After setup, continue to enter the 6-digit code',
    setupCompleteContinue: 'I have completed setup, continue',
    copy: 'Copy',
    completeRegistration: 'Complete Registration',
    completeRegistrationSubtitle: 'to complete registration',
    loginSuccess: 'Login successful',
    registrationSuccess: 'Registration successful',
    loginFailed: 'Login failed. Please check your email and password.',
    registrationFailed: 'Registration failed. Please try again.',
    verificationFailed:
      'OTP verification failed. Please check the code and try again.',
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
    poweredBy:
      'Powered by Aster DEX and Binance, strategically invested by Amber.ac.',

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
    startupMessages3: 'Web console http://localhost:3000',

    // How It Works Section
    howToStart: 'How to Get Started with NOFX',
    fourSimpleSteps:
      'Four simple steps to start your AI automated trading journey',
    step1Title: 'Clone GitHub Repository',
    step1Desc:
      'git clone https://github.com/tinkle-community/nofx and switch to dev branch to test new features.',
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
    coinPoolApiNotConfigured:
      'Coin pool API not configured or inaccessible (check signal source settings)',
    apiConnectionTimeout: 'API connection timeout or returned empty data',
    noCustomCoinsAndApiFailed:
      'No custom coins configured and API fetch failed',
    solutions: 'Solutions:',
    setCustomCoinsInConfig: 'Set custom coin list in trader configuration',
    orConfigureCorrectApiUrl: 'Or configure correct coin pool API address',
    orDisableCoinPoolOptions:
      'Or disable "Use Coin Pool" and "Use OI Top" options',
    signalSourceNotConfigured: 'Signal Source Not Configured',
    signalSourceWarningMessage:
      'You have traders that enabled "Use Coin Pool" or "Use OI Top", but signal source API address is not configured yet. This will cause candidate coins count to be 0, and traders cannot work properly.',
    configureSignalSourceNow: 'Configure Signal Source Now',

    // FAQ Page
    faqTitle: 'Frequently Asked Questions',
    faqSubtitle: 'Find answers to common questions about NOFX',
    faqStillHaveQuestions: 'Still Have Questions?',
    faqContactUs: 'Join our community or check our GitHub for more help',

    // FAQ Categories
    faqCategoryBasics: 'General Questions',
    faqCategoryContributing: 'Contributing & Tasks',
    faqCategorySetup: 'Setup & Configuration',
    faqCategoryTrading: 'Trading Questions',
    faqCategoryTechnical: 'Technical Issues',
    faqCategoryAI: 'AI & Model Questions',
    faqCategoryData: 'Data & Privacy',

    // FAQ Questions & Answers - General
    faqWhatIsNOFX: 'What is NOFX?',
    faqWhatIsNOFXAnswer:
      'NOFX is an AI-powered cryptocurrency trading bot that uses large language models (LLMs) to make trading decisions on futures markets.',

    faqSupportedExchanges: 'Which exchanges are supported?',
    faqSupportedExchangesAnswer:
      'Binance Futures, Hyperliquid, and Aster DEX are supported. More exchanges coming soon.',

    faqIsProfitable: 'Is NOFX profitable?',
    faqIsProfitableAnswer:
      'AI trading is experimental and not guaranteed to be profitable. Always start with small amounts and never invest more than you can afford to lose.',

    faqMultipleTraders: 'Can I run multiple traders simultaneously?',
    faqMultipleTradersAnswer:
      'Yes! NOFX supports running multiple traders with different configurations, AI models, and trading strategies.',

    // Contributing & Community
    faqGithubProjectsTasks: 'How to use GitHub Projects and pick up tasks?',
    faqGithubProjectsTasksAnswer:
      'Roadmap: https://github.com/orgs/NoFxAiOS/projects/3  • Task Dashboard: https://github.com/orgs/NoFxAiOS/projects/5  • Steps: Open links → filter by labels (good first issue / help wanted / frontend / backend) → read Description & Acceptance Criteria → comment "assign me" or self-assign → Fork the repo → sync your fork\'s dev with upstream/dev → create a feature branch from your fork\'s dev → push to your fork → open PR (base: NoFxAiOS/nofx:dev ← compare: your-username/nofx:feat/your-topic) → reference Issue (Closes #123) and use the proper template.',

    faqContributePR: 'How to properly submit PRs and contribute?',
    faqContributePRAnswer:
      "Guidelines: • Fork first; branch from your fork's dev (avoid direct commits to upstream main) • Branch naming: feat/..., fix/..., docs/...; Conventional Commits • Run checks before PR: npm --prefix web run lint && npm --prefix web run build • For UI changes, attach screenshots or a short video • Choose the proper PR template (frontend/backend/docs/general) • Open PR from your fork to NoFxAiOS/nofx:dev and link Issue (Closes #123) • Keep rebasing onto upstream/dev; ensure CI passes; prefer small, focused PRs • Read CONTRIBUTING.md and .github/PR_TITLE_GUIDE.md",

    // Setup & Configuration
    faqSystemRequirements: 'What are the system requirements?',
    faqSystemRequirementsAnswer:
      'OS: Linux, macOS, or Windows (Docker recommended); RAM: 2GB minimum, 4GB recommended; Disk: 1GB for application + logs; Network: Stable internet connection.',

    faqNeedCoding: 'Do I need coding experience?',
    faqNeedCodingAnswer:
      'No! NOFX has a web UI for all configuration. However, basic command line knowledge helps with setup and troubleshooting.',

    faqGetApiKeys: 'How do I get API keys?',
    faqGetApiKeysAnswer:
      'For Binance: Account → API Management → Create API → Enable Futures. For Hyperliquid: Visit Hyperliquid App → API Settings. For Aster DEX: Configure main wallet address (User), API wallet address (Signer), and private key (Private Key).',

    faqUseSubaccount: 'Should I use a subaccount?',
    faqUseSubaccountAnswer:
      'Recommended: Yes, use a subaccount dedicated to NOFX for better risk isolation. However, note that some subaccounts have restrictions (e.g., 5x max leverage on Binance).',

    faqDockerDeployment: 'Docker deployment keeps failing',
    faqDockerDeploymentAnswer:
      'Common issues: Network connection problems, dependency installation failures, insufficient memory (needs at least 2C2G). If stuck at "go build", try: docker compose down && docker compose build --no-cache && docker compose up -d',

    faqBalanceZero: 'Account balance shows 0',
    faqBalanceZeroAnswer:
      'Funds are likely in spot account instead of futures account, or locked in savings products. You need to manually transfer funds to futures account in Binance.',

    faqTestnet: 'Can I use testnet for testing?',
    faqTestnetAnswer:
      'Testnet is not supported at the moment. We recommend using real trading with small amounts (10-50 USDT) for testing.',

    // Trading Questions
    faqNoTrades: "Why isn't my trader making any trades?",
    faqNoTradesAnswer:
      'Common reasons: AI decided to "wait" due to market conditions; Insufficient balance or margin; Position limits reached (default: max 3 positions); Check troubleshooting guide for detailed diagnostics.',

    faqDecisionFrequency: 'How often does the AI make decisions?',
    faqDecisionFrequencyAnswer:
      'Configurable! Default is every 3-5 minutes. Too frequent = overtrading, too slow = missed opportunities.',

    faqCustomStrategy: 'Can I customize the trading strategy?',
    faqCustomStrategyAnswer:
      'Yes! You can adjust leverage settings, modify coin selection pool, change decision intervals, and customize system prompts (advanced).',

    faqMaxPositions: "What's the maximum number of concurrent positions?",
    faqMaxPositionsAnswer:
      'Default: 3 positions. This is a soft limit defined in the AI prompt, not hard-coded.',

    faqMarginInsufficient: 'Margin is insufficient error (code=-2019)',
    faqMarginInsufficientAnswer:
      'Common causes: Funds not transferred to futures account; Leverage set too high (default 20-50x); Existing positions using margin; Need to transfer USDT from spot to futures account first.',

    faqHighFees: 'Trading fees are too high',
    faqHighFeesAnswer:
      'NOFX default 3-minute scan interval can cause frequent trading. Solutions: Increase decision interval to 5-10 minutes; Optimize system prompt to reduce overtrading; Adjust leverage to reduce position sizes.',

    faqNoTakeProfit: "AI doesn't close profitable positions",
    faqNoTakeProfitAnswer:
      'AI may believe the trend will continue. The system lacks trailing stop-loss feature currently. You can manually close positions or adjust the system prompt to be more conservative with profit-taking.',

    // Technical Issues
    faqBinanceApiFailed: 'Binance API call failed (code=-2015)',
    faqBinanceApiFailedAnswer:
      'Error: "Invalid API-key, IP, or permissions for action". Solutions: Add server IP to Binance API whitelist; Check API permissions (needs Read + Futures Trading); Ensure using futures API not unified account API; VPN IP might be unstable.',

    faqBinancePositionMode: 'Binance Position Mode Error (code=-4061)',
    faqBinancePositionModeAnswer:
      'Error: "Order\'s position side does not match user\'s setting". Solution: Switch to Hedge Mode (双向持仓) in Binance Futures settings. You must close all positions first before switching.',

    faqPortInUse: "Backend won't start / Port already in use",
    faqPortInUseAnswer:
      'Check what\'s using port 8080 with "lsof -i :8080" and change the port in your .env file with NOFX_BACKEND_PORT=8081.',

    faqFrontendLoading: 'Frontend shows "Loading..." forever',
    faqFrontendLoadingAnswer:
      'Check if backend is running with "curl http://localhost:8080/api/health". Should return {"status":"ok"}. If not, check the troubleshooting guide.',

    faqDatabaseLocked: 'Database locked error',
    faqDatabaseLockedAnswer:
      'Stop all NOFX processes with "docker compose down" or "pkill nofx", then restart with "docker compose up -d".',

    faqAiLearningFailed: 'AI learning data failed to load',
    faqAiLearningFailedAnswer:
      'Causes: TA-Lib library not properly installed; Insufficient historical data (need completed trades); Environment configuration issues. Install TA-Lib: pip install TA-Lib or check system dependencies.',

    faqConfigNotEffective: 'Configuration changes not taking effect',
    faqConfigNotEffectiveAnswer:
      'For Docker: Need to rebuild with "docker compose down && docker compose up -d --build". For PM2: Restart with "pm2 restart all". Check configuration file format and path are correct.',

    // AI & Model Questions
    faqWhichModels: 'Which AI models are supported?',
    faqWhichModelsAnswer:
      'DeepSeek (recommended for cost/performance), Qwen (Alibaba Cloud), and Custom OpenAI-compatible APIs (can be used for OpenAI, Claude via proxy, or other providers).',

    faqApiCosts: 'How much do API calls cost?',
    faqApiCostsAnswer:
      'Depends on your model and decision frequency: DeepSeek: ~$0.10-0.50 per day (1 trader, 5min intervals); Qwen: ~$0.20-0.80 per day; Custom API (e.g., OpenAI GPT-4): ~$2-5 per day. Estimates based on typical usage.',

    faqMultipleModels: 'Can I use multiple AI models?',
    faqMultipleModelsAnswer:
      'Yes! Each trader can use a different AI model. You can even A/B test different models.',

    faqAiLearning: 'Does the AI learn from its mistakes?',
    faqAiLearningAnswer:
      'Yes, to some extent. NOFX provides historical performance feedback in each decision prompt, allowing the AI to adjust its strategy.',

    faqOnlyShort: 'AI only opens short positions, no long positions',
    faqOnlyShortAnswer:
      'The default system prompt contains "Don\'t have a long bias! Shorting is one of your core tools" which may cause this. Also affected by 4-hour timeframe data and model training bias. You can modify the system prompt to be more balanced.',

    faqModelSelection: 'Which DeepSeek version should I use?',
    faqModelSelectionAnswer:
      "DeepSeek V3 is recommended for best performance. Alternatives: DeepSeek R1 (reasoning model, slower but better logic), SiliconFlow's DeepSeek (alternative API provider). Most users report good results with V3.",

    // Data & Privacy
    faqDataStorage: 'Where is my data stored?',
    faqDataStorageAnswer:
      'All data is stored locally on your machine in SQLite databases: config.db (trader configurations), trading.db (trade history), and decision_logs/ (AI decision records).',

    faqApiKeySecurity: 'Is my API key secure?',
    faqApiKeySecurityAnswer:
      'API keys are stored in local databases. Never share your databases or .env files. We recommend using API keys with IP whitelist restrictions.',

    faqExportHistory: 'Can I export my trading history?',
    faqExportHistoryAnswer:
      'Yes! Trading data is in SQLite format. You can query it directly with: sqlite3 trading.db "SELECT * FROM trades;"',

    faqGetHelp: 'Where can I get help?',
    faqGetHelpAnswer:
      'Check GitHub Discussions, join our Telegram Community, or open an issue on GitHub.',

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
    },

    environmentSteps: {
      checkTitle: '1. Environment check',
      selectTitle: '2. Select exchange',
    },

    // Two-Stage Key Modal
    twoStageKey: {
      title: 'Two-Stage Private Key Input',
      stage1Description:
        'Enter the first {length} hex characters of your private key',
      stage2Description:
        'Enter the remaining {length} hex characters of your private key',
      stage1InputLabel: 'First Part',
      stage2InputLabel: 'Second Part',
      characters: 'characters',
      helpText: '💡 Enter {length} hex chars (without 0x) or {totalWithPrefix} chars (with 0x)',
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
    realtimeNav: '实时',
    configNav: '配置',
    dashboardNav: '看板',
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

    // AI Learning
    aiLearning: 'AI学习与反思',
    tradesAnalyzed: '已分析 {count} 笔交易 · 实时演化',
    latestReflection: '最新反思',
    fullCoT: '📋 完整思维链',
    totalTrades: '总交易数',
    winRate: '胜率',
    avgWin: '平均盈利',
    avgLoss: '平均亏损',
    profitFactor: '盈亏比',
    avgWinDivLoss: '平均盈利 ÷ 平均亏损',
    excellent: '🔥 优秀 - 盈利能力强',
    good: '✓ 良好 - 稳定盈利',
    fair: '⚠️ 一般 - 需要优化',
    poor: '❌ 较差 - 亏损超过盈利',
    bestPerformer: '最佳表现',
    worstPerformer: '最差表现',
    symbolPerformance: '📊 币种表现',
    tradeHistory: '历史成交',
    completedTrades: '最近 {count} 笔已完成交易',
    noCompletedTrades: '暂无完成的交易',
    completedTradesWillAppear: '已完成的交易将显示在这里',
    entry: '入场',
    exit: '出场',
    stopLoss: '止损',
    latest: '最新',

    // AI Learning Description
    howAILearns: '💡 AI如何学习和进化',
    aiLearningPoint1: '每次决策前分析最近20个交易周期',
    aiLearningPoint2: '识别表现最好和最差的币种',
    aiLearningPoint3: '根据胜率优化仓位大小',
    aiLearningPoint4: '避免重复过去的错误',

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
    scanIntervalRecommend: '建议: 3-10分钟',
    useTestnet: '使用测试网',
    enabled: '启用',
    save: '保存',

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
    enterPassphrase: '输入Passphrase (OKX必填)',
    hyperliquidPrivateKeyDesc: 'Hyperliquid 使用私钥进行交易认证',
    hyperliquidWalletAddressDesc: '与私钥对应的钱包地址',
    // Hyperliquid 代理钱包 (新安全模型)
    hyperliquidAgentWalletTitle: 'Hyperliquid 代理钱包配置',
    hyperliquidAgentWalletDesc:
      '使用代理钱包安全交易：代理钱包用于签名（餘額~0），主钱包持有资金（永不暴露私钥）',
    hyperliquidAgentPrivateKey: '代理私钥',
    enterHyperliquidAgentPrivateKey: '输入代理钱包私钥',
    hyperliquidAgentPrivateKeyDesc:
      '代理钱包私钥，用于签名交易（为了安全应保持余额接近0）',
    hyperliquidMainWalletAddress: '主钱包地址',
    enterHyperliquidMainWalletAddress: '输入主钱包地址',
    hyperliquidMainWalletAddressDesc:
      '持有交易资金的主钱包地址（永不暴露其私钥）',
    asterUserDesc:
      '主钱包地址 - 您用于登录 Aster 的 EVM 钱包地址（注意：仅支持 EVM 钱包，不支持 Solana 钱包）',
    asterSignerDesc:
      'API 钱包地址 - 从 https://www.asterdex.com/zh-CN/api-wallet 生成',
    asterPrivateKeyDesc:
      'API 钱包私钥 - 从 https://www.asterdex.com/zh-CN/api-wallet 获取（仅在本地用于签名，不会被传输）',
    asterUsdtWarning:
      '重要提示：Aster 仅统计 USDT 余额。请确保您使用 USDT 作为保证金币种，避免其他资产（BNB、ETH等）的价格波动导致盈亏统计错误',

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
      '留空 = 使用默认币种。必须以USDT结尾（如：BTCUSDT, ETHUSDT）',
    btcEthLeverageValidation: 'BTC/ETH杠杆必须在1-50倍之间',
    altcoinLeverageValidation: '山寨币杠杆必须在1-20倍之间',
    invalidSymbolFormat: '无效的币种格式：{symbol}，必须以USDT结尾',

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
    loadingError: '⚠️ 加载AI学习数据失败',
    noCompleteData: '暂无完整交易数据（需要完成开仓→平仓的完整周期）',

    // AI Traders Page - Additional
    inUse: '正在使用',
    noModelsConfigured: '暂无已配置的AI模型',
    noExchangesConfigured: '暂无已配置的交易所',
    signalSource: '信号源',
    signalSourceConfig: '信号源配置',
    coinPoolDescription: '用于获取币种池数据的API地址，留空则不使用此信号源',
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
    modelConfigInfo1: '• API Key将被加密存储，请确保密钥有效',
    modelConfigInfo2: '• Base URL用于自定义API服务器地址',
    modelConfigInfo3: '• 删除配置后，使用此模型的交易员将无法正常工作',
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
    otpPlaceholder: '000000',
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
    rememberMe: '记住我',
    resetPassword: '重置密码',
    resetPasswordTitle: '重置您的密码',
    newPassword: '新密码',
    newPasswordPlaceholder: '请输入新密码（至少6位）',
    resetPasswordButton: '重置密码',
    resetPasswordSuccess: '密码重置成功！请使用新密码登录',
    resetPasswordFailed: '密码重置失败',
    backToLogin: '返回登录',
    otpCode: 'OTP验证码',
    scanQRCode: '扫描二维码',
    enterOTPCode: '输入6位OTP验证码',
    verifyOTP: '验证OTP',
    setupTwoFactor: '设置双因素认证',
    setupTwoFactorDesc: '请按以下步骤设置Google验证器以保护您的账户安全',
    scanQRCodeInstructions: '使用Google Authenticator或Authy扫描此二维码',
    otpSecret: '或手动输入此密钥：',
    qrCodeHint: '二维码（如果无法扫描，请使用下方密钥）：',
    authStep1Title: '步骤1：下载Google Authenticator',
    authStep1Desc: '在手机应用商店下载并安装Google Authenticator应用',
    authStep2Title: '步骤2：添加账户',
    authStep2Desc: '在应用中点击“+”，选择“扫描二维码”或“手动输入密钥”',
    authStep3Title: '步骤3：验证设置',
    authStep3Desc: '设置完成后，点击下方按钮输入6位验证码',
    setupCompleteContinue: '我已完成设置，继续',
    copy: '复制',
    completeRegistration: '完成注册',
    completeRegistrationSubtitle: '以完成注册',
    loginSuccess: '登录成功',
    registrationSuccess: '注册成功',
    loginFailed: '登录失败，请检查您的邮箱和密码。',
    registrationFailed: '注册失败，请重试。',
    verificationFailed: 'OTP 验证失败，请检查验证码后重试。',
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
    registrationClosedMessage: '平台当前不开放新用户注册，如需访问请联系管理员获取账号。',

    // Hero Section
    githubStarsInDays: '3 天内 2.5K+ GitHub Stars',
    heroTitle1: 'Read the Market.',
    heroTitle2: 'Write the Trade.',
    heroDescription:
      'NOFX 是 AI 交易的未来标准——一个开放、社区驱动的代理式交易操作系统。支持 Binance、Aster DEX 等交易所，自托管、多代理竞争，让 AI 为你自动决策、执行和优化交易。',
    poweredBy: '由 Aster DEX 和 Binance 提供支持，Amber.ac 战略投资。',

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
    startupMessages3: 'Web 控制台 http://localhost:3000',

    // How It Works Section
    howToStart: '如何开始使用 NOFX',
    fourSimpleSteps: '四个简单步骤，开启 AI 自动交易之旅',
    step1Title: '拉取 GitHub 仓库',
    step1Desc:
      'git clone https://github.com/tinkle-community/nofx 并切换到 dev 分支测试新功能。',
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
    coinPoolApiNotConfigured: '币种池API未配置或无法访问（请检查信号源设置）',
    apiConnectionTimeout: 'API连接超时或返回数据为空',
    noCustomCoinsAndApiFailed: '未配置自定义币种且API获取失败',
    solutions: '解决方案：',
    setCustomCoinsInConfig: '在交易员配置中设置自定义币种列表',
    orConfigureCorrectApiUrl: '或者配置正确的币种池API地址',
    orDisableCoinPoolOptions: '或者禁用"使用币种池"和"使用OI Top"选项',
    signalSourceNotConfigured: '信号源未配置',
    signalSourceWarningMessage:
      '您有交易员启用了"使用币种池"或"使用OI Top"，但尚未配置信号源API地址。这将导致候选币种数量为0，交易员无法正常工作。',
    configureSignalSourceNow: '立即配置信号源',

    // FAQ Page
    faqTitle: '常见问题',
    faqSubtitle: '查找关于 NOFX 的常见问题解答',
    faqStillHaveQuestions: '还有其他问题？',
    faqContactUs: '加入我们的社区或查看 GitHub 获取更多帮助',

    // FAQ Categories
    faqCategoryBasics: '基础问题',
    faqCategoryContributing: '贡献与任务',
    faqCategorySetup: '安装与配置',
    faqCategoryTrading: '交易问题',
    faqCategoryTechnical: '技术问题',
    faqCategoryAI: 'AI与模型问题',
    faqCategoryData: '数据与隐私',

    // FAQ Questions & Answers - General
    faqWhatIsNOFX: 'NOFX 是什么？',
    faqWhatIsNOFXAnswer:
      'NOFX 是一个 AI 驱动的加密货币交易机器人，使用大语言模型（LLM）在期货市场进行交易决策。',

    faqSupportedExchanges: '支持哪些交易所？',
    faqSupportedExchangesAnswer:
      '支持币安合约（Binance Futures）、Hyperliquid 和 Aster DEX。更多交易所开发中。',

    faqIsProfitable: 'NOFX 能盈利吗？',
    faqIsProfitableAnswer:
      'AI 交易是实验性的，不保证盈利。请始终用小额资金测试，不要投入超过您承受能力的资金。',

    faqMultipleTraders: '可以同时运行多个交易员吗？',
    faqMultipleTradersAnswer:
      '可以！NOFX 支持运行多个交易员，每个可配置不同的 AI 模型和交易策略。',

    // Contributing & Community
    faqGithubProjectsTasks: '如何在 GitHub Projects 中领取任务？',
    faqGithubProjectsTasksAnswer:
      '路线图：https://github.com/orgs/NoFxAiOS/projects/3 ｜ 任务看板：https://github.com/orgs/NoFxAiOS/projects/5 ｜ 步骤：打开链接 → 按标签筛选（good first issue / help wanted / frontend / backend）→ 阅读描述与验收标准 → 评论“assign me”或自助分配 → Fork 仓库 → 同步你 fork 的 dev 与 upstream/dev → 从你 fork 的 dev 创建特性分支 → 推送到你的 fork → 打开 PR（base：NoFxAiOS/nofx:dev ← compare：你的用户名/nofx:feat/your-topic）→ 关联 Issue（Closes #123）并选择正确模板。',

    faqContributePR: '如何规范地提交 PR 并参与贡献？',
    faqContributePRAnswer:
      '规范：• 先 Fork；在你的 fork 的 dev 分支上创建特性分支（避免直接向上游 main 提交）• 分支命名：feat/...、fix/...、docs/...；提交信息遵循 Conventional Commits • PR 前运行检查：npm --prefix web run lint && npm --prefix web run build • 涉及 UI 变更请附截图/短视频 • 选择正确 PR 模板（frontend/backend/docs/general）• 从你的 fork 发起到 NoFxAiOS/nofx:dev，并在 PR 中关联 Issue（Closes #123）• 持续 rebase 到 upstream/dev，确保 CI 通过；尽量保持 PR 小而聚焦 • 参考 CONTRIBUTING.md 与 .github/PR_TITLE_GUIDE.md',

    // Setup & Configuration
    faqSystemRequirements: '系统要求是什么？',
    faqSystemRequirementsAnswer:
      '操作系统：Linux、macOS 或 Windows（推荐 Docker）；内存：最低 2GB，推荐 4GB；硬盘：应用 + 日志需要 1GB；网络：稳定的互联网连接。',

    faqNeedCoding: '需要编程经验吗？',
    faqNeedCodingAnswer:
      '不需要！NOFX 有 Web 界面进行所有配置。但基础的命令行知识有助于安装和故障排查。',

    faqGetApiKeys: '如何获取 API 密钥？',
    faqGetApiKeysAnswer:
      '币安：账户 → API 管理 → 创建 API → 启用合约。Hyperliquid：访问 Hyperliquid App → API 设置。Aster DEX：配置主钱包地址（User）、API 钱包地址（Signer）和私钥（Private Key）。',

    faqUseSubaccount: '应该使用子账户吗？',
    faqUseSubaccountAnswer:
      '推荐：是的，使用专门的子账户运行 NOFX 可以更好地隔离风险。但请注意，某些子账户有限制（例如币安子账户最高 5 倍杠杆）。',

    faqDockerDeployment: 'Docker 部署一直失败',
    faqDockerDeploymentAnswer:
      '常见问题：网络连接问题、依赖安装失败、内存不足（需要至少 2C2G）。如果卡在 "go build" 不动，尝试：docker compose down && docker compose build --no-cache && docker compose up -d',

    faqBalanceZero: '账户余额显示为 0',
    faqBalanceZeroAnswer:
      '资金可能在现货账户而非合约账户，或被理财功能锁定。您需要在币安手动将资金划转到合约账户。',

    faqTestnet: '可以使用测试网测试吗？',
    faqTestnetAnswer:
      '暂时不支持测试网。我们建议使用真实交易但小额资金（10-50 USDT）进行测试。',

    // Trading Questions
    faqNoTrades: '为什么我的交易员不开仓？',
    faqNoTradesAnswer:
      '常见原因：AI 根据市场情况决定"等待"；余额或保证金不足；达到持仓上限（默认最多 3 个仓位）；查看故障排查指南了解详细诊断。',

    faqDecisionFrequency: 'AI 多久做一次决策？',
    faqDecisionFrequencyAnswer:
      '可配置！默认是每 3-5 分钟。太频繁 = 过度交易，太慢 = 错过机会。',

    faqCustomStrategy: '可以自定义交易策略吗？',
    faqCustomStrategyAnswer:
      '可以！您可以调整杠杆设置、修改币种选择池、更改决策间隔、自定义系统提示词（高级）。',

    faqMaxPositions: '最多可以同时持有多少个仓位？',
    faqMaxPositionsAnswer:
      '默认：3 个仓位。这是 AI 提示词中的软限制，不是硬编码。',

    faqMarginInsufficient: '保证金不足错误 (code=-2019)',
    faqMarginInsufficientAnswer:
      '常见原因：资金未划转到合约账户；杠杆倍数设置过高（默认 20-50 倍）；已有持仓占用保证金；需要先从现货账户划转 USDT 到合约账户。',

    faqHighFees: '交易手续费太高',
    faqHighFeesAnswer:
      'NOFX 默认 3 分钟扫描间隔会导致频繁交易。解决方案：将决策间隔增加到 5-10 分钟；优化系统提示词减少过度交易；调整杠杆降低仓位大小。',

    faqNoTakeProfit: 'AI 不平掉盈利的仓位',
    faqNoTakeProfitAnswer:
      'AI 可能认为趋势会继续。系统目前缺少移动止盈功能。您可以手动平仓或调整系统提示词使其在获利时更保守。',

    // Technical Issues
    faqBinanceApiFailed: '币安 API 调用失败 (code=-2015)',
    faqBinanceApiFailedAnswer:
      '错误："Invalid API-key, IP, or permissions for action"。解决方案：将服务器 IP 添加到币安 API 白名单；检查 API 权限（需要读取 + 合约交易）；确保使用合约 API 而非统一账户 API；VPN IP 可能不稳定。',

    faqBinancePositionMode: '币安持仓模式错误 (code=-4061)',
    faqBinancePositionModeAnswer:
      '错误信息："Order\'s position side does not match user\'s setting"。解决方法：切换为双向持仓模式。登录币安合约 → 点击右上角偏好设置 → 选择持仓模式 → 双向持仓。注意：先平掉所有持仓。',

    faqPortInUse: '后端无法启动 / 端口被占用',
    faqPortInUseAnswer:
      '使用 "lsof -i :8080" 查看占用端口的进程，在 .env 中修改端口：NOFX_BACKEND_PORT=8081。',

    faqFrontendLoading: '前端一直显示"加载中..."',
    faqFrontendLoadingAnswer:
      '使用 "curl http://localhost:8080/api/health" 检查后端是否运行。应该返回 {"status":"ok"}。如果不是，查看故障排查指南。',

    faqDatabaseLocked: '数据库锁定错误',
    faqDatabaseLockedAnswer:
      '使用 "docker compose down" 或 "pkill nofx" 停止所有 NOFX 进程，然后使用 "docker compose up -d" 重启。',

    faqAiLearningFailed: 'AI 学习数据加载失败',
    faqAiLearningFailedAnswer:
      '原因：TA-Lib 库未正确安装；历史数据不足（需要完成交易）；环境配置问题。安装 TA-Lib：pip install TA-Lib 或检查系统依赖。',

    faqConfigNotEffective: '配置文件修改不生效',
    faqConfigNotEffectiveAnswer:
      'Docker 需要重新构建："docker compose down && docker compose up -d --build"。PM2 需要重启："pm2 restart all"。检查配置文件格式和路径是否正确。',

    // AI & Model Questions
    faqWhichModels: '支持哪些 AI 模型？',
    faqWhichModelsAnswer:
      'DeepSeek（推荐性价比）、Qwen（阿里云通义千问）、自定义 OpenAI 兼容 API（可用于 OpenAI、通过代理的 Claude 或其他提供商）。',

    faqApiCosts: 'API 调用成本是多少？',
    faqApiCostsAnswer:
      '取决于您的模型和决策频率：DeepSeek：每天约 $0.10-0.50（1 个交易员，5 分钟间隔）；Qwen：每天约 $0.20-0.80；自定义 API（例如 OpenAI GPT-4）：每天约 $2-5。基于典型使用的估算。',

    faqMultipleModels: '可以使用多个 AI 模型吗？',
    faqMultipleModelsAnswer:
      '可以！每个交易员可以使用不同的 AI 模型。您甚至可以 A/B 测试不同模型。',

    faqAiLearning: 'AI 会从错误中学习吗？',
    faqAiLearningAnswer:
      '会的，在一定程度上。NOFX 在每次决策提示中提供历史表现反馈，允许 AI 调整策略。',

    faqOnlyShort: 'AI 只开空单，不开多单',
    faqOnlyShortAnswer:
      '默认系统提示词包含"不要有做多偏见！做空是你的核心工具之一"，可能导致此问题。还受 4 小时周期数据和模型训练偏向性影响。您可以修改系统提示词使其更平衡。',

    faqModelSelection: '应该使用哪个 DeepSeek 版本？',
    faqModelSelectionAnswer:
      '推荐使用 DeepSeek V3 以获得最佳性能。备选：DeepSeek R1（推理模型，较慢但逻辑更好）、SiliconFlow 的 DeepSeek（备用 API 提供商）。大多数用户反馈 V3 效果良好。',

    // Data & Privacy
    faqDataStorage: '我的数据存储在哪里？',
    faqDataStorageAnswer:
      '所有数据都本地存储在您的机器上，使用 SQLite 数据库：config.db（交易员配置）、trading.db（交易历史）、decision_logs/（AI 决策记录）。',

    faqApiKeySecurity: 'API 密钥安全吗？',
    faqApiKeySecurityAnswer:
      'API 密钥存储在本地数据库中。永远不要分享您的数据库或 .env 文件。我们建议使用带 IP 白名单限制的 API 密钥。',

    faqExportHistory: '可以导出交易历史吗？',
    faqExportHistoryAnswer:
      '可以！交易数据是 SQLite 格式。您可以直接查询：sqlite3 trading.db "SELECT * FROM trades;"',

    faqGetHelp: '在哪里可以获得帮助？',
    faqGetHelpAnswer:
      '查看 GitHub Discussions、加入 Telegram 社区或在 GitHub 上提出 issue。',

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
      helpText: '💡 請輸入 {length} 位（不含 0x）或 {totalWithPrefix} 位（含 0x 前綴）',
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
