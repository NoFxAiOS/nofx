# Exchange Configuration Specification

## MODIFIED Requirements

### Requirement: CEX Exchange Input Field Rendering
The system SHALL display appropriate input fields based on the exchange type and ID when users configure exchange connections in the AI Traders modal.

#### Scenario: Binance exchange configuration
- **WHEN** user selects Binance exchange from the dropdown
- **THEN** the system SHALL display API Key input field
- **AND** the system SHALL display Secret Key input field
- **AND** both fields SHALL be marked as required
- **AND** both fields SHALL use password input type for security

#### Scenario: OKX exchange configuration
- **WHEN** user selects OKX exchange from the dropdown
- **THEN** the system SHALL display API Key input field
- **AND** the system SHALL display Secret Key input field
- **AND** the system SHALL display Passphrase input field
- **AND** all fields SHALL be marked as required

#### Scenario: Standard CEX exchange configuration
- **WHEN** user selects a CEX exchange (type === 'cex') that is not Hyperliquid or Aster
- **THEN** the system SHALL display API Key input field
- **AND** the system SHALL display Secret Key input field
- **AND** both fields SHALL be marked as required

#### Scenario: Hyperliquid exchange configuration
- **WHEN** user selects Hyperliquid exchange
- **THEN** the system SHALL display Private Key input field instead of API Key
- **AND** the system SHALL display Wallet Address input field instead of Secret Key
- **AND** the system SHALL display descriptive help text for each field

#### Scenario: Aster exchange configuration
- **WHEN** user selects Aster exchange
- **THEN** the system SHALL display specialized fields appropriate for Aster
- **AND** the system SHALL NOT display standard CEX fields (API Key/Secret Key)

### Technical Implementation Note
The conditional logic SHALL check:
```typescript
(selectedExchange.id === 'binance' || selectedExchange.type === 'cex') &&
selectedExchange.id !== 'hyperliquid' &&
selectedExchange.id !== 'aster'
```

This ensures:
1. Binance (type: "binance") displays CEX fields by explicit ID check
2. Other CEX exchanges (type: "cex") display CEX fields by type check
3. Hyperliquid and Aster are explicitly excluded and use their own field sets

### Data Source
Backend API endpoint `/api/supported-exchanges` returns exchange configurations with:
- `id`: Unique exchange identifier (e.g., "binance", "okx", "hyperliquid")
- `type`: Exchange category (e.g., "binance", "cex", "dex")
- `name`: Human-readable display name
