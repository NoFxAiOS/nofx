# Implementation Tasks

## 1. Code Fix
- [ ] 1.1 Update conditional rendering logic in AITradersPage.tsx:1371
  - Change from: `(selectedExchange.type === 'cex') && selectedExchange.id !== 'hyperliquid' && selectedExchange.id !== 'aster'`
  - Change to: `(selectedExchange.id === 'binance' || selectedExchange.type === 'cex') && selectedExchange.id !== 'hyperliquid' && selectedExchange.id !== 'aster'`
- [ ] 1.2 Verify the fix aligns with existing debug logging logic (line 1254)

## 2. Testing
- [ ] 2.1 Test Binance exchange selection shows API Key and Secret Key fields
- [ ] 2.2 Test OKX exchange still shows API Key, Secret Key, and Passphrase fields
- [ ] 2.3 Test Hyperliquid still shows Private Key and Wallet Address fields
- [ ] 2.4 Test Aster exchange configuration (if applicable)

## 3. Deployment
- [ ] 3.1 Build and test locally
- [ ] 3.2 Commit changes with descriptive message
- [ ] 3.3 Deploy to production via Vercel
- [ ] 3.4 Verify fix in production environment

## 4. Documentation
- [ ] 4.1 Update debug logging if needed
- [ ] 4.2 Consider adding comments to clarify exchange type handling logic
