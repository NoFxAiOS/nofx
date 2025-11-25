# Security Policy

## 🔒 Security at NOFX

We take the security of NOFX seriously. This document outlines our security policy and procedures for reporting vulnerabilities.

## 📋 Supported Versions

We release patches for security vulnerabilities. Which versions are eligible for receiving such patches depends on the CVSS v3.0 Rating:

| Version | Supported          | Status |
| ------- | ------------------ | ------ |
| 3.x.x   | ✅ Yes             | Active development |
| 2.x.x   | ⚠️ Limited support | Security fixes only |
| < 2.0   | ❌ No              | No longer supported |

## 🚨 Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

If you discover a security vulnerability, please follow these steps:

### 1. Private Disclosure

Send an email to the security team at:
- **Email**: tinklefund@gmail.com (or contact maintainers directly via Twitter DM)
- **Twitter**: [@nofx_official](https://x.com/nofx_official) or [@Web3Tinkle](https://x.com/Web3Tinkle)

### 2. Information to Include

Please include the following details in your report:

- **Description**: A clear description of the vulnerability
- **Impact**: The potential impact of the vulnerability
- **Steps to Reproduce**: Detailed steps to reproduce the issue
- **Proof of Concept**: If applicable, include PoC code or screenshots
- **Suggested Fix**: If you have ideas on how to fix it
- **Your Contact Information**: For follow-up questions

### 3. Response Timeline

- **Initial Response**: Within 48 hours of receiving your report
- **Status Update**: Weekly updates on the progress
- **Fix Timeline**: Critical issues within 7 days, others within 30 days
- **Public Disclosure**: After the fix is deployed (coordinated disclosure)

### 4. What to Expect

After you submit a report:

1. ✅ We will acknowledge receipt of your report
2. 🔍 We will investigate and validate the issue
3. 📋 We will develop and test a fix
4. 🚀 We will deploy the fix to production
5. 📢 We will coordinate public disclosure with you
6. 🏆 We will credit you in the security advisory (if desired)

## 🛡️ Security Best Practices

If you're using NOFX, please follow these security best practices:

### API Keys and Secrets

- ❌ **Never commit** API keys, private keys, or secrets to version control
- ✅ **Use environment variables** for all sensitive configuration
- ✅ **Rotate keys regularly** (at least every 90 days)
- ✅ **Use separate keys** for different environments (dev/staging/prod)
- ✅ **Implement IP whitelisting** for exchange API keys
- ✅ **Enable 2FA** on all exchange accounts

### Private Keys (Hyperliquid/Aster)

- ❌ **Never share** your private keys with anyone
- ✅ **Use dedicated wallets** for trading (not your main wallet)
- ✅ **Use agent wallets** when available (Hyperliquid)
- ✅ **Limit wallet funds** to amounts you can afford to lose
- ✅ **Back up keys securely** using encrypted storage

### API Security

- ✅ **Enable API key restrictions** (IP whitelist, permissions)
- ✅ **Use read-only keys** for monitoring when possible
- ✅ **Set withdrawal restrictions** on exchange accounts
- ✅ **Monitor API usage** for unusual activity
- ✅ **Revoke compromised keys** immediately

### System Security

- ✅ **Keep dependencies updated** (run `npm audit` and `go mod tidy`)
- ✅ **Use HTTPS** for all external communications
- ✅ **Implement rate limiting** on API endpoints
- ✅ **Enable authentication** on production deployments
- ✅ **Review logs regularly** for suspicious activity
- ✅ **Use Docker** for isolated environments

### Database Security

- ✅ **Encrypt sensitive data** at rest (API keys, private keys)
- ✅ **Restrict database access** (not exposed to internet)
- ✅ **Back up regularly** with encrypted backups
- ✅ **Use strong passwords** for database credentials

### Configuration Security

- ❌ **Never use default passwords** or weak credentials
- ✅ **Change default ports** if exposed to internet
- ✅ **Disable unnecessary features** in production
- ✅ **Use firewall rules** to restrict access
- ✅ **Implement RBAC** for multi-user setups

## 🚫 Out of Scope

The following are **not** considered security vulnerabilities:

- ❌ Trading losses due to AI decisions
- ❌ Exchange API rate limiting
- ❌ Network latency issues
- ❌ Market volatility impacts
- ❌ Social engineering attacks
- ❌ DDoS attacks on public infrastructure
- ❌ Issues in third-party dependencies (report to upstream)
- ❌ Already known and documented limitations

## 🏅 Recognition

We appreciate the security research community's efforts. Contributors who responsibly disclose vulnerabilities will be:

- ✅ Credited in security advisories (with permission)
- ✅ Listed in our Hall of Fame (coming soon)
- ✅ Eligible for bug bounties (when program launches)

## 📚 Security Resources

### Documentation

- [Getting Started Guide](../docs/getting-started/README.md)
- [Architecture Documentation](../docs/architecture/README.md)
- [Docker Deployment Guide](../docs/getting-started/docker-deploy.en.md)
- [Troubleshooting Guide](../docs/guides/TROUBLESHOOTING.md)

### Security Tools

- **Code Scanning**: GitHub Advanced Security (enabled)
- **Dependency Scanning**: Dependabot (enabled)
- **Secret Scanning**: GitHub Secret Scanning (enabled)
- **Container Scanning**: Docker Scout (recommended)

### External Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CWE Top 25](https://cwe.mitre.org/top25/archive/2023/2023_top25_list.html)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)

## 🔐 Encryption & Secure Storage

NOFX uses the following security measures:

- **AES-256 encryption** for sensitive data at rest (planned v3.1)
- **TLS 1.3** for all network communications
- **JWT tokens** for API authentication
- **bcrypt** for password hashing (where applicable)
- **Environment isolation** via Docker containers

## 📝 Security Audit History

| Date | Version | Auditor | Report |
|------|---------|---------|--------|
| TBD  | 3.0.0   | Internal | Initial security review |

## 🤝 Responsible Disclosure Policy

We follow a **coordinated disclosure** approach:

1. 📧 Report received and acknowledged
2. 🔍 Investigation and validation (1-7 days)
3. 🛠️ Fix development and testing (7-30 days)
4. 🚀 Fix deployment to production
5. 📢 Public advisory published (after fix)
6. 🏆 Credit to researcher (if desired)

**Please allow us time to fix critical issues before public disclosure.**

## 📞 Contact

For security concerns, reach out via:

- **Email**: Contact maintainers (see [GitHub profile](https://github.com/tinkle-community/nofx))
- **Twitter**: [@nofx_official](https://x.com/nofx_official) (DM open)
- **Telegram**: [NOFX Developer Community](https://t.me/nofx_dev_community)
- **GitHub**: Private security advisory (preferred for verified issues)

## ⚖️ Legal

**Safe Harbor**: We consider security research conducted under this policy to be:

- ✅ Authorized in accordance with applicable law
- ✅ Lawful and in good faith
- ✅ Exempt from DMCA and CFAA claims
- ✅ Protected from legal action by the project

**Conditions**:
- Make a good faith effort to avoid privacy violations
- Do not access or modify other users' data
- Do not disrupt our services or infrastructure
- Do not publicly disclose issues before we've had time to address them

## 🔄 Updates to This Policy

This security policy may be updated from time to time. We will notify users of significant changes via:

- GitHub release notes
- Security advisories
- Community channels (Telegram, Twitter)

---

**Last Updated**: January 2025
**Version**: 1.0.0

Thank you for helping keep NOFX and its users safe! 🙏

---

## 📖 Additional Resources

- [Contributing Guidelines](../CONTRIBUTING.md)
- [Code of Conduct](../CODE_OF_CONDUCT.md)
- [License](../LICENSE)
- [Changelog](../CHANGELOG.md)
