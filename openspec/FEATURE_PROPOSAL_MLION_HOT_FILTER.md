# Feature Proposal: Filter Hot News for Mlion Integration

## 1. Context & Objectives
To improve the signal-to-noise ratio of the crypto news feed, we need to filter the Mlion.ai real-time news source to only include "Hot" news.
Verification has confirmed that the Mlion API supports a server-side query parameter `is_hot=Y`.

## 2. Technical Specifications

### 2.1 API Change
-   **Original URL**: `https://api.mlion.ai/v2/api/news/real/time`
-   **New URL**: `https://api.mlion.ai/v2/api/news/real/time?is_hot=Y`
-   **Logic**: Append the query parameter to the request.

### 2.2 Impact Analysis
-   **Volume**: Expected to reduce news volume significantly (e.g., ~300 items -> ~75 items), focusing on high-impact news.
-   **Latency**: No impact.
-   **Code**: Modify `MlionFetcher` in `service/news/mlion.go`.

## 3. Implementation Plan

### Phase 1: Code Modification
-   Update `service/news/mlion.go`:
    -   Modify `mlionBaseURL` constant or append param in `FetchNews` method.
    -   Ensure the parameter `is_hot=Y` is sent.

### Phase 2: Verification
-   Run unit tests (mock needs update to expect the new URL or ignore it).
-   Run manual verification script to check volume reduction.

## 4. Testing
-   Update `service/news/mlion_test.go` to assert that the request URL contains `is_hot=Y`.
