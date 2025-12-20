# AGPL Violation Evidence Report: ChainOpera Plagiarized NOFX

**Report Date**: December 20, 2025
**Reporting Party**: NOFX Open Source Community
**Project URL**: https://github.com/NoFxAiOS/nofx
**Accused Party**: ChainOpera (COAI)
**License Involved**: GNU Affero General Public License v3.0 (AGPL-3.0)

---

## 1. Executive Summary

ChainOpera used the `equity-history-batch` API interface design from the NOFX project, which is protected under AGPL-3.0, on their website `trading-test.chainopera.ai`, but refused to release their source code, violating the AGPL-3.0 license terms.

ChainOpera claims the interface was "rewritten in Python." This report will prove from both legal and technical perspectives that: **even a rewrite still constitutes an AGPL violation**.

---

## 2. Timeline Evidence

### 2.1 AGPL License Effective Date

| Item | Details |
|------|---------|
| **Effective Time** | 2025-11-03 19:50:50 (UTC+8) |
| **Commit Hash** | `e88f84215831d1682e05141eb0c27216dcbd6d47` |
| **Author** | SkywalkerJi <skywalkerji.cn@gmail.com> |
| **Commit Message** | "Upgrade this repository's open-source license to AGPL." |

### 2.2 equity-history-batch Interface Creation Date

| Item | Details |
|------|---------|
| **Creation Time** | 2025-11-03 20:14:39 (UTC+8) |
| **Commit Hash** | `5af5c0b51773737f166eacea646e3960cee29f59` |
| **Author** | icy <icyoung520@gmail.com> |
| **Commit Message** | "Enhance leaderboard and security for trader management" |

### 2.3 Key Conclusion

```
AGPL Effective Time: 2025-11-03 19:50:50
Interface Creation:  2025-11-03 20:14:39
Time Difference:     24 minutes

Conclusion: The equity-history-batch interface has been protected
under AGPL-3.0 since its creation.
```

---

## 3. Technical Evidence: Code Comparison

### 3.1 API Path Comparison

| Project | API Path | HTTP Method |
|---------|----------|-------------|
| **NOFX** | `/api/equity-history-batch` | POST |
| **ChainOpera** | `/api/equity-history-batch` | POST |

**Similarity: 100%**

### 3.2 Response Structure Comparison

**NOFX Original Code** (`api/server.go` lines 2725-2729):

```go
result["histories"] = histories
result["count"] = len(histories)
if len(errors) > 0 {
    result["errors"] = errors
}
```

**ChainOpera Actual Response** (from network request screenshot):

![ChainOpera API Evidence Screenshot](./chainopera-evidence-screenshot.png)

```json
{
  "histories": {...},
  "errors": {},
  "count": 1
}
```

**Comparison Results**:

| Field | NOFX | ChainOpera | Similarity |
|-------|------|------------|------------|
| `histories` | ✓ | ✓ | 100% |
| `errors` | ✓ | ✓ | 100% |
| `count` | ✓ | ✓ | 100% |

### 3.3 History Data Fields Comparison

**NOFX Original Code** (`api/server.go` lines 2676-2682):

```go
history = append(history, map[string]interface{}{
    "timestamp":     snap.Timestamp,
    "total_equity":  snap.TotalEquity,
    "total_pnl":     snap.UnrealizedPnL,
    "total_pnl_pct": pnlPct,
    "balance":       snap.Balance,
})
```

**ChainOpera Actual Response**:

```json
{
  "timestamp": "2025-12-15T11:21:05.432240",
  "balance": 227.30274403,
  "equity": 227.30274403,
  "total_pnl": 0
}
```

**Comparison Results**:

| NOFX Field | ChainOpera Field | Similarity |
|------------|------------------|------------|
| `timestamp` | `timestamp` | 100% |
| `balance` | `balance` | 100% |
| `total_equity` | `equity` | Semantically identical |
| `total_pnl` | `total_pnl` | 100% |

