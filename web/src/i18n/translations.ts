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
    backtest: 'Backtest',
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
    debateNav: 'Arena',
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

    // Backtest Page
    backtestPage: {
      title: 'Backtest Lab',
      subtitle:
        'Pick a model + time range to replay the full AI decision loop.',
      start: 'Start Backtest',
      starting: 'Starting...',
      quickRanges: {
        h24: '24h',
        d3: '3d',
        d7: '7d',
      },
      actions: {
        pause: 'Pause',
        resume: 'Resume',
        stop: 'Stop',
      },
      states: {
        running: 'Running',
        paused: 'Paused',
        completed: 'Completed',
        failed: 'Failed',
        liquidated: 'Liquidated',
      },
      form: {
        aiModelLabel: 'AI Model',
        selectAiModel: 'Select AI model',
        providerLabel: 'Provider',
        statusLabel: 'Status',
        enabled: 'Enabled',
        disabled: 'Disabled',
        noModelWarning:
          'Please add and enable an AI model on the Model Config page first.',
        runIdLabel: 'Run ID',
        runIdPlaceholder: 'Leave blank to auto-generate',
        decisionTfLabel: 'Decision TF',
        cadenceLabel: 'Decision cadence (bars)',
        timeRangeLabel: 'Time range',
        symbolsLabel: 'Symbols (comma-separated)',
        customTfPlaceholder: 'Custom TFs (comma separated, e.g. 2h,6h)',
        initialBalanceLabel: 'Initial balance (USDT)',
        feeLabel: 'Fee (bps)',
        slippageLabel: 'Slippage (bps)',
        btcEthLeverageLabel: 'BTC/ETH leverage (x)',
        altcoinLeverageLabel: 'Altcoin leverage (x)',
        fillPolicies: {
          nextOpen: 'Next open',
          barVwap: 'Bar VWAP',
          midPrice: 'Mid price',
        },
        promptPresets: {
          baseline: 'Baseline',
          aggressive: 'Aggressive',
          conservative: 'Conservative',
          scalping: 'Scalping',
        },
        cacheAiLabel: 'Reuse AI cache',
        replayOnlyLabel: 'Replay only',
        overridePromptLabel: 'Use only custom prompt',
        customPromptLabel: 'Custom prompt (optional)',
        customPromptPlaceholder:
          'Append or fully customize the strategy prompt',
      },
      runList: {
        title: 'Runs',
        count: 'Total {count} records',
      },
      filters: {
        allStates: 'All states',
        searchPlaceholder: 'Run ID / label',
      },
      tableHeaders: {
        runId: 'Run ID',
        label: 'Label',
        state: 'State',
        progress: 'Progress',
        equity: 'Equity',
        lastError: 'Last Error',
        updated: 'Updated',
      },
      emptyStates: {
        noRuns: 'No runs yet',
        selectRun: 'Select a run to view details',
      },
      detail: {
        tfAndSymbols: 'TF: {tf} · Symbols {count}',
        labelPlaceholder: 'Label note',
        saveLabel: 'Save',
        deleteLabel: 'Delete',
        exportLabel: 'Export',
        errorLabel: 'Error',
      },
      toasts: {
        selectModel: 'Please select an AI model first.',
        modelDisabled: 'AI model {name} is disabled.',
        invalidRange: 'End time must be later than start time.',
        startSuccess: 'Backtest {id} started.',
        startFailed: 'Failed to start. Please try again later.',
        actionSuccess: '{action} {id} succeeded.',
        actionFailed: 'Operation failed. Please try again later.',
        labelSaved: 'Label updated.',
        labelFailed: 'Failed to update label.',
        confirmDelete: 'Delete backtest {id}? This action cannot be undone.',
        deleteSuccess: 'Backtest record deleted.',
        deleteFailed: 'Failed to delete. Please try again later.',
        traceFailed: 'Failed to fetch AI trace.',
        exportSuccess: 'Exported data for {id}.',
        exportFailed: 'Failed to export.',
      },
      aiTrace: {
        title: 'AI Trace',
        clear: 'Clear',
        cyclePlaceholder: 'Cycle',
        fetch: 'Fetch',
        prompt: 'Prompt',
        cot: 'Chain of thought',
        output: 'Output',
        cycleTag: 'Cycle #{cycle}',
      },
      decisionTrail: {
        title: 'AI Decision Trail',
        subtitle: 'Showing last {count} cycles',
        empty: 'No records yet',
        emptyHint:
          'The AI thought & execution log will appear once the run starts.',
      },
      charts: {
        equityTitle: 'Equity Curve',
        equityEmpty: 'No data yet',
      },
      metrics: {
        title: 'Metrics',
        totalReturn: 'Total Return %',
        maxDrawdown: 'Max Drawdown %',
        sharpe: 'Sharpe',
        profitFactor: 'Profit Factor',
        pending: 'Calculating...',
        realized: 'Realized PnL',
        unrealized: 'Unrealized PnL',
      },
      trades: {
        title: 'Trade Events',
        headers: {
          time: 'Time',
          symbol: 'Symbol',
          action: 'Action',
          qty: 'Qty',
          leverage: 'Leverage',
          pnl: 'PnL',
        },
        empty: 'No trades yet',
      },
      metadata: {
        title: 'Metadata',
        created: 'Created',
        updated: 'Updated',
        processedBars: 'Processed Bars',
        maxDrawdown: 'Max DD',
        liquidated: 'Liquidated',
        yes: 'Yes',
        no: 'No',
      },
    },

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
    faqTitle: 'Frequently Asked Questions',
    faqSubtitle: 'Find answers to common questions about NOFX',
    faqStillHaveQuestions: 'Still Have Questions?',
    faqContactUs: 'Join our community or check our GitHub for more help',

    // FAQ Categories
    faqCategoryGettingStarted: 'Getting Started',
    faqCategoryInstallation: 'Installation',
    faqCategoryConfiguration: 'Configuration',
    faqCategoryTrading: 'Trading',
    faqCategoryTechnicalIssues: 'Technical Issues',
    faqCategorySecurity: 'Security',
    faqCategoryFeatures: 'Features',
    faqCategoryAIModels: 'AI Models',
    faqCategoryContributing: 'Contributing',

    // ===== GETTING STARTED =====
    faqWhatIsNOFX: 'What is NOFX?',
    faqWhatIsNOFXAnswer:
      'NOFX is an open-source AI-powered trading operating system for cryptocurrency and US stock markets. It uses large language models (LLMs) like DeepSeek, GPT, Claude, Gemini to analyze market data and make autonomous trading decisions. Key features include: multi-AI model support, multi-exchange trading, visual strategy builder, backtesting, and AI debate arena for consensus decisions.',

    faqHowDoesItWork: 'How does NOFX work?',
    faqHowDoesItWorkAnswer:
      'NOFX works in 5 steps: 1) Configure AI models and exchange API credentials; 2) Create a trading strategy (coin selection, indicators, risk controls); 3) Create a "Trader" combining AI model + Exchange + Strategy; 4) Start the trader - it will analyze market data at regular intervals and make buy/sell/hold decisions; 5) Monitor performance on the dashboard. The AI uses Chain of Thought reasoning to explain each decision.',

    faqIsProfitable: 'Is NOFX profitable?',
    faqIsProfitableAnswer:
      'AI trading is experimental and NOT guaranteed to be profitable. Cryptocurrency futures are highly volatile and risky. NOFX is designed for educational and research purposes. We strongly recommend: starting with small amounts (10-50 USDT), never investing more than you can afford to lose, thoroughly testing with backtests before live trading, and understanding that past performance does not guarantee future results.',

    faqSupportedExchanges: 'Which exchanges are supported?',
    faqSupportedExchangesAnswer:
      'CEX (Centralized): Binance Futures, Bybit, OKX, Bitget. DEX (Decentralized): Hyperliquid, Aster DEX, Lighter. Each exchange has different features - Binance has the most liquidity, Hyperliquid is fully on-chain with no KYC required. Check the documentation for setup guides for each exchange.',

    faqSupportedAIModels: 'Which AI models are supported?',
    faqSupportedAIModelsAnswer:
      'NOFX supports 7+ AI models: DeepSeek (recommended for cost/performance), Alibaba Qwen, OpenAI (GPT-5.2), Anthropic Claude, Google Gemini, xAI Grok, and Kimi (Moonshot). You can also use any OpenAI-compatible API endpoint. Each model has different strengths - DeepSeek is cost-effective, OpenAI models are powerful but expensive, Claude excels at reasoning.',

    faqSystemRequirements: 'What are the system requirements?',
    faqSystemRequirementsAnswer:
      'Minimum: 2 CPU cores, 2GB RAM, 1GB disk space, stable internet. Recommended: 4GB RAM for running multiple traders. Supported OS: Linux, macOS, or Windows (via Docker or WSL2). Docker is the easiest installation method. For manual installation, you need Go 1.21+, Node.js 18+, and TA-Lib library.',

    // ===== INSTALLATION =====
    faqHowToInstall: 'How do I install NOFX?',
    faqHowToInstallAnswer:
      'Easiest method (Linux/macOS): Run "curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash" - this installs Docker containers automatically. Then open http://127.0.0.1:3000 in your browser. For manual installation or development, clone the repository and follow the README instructions.',

    faqWindowsInstallation: 'How do I install on Windows?',
    faqWindowsInstallationAnswer:
      'Three options: 1) Docker Desktop (Recommended) - Install Docker Desktop, then run "docker compose -f docker-compose.prod.yml up -d" in PowerShell; 2) WSL2 - Install Windows Subsystem for Linux, then follow Linux installation; 3) Docker in WSL2 - Best of both worlds, run the install script in WSL2 terminal. Access via http://127.0.0.1:3000',

    faqDockerDeployment: 'Docker deployment keeps failing',
    faqDockerDeploymentAnswer:
      'Common solutions: 1) Check Docker is running: "docker info"; 2) Ensure sufficient memory (2GB minimum); 3) If stuck on "go build", try: "docker compose down && docker compose build --no-cache && docker compose up -d"; 4) Check logs: "docker compose logs -f"; 5) For slow pulls, configure a Docker mirror in daemon.json.',

    faqManualInstallation: 'How do I install manually for development?',
    faqManualInstallationAnswer:
      'Prerequisites: Go 1.21+, Node.js 18+, TA-Lib. Steps: 1) Clone repo: "git clone https://github.com/NoFxAiOS/nofx.git"; 2) Install backend deps: "go mod download"; 3) Install frontend deps: "cd web && npm install"; 4) Build backend: "go build -o nofx"; 5) Run backend: "./nofx"; 6) Run frontend (new terminal): "cd web && npm run dev". Access at http://127.0.0.1:3000',

    faqServerDeployment: 'How do I deploy to a remote server?',
    faqServerDeploymentAnswer:
      'Run the install script on your server - it auto-detects the server IP. Access via http://YOUR_SERVER_IP:3000. For HTTPS: 1) Use Cloudflare (free) - add domain, create A record pointing to server IP, set SSL to "Flexible"; 2) Enable TRANSPORT_ENCRYPTION=true in .env for browser-side encryption; 3) Access via https://your-domain.com',

    faqUpdateNOFX: 'How do I update NOFX?',
    faqUpdateNOFXAnswer:
      'For Docker: Run "docker compose pull && docker compose up -d" to pull latest images and restart. For manual installation: "git pull && go build -o nofx" for backend, "cd web && npm install && npm run build" for frontend. Your configurations in data.db are preserved during updates.',

    // ===== CONFIGURATION =====
    faqConfigureAIModels: 'How do I configure AI models?',
    faqConfigureAIModelsAnswer:
      'Go to Config page → AI Models section. For each model: 1) Get API key from the provider (links provided in UI); 2) Enter API key; 3) Optionally customize base URL and model name; 4) Save. API keys are encrypted before storage. Test the connection after saving to verify it works.',

    faqConfigureExchanges: 'How do I configure exchange connections?',
    faqConfigureExchangesAnswer:
      'Go to Config page → Exchanges section. Click "Add Exchange", select exchange type, and enter credentials. For CEX (Binance/Bybit/OKX): Need API Key + Secret Key (+ Passphrase for OKX). For DEX (Hyperliquid/Aster/Lighter): Need wallet address and private key. Always enable only necessary permissions (Futures Trading) and consider IP whitelisting.',

    faqBinanceAPISetup: 'How do I set up Binance API correctly?',
    faqBinanceAPISetupAnswer:
      'Important steps: 1) Create API key in Binance → API Management; 2) Enable ONLY "Enable Futures" permission; 3) Consider adding IP whitelist for security; 4) CRITICAL: Switch to Hedge Mode (双向持仓) in Futures settings → Preferences → Position Mode; 5) Ensure funds are in Futures wallet (not Spot). Common error -4061 means you need Hedge Mode.',

    faqHyperliquidSetup: 'How do I set up Hyperliquid?',
    faqHyperliquidSetupAnswer:
      'Hyperliquid is a decentralized exchange requiring wallet authentication. Steps: 1) Go to app.hyperliquid.xyz; 2) Connect your wallet; 3) Generate an API wallet (recommended) or use your main wallet; 4) Copy the wallet address and private key; 5) In NOFX, add Hyperliquid exchange with these credentials. No KYC required, fully on-chain.',

    faqCreateStrategy: 'How do I create a trading strategy?',
    faqCreateStrategyAnswer:
      'Go to Strategy Studio: 1) Coin Source - select which coins to trade (static list, AI500 pool, or OI Top ranking); 2) Indicators - enable technical indicators (EMA, MACD, RSI, ATR, Volume, OI, Funding Rate); 3) Risk Controls - set leverage limits, max positions, margin usage cap, position size limits; 4) Custom Prompt (optional) - add specific instructions for the AI. Save and assign to a trader.',

    faqCreateTrader: 'How do I create and start a trader?',
    faqCreateTraderAnswer:
      'Go to Traders page: 1) Click "Create Trader"; 2) Select AI Model (must be configured first); 3) Select Exchange (must be configured first); 4) Select Strategy (or use default); 5) Set decision interval (e.g., 5 minutes); 6) Save, then click "Start" to begin trading. Monitor performance on Dashboard page.',

    // ===== TRADING =====
    faqHowAIDecides: 'How does the AI make trading decisions?',
    faqHowAIDecidesAnswer:
      'The AI uses Chain of Thought (CoT) reasoning in 4 steps: 1) Position Analysis - reviews current holdings and P/L; 2) Risk Assessment - checks account margin, available balance; 3) Opportunity Evaluation - analyzes market data, indicators, candidate coins; 4) Final Decision - outputs specific action (buy/sell/hold) with reasoning. You can view the full reasoning in decision logs.',

    faqDecisionFrequency: 'How often does the AI make decisions?',
    faqDecisionFrequencyAnswer:
      'Configurable per trader, default is 3-5 minutes. Considerations: Too frequent (1-2 min) = overtrading, high fees; Too slow (30+ min) = missed opportunities. Recommended: 5 minutes for active trading, 15-30 minutes for swing trading. The AI may decide to "hold" (no action) in many cycles.',

    faqNoTradesExecuting: "Why isn't my trader executing any trades?",
    faqNoTradesExecutingAnswer:
      'Common causes: 1) AI decided to wait (check decision logs for reasoning); 2) Insufficient balance in futures account; 3) Max positions limit reached (default: 3); 4) Exchange API issues (check error messages); 5) Strategy constraints too restrictive. Check Dashboard → Decision Logs for detailed AI reasoning each cycle.',

    faqOnlyShortPositions: 'Why is the AI only opening short positions?',
    faqOnlyShortPositionsAnswer:
      'This is usually due to Binance Position Mode. Solution: Switch to Hedge Mode (双向持仓) in Binance Futures → Preferences → Position Mode. You must close all positions first. After switching, the AI can open both long and short positions independently.',

    faqLeverageSettings: 'How do leverage settings work?',
    faqLeverageSettingsAnswer:
      'Leverage is set in Strategy → Risk Controls: BTC/ETH leverage (typically 5-20x) and Altcoin leverage (typically 3-10x). Higher leverage = higher risk and potential returns. Subaccounts may have restrictions (e.g., Binance subaccounts limited to 5x). The AI respects these limits when placing orders.',

    faqStopLossTakeProfit: 'Does NOFX support stop-loss and take-profit?',
    faqStopLossTakeProfitAnswer:
      'The AI can suggest stop-loss/take-profit levels in its decisions, but these are guidance-based rather than hard-coded exchange orders. The AI monitors positions each cycle and may decide to close based on P/L. For guaranteed stop-loss, you can set exchange-level orders manually or adjust the strategy prompt to be more conservative.',

    faqMultipleTraders: 'Can I run multiple traders?',
    faqMultipleTradersAnswer:
      'Yes! NOFX supports running 20+ concurrent traders. Each trader can have different: AI model, exchange account, strategy, decision interval. Use this to A/B test strategies, compare AI models, or diversify across exchanges. Monitor all traders on the Competition page.',

    faqAICosts: 'How much do AI API calls cost?',
    faqAICostsAnswer:
      'Approximate daily costs per trader (5-min intervals): DeepSeek: $0.10-0.50; Qwen: $0.20-0.80; OpenAI: $2-5; Claude: $1-3. Costs depend on prompt length and response tokens. DeepSeek offers the best cost/performance ratio. Longer decision intervals reduce costs.',

    // ===== TECHNICAL ISSUES =====
    faqPortInUse: 'Port 8080 or 3000 already in use',
    faqPortInUseAnswer:
      'Check what\'s using the port: "lsof -i :8080" (macOS/Linux) or "netstat -ano | findstr 8080" (Windows). Kill the process or change ports in .env: NOFX_BACKEND_PORT=8081, NOFX_FRONTEND_PORT=3001. Restart with "docker compose down && docker compose up -d".',

    faqFrontendNotLoading: 'Frontend shows "Loading..." forever',
    faqFrontendNotLoadingAnswer:
      'Backend may not be running or reachable. Check: 1) "curl http://127.0.0.1:8080/api/health" should return {"status":"ok"}; 2) "docker compose ps" to verify containers are running; 3) Check backend logs: "docker compose logs nofx-backend"; 4) Ensure firewall allows port 8080.',

    faqDatabaseLocked: 'Database locked error',
    faqDatabaseLockedAnswer:
      'Multiple processes accessing SQLite simultaneously. Solution: 1) Stop all processes: "docker compose down" or "pkill nofx"; 2) Remove lock files if present: "rm -f data/data.db-wal data/data.db-shm"; 3) Restart: "docker compose up -d". Only one backend instance should access the database.',

    faqTALibNotFound: 'TA-Lib not found during build',
    faqTALibNotFoundAnswer:
      'TA-Lib is required for technical indicators. Install: macOS: "brew install ta-lib"; Ubuntu/Debian: "sudo apt-get install libta-lib0-dev"; CentOS: "yum install ta-lib-devel". After installing, rebuild: "go build -o nofx". Docker images include TA-Lib pre-installed.',

    faqAIAPITimeout: 'AI API timeout or connection refused',
    faqAIAPITimeoutAnswer:
      'Check: 1) API key is valid (test with curl); 2) Network can reach API endpoint (ping/curl); 3) API provider is not down (check status page); 4) VPN/firewall not blocking; 5) Rate limits not exceeded. Default timeout is 120 seconds.',

    faqBinancePositionMode: 'Binance error code -4061 (Position Mode)',
    faqBinancePositionModeAnswer:
      'Error: "Order\'s position side does not match user\'s setting". You\'re in One-way Mode but NOFX requires Hedge Mode. Fix: 1) Close ALL positions first; 2) Binance Futures → Settings (gear icon) → Preferences → Position Mode → Switch to "Hedge Mode" (双向持仓); 3) Restart your trader.',

    faqBalanceShowsZero: 'Account balance shows 0',
    faqBalanceShowsZeroAnswer:
      'Funds are likely in Spot wallet, not Futures wallet. Solution: 1) In Binance, go to Wallet → Futures → Transfer; 2) Transfer USDT from Spot to Futures; 3) Refresh NOFX dashboard. Also check: funds not locked in savings/staking products.',

    faqDockerPullFailed: 'Docker image pull failed or slow',
    faqDockerPullFailedAnswer:
      'Docker Hub can be slow in some regions. Solutions: 1) Configure a Docker mirror in /etc/docker/daemon.json: {"registry-mirrors": ["https://mirror.gcr.io"]}; 2) Restart Docker; 3) Retry pull. Alternatively, use GitHub Container Registry (ghcr.io) which may have better connectivity in your region.',

    // ===== SECURITY =====
    faqAPIKeyStorage: 'How are API keys stored?',
    faqAPIKeyStorageAnswer:
      'API keys are encrypted using AES-256-GCM before storage in the local SQLite database. The encryption key (DATA_ENCRYPTION_KEY) is stored in your .env file. Keys are decrypted only in memory when needed for API calls. Never share your data.db or .env files.',

    faqEncryptionDetails: 'What encryption does NOFX use?',
    faqEncryptionDetailsAnswer:
      'NOFX uses multiple encryption layers: 1) AES-256-GCM for database storage (API keys, secrets); 2) RSA-2048 for optional transport encryption (browser to server); 3) JWT for authentication tokens. Keys are generated during installation. Enable TRANSPORT_ENCRYPTION=true for HTTPS environments.',

    faqSecurityBestPractices: 'What are security best practices?',
    faqSecurityBestPracticesAnswer:
      'Recommended: 1) Use exchange API keys with IP whitelist and minimal permissions (Futures Trading only); 2) Use dedicated subaccount for NOFX; 3) Enable TRANSPORT_ENCRYPTION for remote deployments; 4) Never share .env or data.db files; 5) Use HTTPS with valid certificates; 6) Regularly rotate API keys; 7) Monitor account activity.',

    faqCanNOFXStealFunds: 'Can NOFX steal my funds?',
    faqCanNOFXStealFundsAnswer:
      'NOFX is open-source (AGPL-3.0 license) - you can audit all code on GitHub. API keys are stored locally on YOUR machine, never sent to external servers. NOFX only has the permissions you grant via API keys. For maximum safety: use API keys with trading-only permissions (no withdrawal), enable IP whitelist, use a dedicated subaccount.',

    // ===== FEATURES =====
    faqStrategyStudio: 'What is Strategy Studio?',
    faqStrategyStudioAnswer:
      'Strategy Studio is a visual strategy builder where you configure: 1) Coin Sources - which cryptocurrencies to trade (static list, AI500 top coins, OI ranking); 2) Technical Indicators - EMA, MACD, RSI, ATR, Volume, Open Interest, Funding Rate; 3) Risk Controls - leverage limits, position sizing, margin caps; 4) Custom Prompts - specific instructions for AI. No coding required.',

    faqBacktestLab: 'What is Backtest Lab?',
    faqBacktestLabAnswer:
      'Backtest Lab tests your strategy against historical data without risking real funds. Features: 1) Configure AI model, date range, initial balance; 2) Watch real-time progress with equity curve; 3) View metrics: Return %, Max Drawdown, Sharpe Ratio, Win Rate; 4) Analyze individual trades and AI reasoning. Essential for validating strategies before live trading.',

    faqDebateArena: 'What is Debate Arena?',
    faqDebateArenaAnswer:
      'Debate Arena lets multiple AI models debate trading decisions before execution. Setup: 1) Choose 2-5 AI models; 2) Assign personalities (Bull, Bear, Analyst, Contrarian, Risk Manager); 3) Watch them debate in rounds; 4) Final decision based on consensus voting. Useful for high-conviction trades where you want multiple perspectives.',

    faqCompetitionMode: 'What is Competition Mode?',
    faqCompetitionModeAnswer:
      'Competition page shows a real-time leaderboard of all your traders. Compare: ROI, P&L, Sharpe ratio, win rate, number of trades. Use this to A/B test different AI models, strategies, or configurations. Traders can be marked as "Show in Competition" to appear on the leaderboard.',

    faqChainOfThought: 'What is Chain of Thought (CoT)?',
    faqChainOfThoughtAnswer:
      "Chain of Thought is the AI's reasoning process, visible in decision logs. The AI explains its thinking in 4 steps: 1) Current position analysis; 2) Account risk assessment; 3) Market opportunity evaluation; 4) Final decision rationale. This transparency helps you understand WHY the AI made each decision, useful for improving strategies.",

    // ===== AI MODELS =====
    faqWhichAIModelBest: 'Which AI model should I use?',
    faqWhichAIModelBestAnswer:
      'Recommended: DeepSeek for best cost/performance ratio ($0.10-0.50/day). Alternatives: OpenAI for best reasoning but expensive ($2-5/day); Claude for nuanced analysis; Qwen for competitive pricing. You can run multiple traders with different models to compare. Check the Competition page to see which performs best for your strategy.',

    faqCustomAIAPI: 'Can I use a custom AI API?',
    faqCustomAIAPIAnswer:
      'Yes! NOFX supports any OpenAI-compatible API. In Config → AI Models → Custom API: 1) Enter your API endpoint URL (e.g., https://your-api.com/v1); 2) Enter API key; 3) Specify model name. This works with self-hosted models, alternative providers, or Claude via third-party proxies.',

    faqAIHallucinations: 'What about AI hallucinations?',
    faqAIHallucinationsAnswer:
      'AI models can sometimes produce incorrect or fabricated information ("hallucinations"). NOFX mitigates this by: 1) Providing structured prompts with real market data; 2) Enforcing JSON output format for decisions; 3) Validating orders before execution. However, AI trading is experimental - always monitor decisions and don\'t rely solely on AI judgment.',

    faqCompareAIModels: 'How do I compare different AI models?',
    faqCompareAIModelsAnswer:
      'Create multiple traders with different AI models but same strategy/exchange. Run them simultaneously and compare on Competition page. Metrics to watch: ROI, win rate, Sharpe ratio, max drawdown. Alternatively, use Backtest Lab to test models against same historical data. The Debate Arena also shows how different models reason about the same situation.',

    // ===== CONTRIBUTING =====
    faqHowToContribute: 'How can I contribute to NOFX?',
    faqHowToContributeAnswer:
      'NOFX is open-source and welcomes contributions! Ways to contribute: 1) Code - fix bugs, add features (check GitHub Issues); 2) Documentation - improve guides, translate; 3) Bug Reports - report issues with details; 4) Feature Ideas - suggest improvements. Start with issues labeled "good first issue". All contributors may receive airdrop rewards.',

    faqPRGuidelines: 'What are the PR guidelines?',
    faqPRGuidelinesAnswer:
      'PR Process: 1) Fork repo to your account; 2) Create feature branch from dev: "git checkout -b feat/your-feature"; 3) Make changes, run lint: "npm --prefix web run lint"; 4) Commit with Conventional Commits format; 5) Push and create PR to NoFxAiOS/nofx:dev; 6) Reference related issue (Closes #123); 7) Wait for review. Keep PRs small and focused.',

    faqBountyProgram: 'Is there a bounty program?',
    faqBountyProgramAnswer:
      'Yes! Contributors receive airdrop rewards based on contributions: Code commits (highest weight), bug fixes, feature suggestions, documentation. Issues with "bounty" label have cash rewards. After completing work, submit a Bounty Claim. Check CONTRIBUTING.md for details on the reward structure.',

    faqReportBugs: 'How do I report bugs?',
    faqReportBugsAnswer:
      'For bugs: Open a GitHub Issue with: 1) Clear description of the problem; 2) Steps to reproduce; 3) Expected vs actual behavior; 4) System info (OS, Docker version, browser); 5) Relevant logs. For SECURITY vulnerabilities: Do NOT open public issues - DM @Web3Tinkle on Twitter instead.',

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

    // Debate Arena Page
    debatePage: {
      title: 'Market Debate Arena',
      subtitle: 'Watch AI models debate market conditions and reach consensus',
      newDebate: 'New Debate',
      noDebates: 'No debates yet',
      createFirst: 'Create your first debate to get started',
      selectDebate: 'Select a debate to view details',
      createDebate: 'Create Debate',
      creating: 'Creating...',
      debateName: 'Debate Name',
      debateNamePlaceholder: 'e.g., BTC Bull or Bear?',
      tradingPair: 'Trading Pair',
      strategy: 'Strategy',
      selectStrategy: 'Select a strategy',
      maxRounds: 'Max Rounds',
      autoExecute: 'Auto Execute',
      autoExecuteHint: 'Automatically execute the consensus trade',
      participants: 'Participants',
      addParticipant: 'Add AI Participant',
      noModels: 'No AI models available',
      atLeast2: 'Add at least 2 participants',
      personalities: {
        bull: 'Aggressive Bull',
        bear: 'Cautious Bear',
        analyst: 'Data Analyst',
        contrarian: 'Contrarian',
        risk_manager: 'Risk Manager',
      },
      status: {
        pending: 'Pending',
        running: 'Running',
        voting: 'Voting',
        completed: 'Completed',
        cancelled: 'Cancelled',
      },
      actions: {
        start: 'Start Debate',
        starting: 'Starting...',
        cancel: 'Cancel',
        delete: 'Delete',
        execute: 'Execute Trade',
      },
      round: 'Round',
      roundOf: 'Round {current} of {max}',
      messages: 'Messages',
      noMessages: 'No messages yet',
      waitingStart: 'Waiting for debate to start...',
      votes: 'Votes',
      consensus: 'Consensus',
      finalDecision: 'Final Decision',
      confidence: 'Confidence',
      votesCount: '{count} votes',
      decision: {
        open_long: 'Open Long',
        open_short: 'Open Short',
        close_long: 'Close Long',
        close_short: 'Close Short',
        hold: 'Hold',
        wait: 'Wait',
      },
      messageTypes: {
        analysis: 'Analysis',
        rebuttal: 'Rebuttal',
        vote: 'Vote',
        summary: 'Summary',
      },
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
    backtest: '回测',
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
    debateNav: '竞技场',
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

    // Backtest Page
    backtestPage: {
      title: '回测实验室',
      subtitle: '选择模型与时间范围，快速复盘 AI 决策链路。',
      start: '启动回测',
      starting: '启动中...',
      quickRanges: {
        h24: '24小时',
        d3: '3天',
        d7: '7天',
      },
      actions: {
        pause: '暂停',
        resume: '恢复',
        stop: '停止',
      },
      states: {
        running: '运行中',
        paused: '已暂停',
        completed: '已完成',
        failed: '失败',
        liquidated: '已爆仓',
      },
      form: {
        aiModelLabel: 'AI 模型',
        selectAiModel: '选择AI模型',
        providerLabel: 'Provider',
        statusLabel: '状态',
        enabled: '已启用',
        disabled: '未启用',
        noModelWarning: '请先在「模型配置」页面添加并启用AI模型。',
        runIdLabel: 'Run ID',
        runIdPlaceholder: '留空则自动生成',
        decisionTfLabel: '决策周期',
        cadenceLabel: '决策节奏（根数）',
        timeRangeLabel: '时间范围',
        symbolsLabel: '交易标的（逗号分隔）',
        customTfPlaceholder: '自定义周期（逗号分隔，例如 2h,6h）',
        initialBalanceLabel: '初始资金 (USDT)',
        feeLabel: '手续费 (bps)',
        slippageLabel: '滑点 (bps)',
        btcEthLeverageLabel: 'BTC/ETH 杠杆 (倍)',
        altcoinLeverageLabel: '山寨币杠杆 (倍)',
        fillPolicies: {
          nextOpen: '下一根开盘价',
          barVwap: 'K线 VWAP',
          midPrice: '中间价',
        },
        promptPresets: {
          baseline: '基础版',
          aggressive: '激进版',
          conservative: '稳健版',
          scalping: '剥头皮',
        },
        cacheAiLabel: '复用AI缓存',
        replayOnlyLabel: '仅回放记录',
        overridePromptLabel: '仅使用自定义提示词',
        customPromptLabel: '自定义提示词（可选）',
        customPromptPlaceholder: '追加或完全自定义策略提示词',
      },
      runList: {
        title: '运行列表',
        count: '共 {count} 条记录',
      },
      filters: {
        allStates: '全部状态',
        searchPlaceholder: 'Run ID / 标签',
      },
      tableHeaders: {
        runId: 'Run ID',
        label: '标签',
        state: '状态',
        progress: '进度',
        equity: '净值',
        lastError: '最后错误',
        updated: '更新时间',
      },
      emptyStates: {
        noRuns: '暂无记录',
        selectRun: '请选择一个运行查看详情',
      },
      detail: {
        tfAndSymbols: '周期: {tf} · 币种 {count}',
        labelPlaceholder: '备注标签',
        saveLabel: '保存',
        deleteLabel: '删除',
        exportLabel: '导出',
        errorLabel: '错误',
      },
      toasts: {
        selectModel: '请先选择一个AI模型。',
        modelDisabled: 'AI模型 {name} 尚未启用。',
        invalidRange: '结束时间必须晚于开始时间。',
        startSuccess: '回测 {id} 已启动。',
        startFailed: '启动失败，请稍后再试。',
        actionSuccess: '{action} {id} 成功。',
        actionFailed: '操作失败，请稍后再试。',
        labelSaved: '标签已更新。',
        labelFailed: '更新标签失败。',
        confirmDelete: '确认删除回测 {id} 吗？该操作不可恢复。',
        deleteSuccess: '回测记录已删除。',
        deleteFailed: '删除失败，请稍后再试。',
        traceFailed: '获取AI思维链失败。',
        exportSuccess: '已导出 {id} 的数据。',
        exportFailed: '导出失败。',
      },
      aiTrace: {
        title: 'AI 思维链',
        clear: '清除',
        cyclePlaceholder: '循环编号',
        fetch: '获取',
        prompt: '提示词',
        cot: '思考链',
        output: '输出',
        cycleTag: '周期 #{cycle}',
      },
      decisionTrail: {
        title: 'AI 决策轨迹',
        subtitle: '展示最近 {count} 次循环',
        empty: '暂无记录',
        emptyHint: '回测运行后将自动记录每次 AI 思考与执行',
      },
      charts: {
        equityTitle: '净值曲线',
        equityEmpty: '暂无数据',
      },
      metrics: {
        title: '指标',
        totalReturn: '总收益率 %',
        maxDrawdown: '最大回撤 %',
        sharpe: '夏普比率',
        profitFactor: '盈亏因子',
        pending: '计算中...',
        realized: '已实现盈亏',
        unrealized: '未实现盈亏',
      },
      trades: {
        title: '交易事件',
        headers: {
          time: '时间',
          symbol: '币种',
          action: '操作',
          qty: '数量',
          leverage: '杠杆',
          pnl: '盈亏',
        },
        empty: '暂无交易',
      },
      metadata: {
        title: '元信息',
        created: '创建时间',
        updated: '更新时间',
        processedBars: '已处理K线',
        maxDrawdown: '最大回撤',
        liquidated: '是否爆仓',
        yes: '是',
        no: '否',
      },
    },

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

    // AI Traders Page - Additional
    inUse: '正在使用',
    noModelsConfigured: '暂无已配置的AI模型',
    noExchangesConfigured: '暂无已配置的交易所',
    signalSource: '信号源',
    signalSourceConfig: '信号源配置',
    ai500Description:
      '用于获取 AI500 数据源的 API 地址，留空则不使用此数据源',
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
    faqTitle: '常见问题',
    faqSubtitle: '查找关于 NOFX 的常见问题解答',
    faqStillHaveQuestions: '还有其他问题？',
    faqContactUs: '加入我们的社区或查看 GitHub 获取更多帮助',

    // FAQ Categories
    faqCategoryGettingStarted: '入门指南',
    faqCategoryInstallation: '安装部署',
    faqCategoryConfiguration: '配置设置',
    faqCategoryTrading: '交易相关',
    faqCategoryTechnicalIssues: '技术问题',
    faqCategorySecurity: '安全相关',
    faqCategoryFeatures: '功能介绍',
    faqCategoryAIModels: 'AI 模型',
    faqCategoryContributing: '参与贡献',

    // ===== 入门指南 =====
    faqWhatIsNOFX: 'NOFX 是什么？',
    faqWhatIsNOFXAnswer:
      'NOFX 是一个开源的 AI 驱动交易操作系统，支持加密货币和美股市场。它使用大语言模型（LLM）如 DeepSeek、GPT、Claude、Gemini 来分析市场数据，进行自主交易决策。核心功能包括：多 AI 模型支持、多交易所交易、可视化策略构建器、回测系统、以及用于共识决策的 AI 辩论竞技场。',

    faqHowDoesItWork: 'NOFX 是如何工作的？',
    faqHowDoesItWorkAnswer:
      'NOFX 分 5 步工作：1）配置 AI 模型和交易所 API 凭证；2）创建交易策略（币种选择、指标、风控）；3）创建"交易员"，组合 AI 模型 + 交易所 + 策略；4）启动交易员 - 它会定期分析市场数据并做出买入/卖出/持有决策；5）在仪表板上监控表现。AI 使用思维链（Chain of Thought）推理来解释每个决策。',

    faqIsProfitable: 'NOFX 能盈利吗？',
    faqIsProfitableAnswer:
      'AI 交易是实验性的，不保证盈利。加密货币期货波动性大、风险高。NOFX 仅用于教育和研究目的。我们强烈建议：从小额开始（10-50 USDT），不要投入超过承受能力的资金，在实盘交易前充分回测，并理解过去的表现不代表未来的结果。',

    faqSupportedExchanges: '支持哪些交易所？',
    faqSupportedExchangesAnswer:
      'CEX（中心化）：币安合约、Bybit、OKX、Bitget。DEX（去中心化）：Hyperliquid、Aster DEX、Lighter。每个交易所有不同特点 - 币安流动性最好，Hyperliquid 完全链上无需 KYC。查看文档获取各交易所的设置指南。',

    faqSupportedAIModels: '支持哪些 AI 模型？',
    faqSupportedAIModelsAnswer:
      'NOFX 支持 7+ 种 AI 模型：DeepSeek（推荐性价比）、阿里云通义千问、OpenAI（GPT-5.2）、Anthropic Claude、Google Gemini、xAI Grok、Kimi（月之暗面）。您也可以使用任何 OpenAI 兼容的 API 端点。每个模型各有优势 - DeepSeek 性价比高，OpenAI 能力强但贵，Claude 擅长推理。',

    faqSystemRequirements: '系统要求是什么？',
    faqSystemRequirementsAnswer:
      '最低配置：2 核 CPU，2GB 内存，1GB 硬盘，稳定网络。推荐：4GB 内存用于运行多个交易员。支持系统：Linux、macOS 或 Windows（通过 Docker 或 WSL2）。Docker 是最简单的安装方式。手动安装需要 Go 1.21+、Node.js 18+ 和 TA-Lib 库。',

    // ===== 安装部署 =====
    faqHowToInstall: '如何安装 NOFX？',
    faqHowToInstallAnswer:
      '最简单的方法（Linux/macOS）：运行 "curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash" - 这会自动安装 Docker 容器。然后在浏览器中打开 http://127.0.0.1:3000。手动安装或开发请克隆仓库并按照 README 说明操作。',

    faqWindowsInstallation: 'Windows 如何安装？',
    faqWindowsInstallationAnswer:
      '三种方式：1）Docker Desktop（推荐）- 安装 Docker Desktop，然后在 PowerShell 中运行 "docker compose -f docker-compose.prod.yml up -d"；2）WSL2 - 安装 Windows 子系统 Linux，然后按 Linux 方式安装；3）WSL2 + Docker - 两全其美，在 WSL2 终端运行安装脚本。通过 http://127.0.0.1:3000 访问。',

    faqDockerDeployment: 'Docker 部署一直失败',
    faqDockerDeploymentAnswer:
      '常见解决方案：1）检查 Docker 是否运行："docker info"；2）确保足够内存（最少 2GB）；3）如果卡在 "go build"，尝试："docker compose down && docker compose build --no-cache && docker compose up -d"；4）查看日志："docker compose logs -f"；5）拉取较慢时，在 daemon.json 配置 Docker 镜像。',

    faqManualInstallation: '如何手动安装用于开发？',
    faqManualInstallationAnswer:
      '前置条件：Go 1.21+、Node.js 18+、TA-Lib。步骤：1）克隆仓库："git clone https://github.com/NoFxAiOS/nofx.git"；2）安装后端依赖："go mod download"；3）安装前端依赖："cd web && npm install"；4）构建后端："go build -o nofx"；5）运行后端："./nofx"；6）运行前端（新终端）："cd web && npm run dev"。访问 http://127.0.0.1:3000',

    faqServerDeployment: '如何部署到远程服务器？',
    faqServerDeploymentAnswer:
      '在服务器上运行安装脚本 - 它会自动检测服务器 IP。通过 http://服务器IP:3000 访问。配置 HTTPS：1）使用 Cloudflare（免费）- 添加域名，创建 A 记录指向服务器 IP，SSL 设为"灵活"；2）在 .env 中启用 TRANSPORT_ENCRYPTION=true 进行浏览器端加密；3）通过 https://你的域名.com 访问。',

    faqUpdateNOFX: '如何更新 NOFX？',
    faqUpdateNOFXAnswer:
      'Docker 方式：运行 "docker compose pull && docker compose up -d" 拉取最新镜像并重启。手动安装：后端 "git pull && go build -o nofx"，前端 "cd web && npm install && npm run build"。data.db 中的配置在更新时会保留。',

    // ===== 配置设置 =====
    faqConfigureAIModels: '如何配置 AI 模型？',
    faqConfigureAIModelsAnswer:
      '进入配置页面 → AI 模型部分。对于每个模型：1）从提供商获取 API 密钥（界面提供链接）；2）输入 API 密钥；3）可选自定义基础 URL 和模型名称；4）保存。API 密钥在存储前会加密。保存后测试连接以验证。',

    faqConfigureExchanges: '如何配置交易所连接？',
    faqConfigureExchangesAnswer:
      '进入配置页面 → 交易所部分。点击"添加交易所"，选择类型并输入凭证。CEX（币安/Bybit/OKX）：需要 API Key + Secret Key（OKX 还需要 Passphrase）。DEX（Hyperliquid/Aster/Lighter）：需要钱包地址和私钥。务必只启用必要权限（合约交易）并考虑 IP 白名单。',

    faqBinanceAPISetup: '如何正确设置币安 API？',
    faqBinanceAPISetupAnswer:
      '重要步骤：1）在币安 → API 管理中创建 API 密钥；2）仅启用"启用合约"权限；3）考虑添加 IP 白名单增强安全；4）关键：在合约设置 → 偏好设置 → 持仓模式中切换为双向持仓模式；5）确保资金在合约钱包（不是现货）。-4061 错误表示需要双向持仓模式。',

    faqHyperliquidSetup: '如何设置 Hyperliquid？',
    faqHyperliquidSetupAnswer:
      'Hyperliquid 是去中心化交易所，需要钱包认证。步骤：1）访问 app.hyperliquid.xyz；2）连接钱包；3）生成 API 钱包（推荐）或使用主钱包；4）复制钱包地址和私钥；5）在 NOFX 中添加 Hyperliquid 交易所并填入凭证。无需 KYC，完全链上。',

    faqCreateStrategy: '如何创建交易策略？',
    faqCreateStrategyAnswer:
      '进入策略工作室：1）币种来源 - 选择交易哪些币（静态列表、AI500 池或 OI 排行）；2）指标 - 启用技术指标（EMA、MACD、RSI、ATR、成交量、OI、资金费率）；3）风控 - 设置杠杆限制、最大持仓数、保证金使用上限、仓位大小限制；4）自定义提示词（可选）- 为 AI 添加特定指令。保存后分配给交易员。',

    faqCreateTrader: '如何创建并启动交易员？',
    faqCreateTraderAnswer:
      '进入交易员页面：1）点击"创建交易员"；2）选择 AI 模型（需先配置）；3）选择交易所（需先配置）；4）选择策略（或使用默认）；5）设置决策间隔（如 5 分钟）；6）保存，然后点击"启动"开始交易。在仪表板页面监控表现。',

    // ===== 交易相关 =====
    faqHowAIDecides: 'AI 如何做出交易决策？',
    faqHowAIDecidesAnswer:
      'AI 使用思维链（CoT）推理分 4 步：1）持仓分析 - 审查当前持仓和盈亏；2）风险评估 - 检查账户保证金、可用余额；3）机会评估 - 分析市场数据、指标、候选币种；4）最终决策 - 输出具体操作（买入/卖出/持有）及理由。您可以在决策日志中查看完整推理过程。',

    faqDecisionFrequency: 'AI 多久做一次决策？',
    faqDecisionFrequencyAnswer:
      '每个交易员可单独配置，默认 3-5 分钟。考虑因素：太频繁（1-2 分钟）= 过度交易、手续费高；太慢（30+ 分钟）= 错过机会。建议：活跃交易 5 分钟，波段交易 15-30 分钟。AI 在很多周期可能决定"持有"（不操作）。',

    faqNoTradesExecuting: '为什么交易员不执行任何交易？',
    faqNoTradesExecutingAnswer:
      '常见原因：1）AI 决定等待（查看决策日志了解原因）；2）合约账户余额不足；3）达到最大持仓数限制（默认：3）；4）交易所 API 问题（检查错误信息）；5）策略约束太严格。查看仪表板 → 决策日志了解每个周期的 AI 推理详情。',

    faqOnlyShortPositions: '为什么 AI 只开空单？',
    faqOnlyShortPositionsAnswer:
      '通常是因为币安持仓模式问题。解决方案：在币安合约 → 偏好设置 → 持仓模式中切换为双向持仓。必须先平掉所有持仓。切换后，AI 可以独立开多单和空单。',

    faqLeverageSettings: '杠杆设置如何工作？',
    faqLeverageSettingsAnswer:
      '杠杆在策略 → 风控中设置：BTC/ETH 杠杆（通常 5-20 倍）和山寨币杠杆（通常 3-10 倍）。更高杠杆 = 更高风险和潜在收益。子账户可能有限制（如币安子账户限制 5 倍）。AI 下单时会遵守这些限制。',

    faqStopLossTakeProfit: 'NOFX 支持止损止盈吗？',
    faqStopLossTakeProfitAnswer:
      'AI 可以在决策中建议止损/止盈价位，但这是基于指导而非交易所硬编码订单。AI 每个周期监控持仓，可能根据盈亏决定平仓。如需保证止损，可以手动在交易所设置订单，或调整策略提示词使其更保守。',

    faqMultipleTraders: '可以运行多个交易员吗？',
    faqMultipleTradersAnswer:
      '可以！NOFX 支持运行 20+ 个并发交易员。每个交易员可以有不同的：AI 模型、交易所账户、策略、决策间隔。用于 A/B 测试策略、比较 AI 模型或跨交易所分散风险。在竞赛页面监控所有交易员。',

    faqAICosts: 'AI API 调用费用是多少？',
    faqAICostsAnswer:
      '每个交易员每天大约费用（5 分钟间隔）：DeepSeek：$0.10-0.50；Qwen：$0.20-0.80；OpenAI：$2-5；Claude：$1-3。费用取决于提示词长度和响应 token 数。DeepSeek 性价比最高。更长的决策间隔可降低费用。',

    // ===== 技术问题 =====
    faqPortInUse: '端口 8080 或 3000 被占用',
    faqPortInUseAnswer:
      '查看占用端口的进程：macOS/Linux 用 "lsof -i :8080"，Windows 用 "netstat -ano | findstr 8080"。终止进程或在 .env 中修改端口：NOFX_BACKEND_PORT=8081、NOFX_FRONTEND_PORT=3001。然后 "docker compose down && docker compose up -d" 重启。',

    faqFrontendNotLoading: '前端一直显示"加载中..."',
    faqFrontendNotLoadingAnswer:
      '后端可能未运行或无法访问。检查：1）"curl http://127.0.0.1:8080/api/health" 应返回 {"status":"ok"}；2）"docker compose ps" 验证容器运行中；3）查看后端日志："docker compose logs nofx-backend"；4）确保防火墙允许 8080 端口。',

    faqDatabaseLocked: '数据库锁定错误',
    faqDatabaseLockedAnswer:
      '多个进程同时访问 SQLite 导致。解决方案：1）停止所有进程："docker compose down" 或 "pkill nofx"；2）如有锁文件删除："rm -f data/data.db-wal data/data.db-shm"；3）重启："docker compose up -d"。只能有一个后端实例访问数据库。',

    faqTALibNotFound: '构建时找不到 TA-Lib',
    faqTALibNotFoundAnswer:
      'TA-Lib 是技术指标所需。安装：macOS："brew install ta-lib"；Ubuntu/Debian："sudo apt-get install libta-lib0-dev"；CentOS："yum install ta-lib-devel"。安装后重新构建："go build -o nofx"。Docker 镜像已预装 TA-Lib。',

    faqAIAPITimeout: 'AI API 超时或连接被拒绝',
    faqAIAPITimeoutAnswer:
      '检查：1）API 密钥有效（用 curl 测试）；2）网络能访问 API 端点（ping/curl）；3）API 提供商未宕机（查看状态页）；4）VPN/防火墙未阻止；5）未超过速率限制。默认超时 120 秒。',

    faqBinancePositionMode: '币安错误代码 -4061（持仓模式）',
    faqBinancePositionModeAnswer:
      '错误："Order\'s position side does not match user\'s setting"。您处于单向持仓模式，但 NOFX 需要双向持仓模式。修复：1）先平掉所有持仓；2）币安合约 → 设置（齿轮图标）→ 偏好设置 → 持仓模式 → 切换为"双向持仓"；3）重启交易员。',

    faqBalanceShowsZero: '账户余额显示 0',
    faqBalanceShowsZeroAnswer:
      '资金可能在现货钱包而非合约钱包。解决方案：1）在币安进入钱包 → 合约 → 划转；2）将 USDT 从现货划转到合约；3）刷新 NOFX 仪表板。也检查：资金未被理财/质押产品锁定。',

    faqDockerPullFailed: 'Docker 镜像拉取失败或缓慢',
    faqDockerPullFailedAnswer:
      'Docker Hub 在某些地区可能较慢。解决方案：1）在 /etc/docker/daemon.json 配置 Docker 镜像：{"registry-mirrors": ["https://mirror.gcr.io"]}；2）重启 Docker；3）重试拉取。或使用 GitHub Container Registry（ghcr.io）在您的地区可能连接更好。',

    // ===== 安全相关 =====
    faqAPIKeyStorage: 'API 密钥如何存储？',
    faqAPIKeyStorageAnswer:
      'API 密钥使用 AES-256-GCM 加密后存储在本地 SQLite 数据库中。加密密钥（DATA_ENCRYPTION_KEY）存储在您的 .env 文件中。密钥仅在 API 调用需要时在内存中解密。切勿分享您的 data.db 或 .env 文件。',

    faqEncryptionDetails: 'NOFX 使用什么加密？',
    faqEncryptionDetailsAnswer:
      'NOFX 使用多层加密：1）AES-256-GCM 用于数据库存储（API 密钥、密钥）；2）RSA-2048 用于可选的传输加密（浏览器到服务器）；3）JWT 用于认证令牌。密钥在安装时生成。HTTPS 环境启用 TRANSPORT_ENCRYPTION=true。',

    faqSecurityBestPractices: '安全最佳实践是什么？',
    faqSecurityBestPracticesAnswer:
      '建议：1）使用带 IP 白名单和最小权限（仅合约交易）的交易所 API 密钥；2）为 NOFX 使用专用子账户；3）远程部署启用 TRANSPORT_ENCRYPTION；4）切勿分享 .env 或 data.db 文件；5）使用有效证书的 HTTPS；6）定期轮换 API 密钥；7）监控账户活动。',

    faqCanNOFXStealFunds: 'NOFX 会盗取我的资金吗？',
    faqCanNOFXStealFundsAnswer:
      'NOFX 是开源的（AGPL-3.0 许可）- 您可以在 GitHub 审计所有代码。API 密钥存储在您的机器本地，从不发送到外部服务器。NOFX 只有您通过 API 密钥授予的权限。为最大安全：使用仅交易权限（无提现）的 API 密钥，启用 IP 白名单，使用专用子账户。',

    // ===== 功能介绍 =====
    faqStrategyStudio: '什么是策略工作室？',
    faqStrategyStudioAnswer:
      '策略工作室是可视化策略构建器，您可以配置：1）币种来源 - 交易哪些加密货币（静态列表、AI500 热门币、OI 排行）；2）技术指标 - EMA、MACD、RSI、ATR、成交量、持仓量、资金费率；3）风控 - 杠杆限制、仓位大小、保证金上限；4）自定义提示词 - AI 的特定指令。无需编程。',

    faqBacktestLab: '什么是回测实验室？',
    faqBacktestLabAnswer:
      '回测实验室用历史数据测试您的策略，无需冒真金风险。功能：1）配置 AI 模型、日期范围、初始余额；2）实时观看进度和权益曲线；3）查看指标：收益率、最大回撤、夏普比率、胜率；4）分析单笔交易和 AI 推理。实盘交易前验证策略的必备工具。',

    faqDebateArena: '什么是辩论竞技场？',
    faqDebateArenaAnswer:
      '辩论竞技场让多个 AI 模型在执行前辩论交易决策。设置：1）选择 2-5 个 AI 模型；2）分配角色（多头、空头、分析师、逆向者、风险经理）；3）观看他们多轮辩论；4）基于共识投票做最终决策。适用于需要多角度考虑的高确信度交易。',

    faqCompetitionMode: '什么是竞赛模式？',
    faqCompetitionModeAnswer:
      '竞赛页面显示所有交易员的实时排行榜。比较：ROI、盈亏、夏普比率、胜率、交易次数。用于 A/B 测试不同 AI 模型、策略或配置。交易员可标记为"在竞赛中显示"以出现在排行榜上。',

    faqChainOfThought: '什么是思维链（CoT）？',
    faqChainOfThoughtAnswer:
      '思维链是 AI 的推理过程，可在决策日志中查看。AI 分 4 步解释思考：1）当前持仓分析；2）账户风险评估；3）市场机会评估；4）最终决策理由。这种透明度帮助您理解 AI 为什么做出每个决策，有助于改进策略。',

    // ===== AI 模型 =====
    faqWhichAIModelBest: '应该使用哪个 AI 模型？',
    faqWhichAIModelBestAnswer:
      '推荐：DeepSeek 性价比最高（每天 $0.10-0.50）。备选：OpenAI 推理能力最强但贵（每天 $2-5）；Claude 适合细致分析；Qwen 价格有竞争力。您可以运行多个交易员使用不同模型进行比较。查看竞赛页面看哪个对您的策略表现最好。',

    faqCustomAIAPI: '可以使用自定义 AI API 吗？',
    faqCustomAIAPIAnswer:
      '可以！NOFX 支持任何 OpenAI 兼容的 API。在配置 → AI 模型 → 自定义 API 中：1）输入 API 端点 URL（如 https://your-api.com/v1）；2）输入 API 密钥；3）指定模型名称。适用于自托管模型、替代提供商或通过第三方代理的 Claude。',

    faqAIHallucinations: 'AI 幻觉问题怎么办？',
    faqAIHallucinationsAnswer:
      'AI 模型有时会产生不正确或虚构的信息（"幻觉"）。NOFX 通过以下方式缓解：1）提供带真实市场数据的结构化提示词；2）强制 JSON 输出格式；3）执行前验证订单。但 AI 交易是实验性的 - 始终监控决策，不要完全依赖 AI 判断。',

    faqCompareAIModels: '如何比较不同 AI 模型？',
    faqCompareAIModelsAnswer:
      '创建多个交易员，使用不同 AI 模型但相同策略/交易所。同时运行并在竞赛页面比较。关注指标：ROI、胜率、夏普比率、最大回撤。或者使用回测实验室用相同历史数据测试模型。辩论竞技场也展示不同模型对同一情况的推理方式。',

    // ===== 参与贡献 =====
    faqHowToContribute: '如何为 NOFX 做贡献？',
    faqHowToContributeAnswer:
      'NOFX 是开源项目，欢迎贡献！贡献方式：1）代码 - 修复 bug、添加功能（查看 GitHub Issues）；2）文档 - 改进指南、翻译；3）Bug 报告 - 详细报告问题；4）功能建议 - 提出改进意见。从标记为"good first issue"的问题开始。所有贡献者可能获得空投奖励。',

    faqPRGuidelines: 'PR 指南是什么？',
    faqPRGuidelinesAnswer:
      'PR 流程：1）Fork 仓库到您的账户；2）从 dev 创建功能分支："git checkout -b feat/your-feature"；3）修改代码，运行 lint："npm --prefix web run lint"；4）使用 Conventional Commits 格式提交；5）推送并创建 PR 到 NoFxAiOS/nofx:dev；6）关联相关 issue（Closes #123）；7）等待审核。保持 PR 小而聚焦。',

    faqBountyProgram: '有赏金计划吗？',
    faqBountyProgramAnswer:
      '有！贡献者根据贡献获得空投奖励：代码提交（权重最高）、bug 修复、功能建议、文档。带"bounty"标签的 issue 有现金奖励。完成工作后提交 Bounty Claim。查看 CONTRIBUTING.md 了解奖励结构详情。',

    faqReportBugs: '如何报告 bug？',
    faqReportBugsAnswer:
      'Bug 报告：在 GitHub 开 Issue，包含：1）问题清晰描述；2）复现步骤；3）预期 vs 实际行为；4）系统信息（OS、Docker 版本、浏览器）；5）相关日志。安全漏洞：不要开公开 issue - 请在 Twitter 私信 @Web3Tinkle。',

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

    // Debate Arena Page
    debatePage: {
      title: '行情辩论大赛',
      subtitle: '观看AI模型辩论市场行情并达成共识',
      newDebate: '新建辩论',
      noDebates: '暂无辩论',
      createFirst: '创建您的第一场辩论开始',
      selectDebate: '选择辩论查看详情',
      createDebate: '创建辩论',
      creating: '创建中...',
      debateName: '辩论名称',
      debateNamePlaceholder: '例如：BTC是牛还是熊？',
      tradingPair: '交易对',
      strategy: '策略',
      selectStrategy: '选择策略',
      maxRounds: '最大回合',
      autoExecute: '自动执行',
      autoExecuteHint: '自动执行共识交易',
      participants: '参与者',
      addParticipant: '添加AI参与者',
      noModels: '暂无可用AI模型',
      atLeast2: '至少添加2名参与者',
      personalities: {
        bull: '激进多头',
        bear: '谨慎空头',
        analyst: '数据分析师',
        contrarian: '逆势者',
        risk_manager: '风控经理',
      },
      status: {
        pending: '待开始',
        running: '进行中',
        voting: '投票中',
        completed: '已完成',
        cancelled: '已取消',
      },
      actions: {
        start: '开始辩论',
        starting: '启动中...',
        cancel: '取消',
        delete: '删除',
        execute: '执行交易',
      },
      round: '回合',
      roundOf: '第 {current} / {max} 回合',
      messages: '消息',
      noMessages: '暂无消息',
      waitingStart: '等待辩论开始...',
      votes: '投票',
      consensus: '共识',
      finalDecision: '最终决定',
      confidence: '信心度',
      votesCount: '{count} 票',
      decision: {
        open_long: '开多',
        open_short: '开空',
        close_long: '平多',
        close_short: '平空',
        hold: '持有',
        wait: '观望',
      },
      messageTypes: {
        analysis: '分析',
        rebuttal: '反驳',
        vote: '投票',
        summary: '总结',
      },
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
