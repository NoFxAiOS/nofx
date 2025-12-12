# Feature Proposal: User Profile Auto-Refresh

## 1. Context & Objectives
Currently, user data (including `invite_code`) is cached in `localStorage` and only updated on Login/Register. This causes stale data issues for existing sessions when new fields are added.
**Goal**: Implement a mechanism to automatically refresh the user's profile from the backend upon application load.

## 2. Technical Architecture

### 2.1 Backend API
**Endpoint**: `GET /api/user/me`
**Auth**: Required (Bearer Token).
**Response**:
```json
{
  "id": "user_uuid",
  "email": "user@example.com",
  "invite_code": "ABC12345",
  "is_admin": false,
  "created_at": "2025-..."
}
```

### 2.2 Frontend Logic
**File**: `web/src/contexts/AuthContext.tsx`
-   **Action**: Add `fetchCurrentUser()` function.
-   **Trigger**: Call `fetchCurrentUser()` inside the existing `useEffect` that checks for `savedToken`.
-   **State Update**: If API call succeeds, update `user` state and `localStorage`. If it fails (e.g., 401), handle logout.

## 3. Implementation Plan

1.  **Backend**:
    -   Modify `api/server.go` to add `GET /api/user/me`.
    -   Implement handler `handleGetMe`.
2.  **Frontend**:
    -   Modify `web/src/contexts/AuthContext.tsx`.
    -   Implement `fetchCurrentUser`.
    -   Integrate into initialization flow.

## 4. Performance & Error Handling
-   **Frequency**: Once per session initialization (App mount).
-   **Error Handling**:
    -   401 Unauthorized -> Clear token, Logout.
    -   Network Error -> Keep using cached data (Graceful degradation) but show warning (optional, silent fail is better for UX if cache exists).

## 5. Verification
-   **Manual**:
    1.  Log in.
    2.  Manually edit `localStorage` to remove `invite_code`.
    3.  Refresh page.
    4.  Verify `invite_code` reappears in `localStorage` and Profile UI.