### 3.4 Originality Evidence

`equity-history-batch` is an **original design** by NOFX:

1. **Interface Naming**: `equity-history-batch` is a self-created compound term, not an industry standard
2. **Batch Query Design**: Supporting multiple trader_id queries simultaneously is a unique design for performance optimization
3. **Response Structure**: The `{histories, errors, count}` triplet is an original design
4. **Time Filtering**: The `hours` parameter design is an original feature

---

## 4. Legal Rebuttal to the "Python Rewrite" Defense

### 4.1 AGPL-3.0 Definition of "Modify"

**AGPL-3.0 Section 0**:

> "To 'modify' a work means to copy from or adapt all or part of the work in a fashion requiring copyright permission, other than the making of an exact copy."

**Key Point**: Rewriting in another language (Go → Python) constitutes "adaptation" and remains subject to AGPL.

### 4.2 Legal Definition of Derivative Works

**AGPL-3.0 Section 0**:

> A "covered work" means either the unmodified Program or a work based on the Program.

**U.S. Copyright Law 17 U.S.C. § 101**:

> A "derivative work" is a work based upon one or more preexisting works, such as a translation... in which a work may be recast, transformed, or adapted.

**Key Point**: "Translating" Go code to Python code falls under the legal definition of a "derivative work."

### 4.3 Why "Rewriting" Still Constitutes Infringement

| Argument | Legal Analysis |
|----------|----------------|
| "We rewrote it in Python" | Language conversion is "adaptation"; derivative works must comply with the original license |
| "The code is completely different" | Copyright protects **expression**; API design is a form of expression |
| "This is generic functionality" | `equity-history-batch` naming and `{histories, errors, count}` structure are original designs, not generic functionality |

### 4.4 Case Reference

**Oracle v. Google (2021)**:

The U.S. Supreme Court confirmed that API designs are subject to copyright protection. Even though Google reimplemented the Java API, copyright issues still needed to be considered.

**Key Implications**:
- The **Structure, Sequence, and Organization (SSO)** of APIs is protected by copyright
- Even if implemented in a different language, identical API designs may still constitute infringement

---

## 5. Questions ChainOpera Must Answer

ChainOpera has not responded to the following core questions:

| # | Question | ChainOpera Response |
|---|----------|---------------------|
| 1 | Why is the API path identical to NOFX? | ❌ No response |
| 2 | Why is the response structure `{histories, errors, count}` identical? | ❌ No response |
| 3 | Why are field names `timestamp, balance, total_pnl` identical? | ❌ No response |
| 4 | If independently developed, why is it highly consistent with NOFX? | ❌ No response |
| 5 | Are you willing to release source code per AGPL-3.0? | ❌ No response |

---

## 6. Git Evidence Verification Method

Anyone can verify the authenticity of the evidence with the following commands:

```bash
# Clone the repository
git clone https://github.com/NoFxAiOS/nofx.git
cd nofx

# Verify AGPL license effective date
git show e88f84215831d1682e05141eb0c27216dcbd6d47 --format="%H %ai %s" --no-patch
# Output: e88f8421... 2025-11-03 19:50:50 +0800 Upgrade this repository's open-source license to AGPL.

# Verify equity-history-batch interface creation date
git show 5af5c0b51773737f166eacea646e3960cee29f59 --format="%H %ai %s" --no-patch
# Output: 5af5c0b5... 2025-11-03 20:14:39 +0800 Enhance leaderboard and security for trader management

# View interface implementation code
git show 5af5c0b51773737f166eacea646e3960cee29f59:api/server.go | grep -A 50 "handleEquityHistoryBatch"
```

---

## 7. Legal Basis Summary

### 7.1 Key AGPL-3.0 Provisions

**Section 13 - Remote Network Interaction**:

> Notwithstanding any other provision of this License, if you modify the Program, your modified version must prominently offer all users interacting with it remotely through a computer network... an opportunity to receive the Corresponding Source of your version.

