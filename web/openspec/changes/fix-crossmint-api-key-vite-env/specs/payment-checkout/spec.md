## MODIFIED Requirements

### Requirement: Crossmint Client API Key Environment Variable Configuration
The application SHALL properly expose the Crossmint Client API Key to the client-side Vite build using Vite's environment variable conventions, enabling the payment system to access the API key at runtime.

#### Scenario: Environment variable is exposed to client code
- **WHEN** Vite build runs during development or production
- **THEN** environment variable `VITE_CROSSMINT_CLIENT_API_KEY` is exposed to client code
- **AND** `import.meta.env.VITE_CROSSMINT_CLIENT_API_KEY` returns the configured API key value
- **AND** value is not empty or undefined

#### Scenario: CrossmintService receives the API key
- **WHEN** CrossmintService is instantiated
- **THEN** it successfully reads `VITE_CROSSMINT_CLIENT_API_KEY` from environment
- **AND** `isConfigured()` returns true
- **AND** no warning "API Key not configured" is logged

#### Scenario: PaymentModal can access the API key
- **WHEN** PaymentModal renders
- **THEN** it reads `VITE_CROSSMINT_CLIENT_API_KEY` successfully
- **AND** no error message "Payment feature temporarily unavailable" displays
- **AND** checkout widget can be initialized

#### Scenario: Vercel environment variables are properly configured
- **WHEN** application is deployed to Vercel
- **THEN** environment variable `VITE_CROSSMINT_CLIENT_API_KEY` is set in Vercel project settings
- **AND** variable is available for both production and preview environments
- **AND** value is correctly passed to the Vite build process

#### Scenario: TypeScript type checking works for the variable
- **WHEN** code accesses `import.meta.env.VITE_CROSSMINT_CLIENT_API_KEY`
- **THEN** TypeScript recognizes the variable as a valid environment variable
- **AND** no "Property 'VITE_CROSSMINT_CLIENT_API_KEY' does not exist" error
- **AND** IDE autocomplete suggests the variable

#### Scenario: No payment feature errors in production
- **WHEN** user visits production site and opens payment modal
- **THEN** no "API Key not configured" warning in console
- **AND** no "Payment feature temporarily unavailable" message displays
- **AND** payment UI initializes successfully