### 7.2 ChainOpera's Violations

| Violation | Description |
|-----------|-------------|
| Using AGPL code | Used NOFX's API design |
| Providing network service | Operating publicly at `trading-test.chainopera.ai` |
| Not releasing source code | No source code access provided |
| Not declaring license | Did not declare use of AGPL code |

---

## 8. Additional Evidence: Brand and Slogan Plagiarism

### 8.1 Google Search Results Evidence

![ChainOpera Google Search Evidence](./chainopera-evidence-google-search.png)

**Screenshot Time**: December 19, 2025 07:58:29 (Time.is third-party timestamp)

### 8.2 Key Findings

| Evidence Item | Content | Analysis |
|---------------|---------|----------|
| **Website Description** | "The future standard for AI Trading - an open community-driven agentic trading OS" | Highly consistent with NOFX's slogan |
| **Login Page** | Displays "NoFx Logo" | Direct use of NOFX brand assets |

### 8.3 Brand Infringement Evidence

ChainOpera's website `trading-test.chainopera.ai` Login page HTML contains **"NoFx Logo"** text, proving:

1. ChainOpera directly used NOFX's frontend code
2. They didn't even modify brand-related text identifiers
3. This is not "independent development" or "Python rewrite" - it's direct copying

---

## 9. Evidence List

| # | Evidence Type | Description | Preservation Method |
|---|---------------|-------------|---------------------|
| 1 | Git Commit | AGPL license effective record | SHA-1: `e88f8421...` |
| 2 | Git Commit | equity-history-batch creation record | SHA-1: `5af5c0b5...` |
| 3 | Source Code | api/server.go lines 2542-2732 | Git repository |
| 4 | Website Screenshot | ChainOpera API response | Blockchain timestamping |
| 5 | Network Request | trading-test.chainopera.ai request logs | Notarization recommended |
| 6 | Google Search | "NoFx Logo" brand infringement evidence | Screenshot + Time.is timestamp |

---

## 10. Conclusions

1. **Timeline evidence is conclusive**: The `equity-history-batch` interface was created 24 minutes after AGPL took effect; it has been protected since inception.

2. **Technical evidence is sufficient**: API path, response structure, and field naming are highly consistent, beyond reasonable coincidence.

3. **"Python rewrite" defense is invalid**:
   - Language conversion constitutes "adaptation"; derivative works must comply with AGPL
   - API design itself is protected by copyright
   - Identical structure, sequence, and organization proves copying, not independent development

4. **ChainOpera must either**:
   - Release their complete source code in compliance with AGPL-3.0; OR
   - Cease using the related functionality and take down the service

5. **Based on the infringement that has already occurred, the NOFX community reserves the right to pursue the following legal remedies**:
   - **Injunctive Relief**: Immediately cease using NOFX's AGPL-protected code
   - **Public Acknowledgment**: Publicly disclose on ChainOpera's official channels that they used NOFX code
   - **Compensatory Damages**: Compensation for actual losses or disgorgement of profits obtained through infringement
   - **Statutory Damages**: Statutory damages under applicable jurisdiction
   - **Legal Costs**: Including but not limited to notarization fees, attorney fees, and litigation costs

   **Applicable International Legal Framework**:
   - Berne Convention - Computer programs protected as "literary works"
   - TRIPS Agreement Article 10 (WTO) - Computer programs, whether in source or object code, shall be protected as literary works
   - WIPO Copyright Treaty (WCT) Article 4 - Computer programs protected as literary works under Berne Convention

---

## 11. Contact Information

For any questions, please contact:

- **GitHub Issues**: https://github.com/NoFxAiOS/nofx/issues
- **Email**: contact@vergex.trade

---

**Disclaimer**: This report only states facts and legal analysis. The NOFX community reserves the right to pursue legal action for infringement.

---

*Report Version: 1.0*
*Last Updated: 2025-12-20*
