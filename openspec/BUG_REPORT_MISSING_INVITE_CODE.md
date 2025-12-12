# Bug Report: Missing Invitation Code on Profile Page

**Status:** Investigated
**Date:** 2025-12-11

## 1. Issue Description
Users reported that the "Invitation Code" section is missing from the User Profile page (`/profile`), despite the feature being implemented.

## 2. Root Cause Analysis
The investigation reveals that the issue is caused by **Stale Client-Side Data Persistence**.

1.  **Mechanism**: The `AuthContext` in the frontend persists the `user` object (which contains fields like `id`, `email`, and now `invite_code`) to the browser's `localStorage` to maintain sessions across reloads.
2.  **The Gap**: The `invite_code` field was added to the `User` object structure in the most recent update.
    -   **New Logins/Registrations**: When a user logs in or registers *after* the update, the backend returns the new `invite_code`, and it is correctly saved to `localStorage`. The feature works as expected.
    -   **Existing Sessions**: For users who were already logged in (or have a valid token in `localStorage`), the application loads the *old* user object from storage. This old object **does not** contain the `invite_code` field.
3.  **Missing Refresh**: The current frontend architecture does not automatically refresh the user's basic profile (like `invite_code`) from the backend when the app loads. It trusts the cached data in `localStorage`.

## 3. Verification
The feature logic in `UserProfilePage.tsx` explicitly checks for the existence of the code:
```typescript
{user?.invite_code && ( ... )}
```
Since `user.invite_code` is undefined in stale sessions, the section is not rendered.

## 4. Solutions

### 4.1 Immediate Workaround (For Testers/Users)
**Logout and Login again.**
This will force the frontend to fetch the latest user object (including the invite code) from the backend and update the local storage.

### 4.2 Permanent Engineering Fix (Recommended)
To prevent this issue for all users and ensure data consistency:

1.  **Backend**: Implement a new API endpoint `GET /api/user/me` (or `/api/user/profile`) that returns the current authenticated user's details (ID, Email, InviteCode, etc.).
2.  **Frontend**: Modify `AuthContext.tsx` to call this endpoint upon application initialization (if a token exists) to refresh the `user` state and `localStorage` with the latest server data.

## 5. Conclusion
The code is functional, but the data migration strategy for *client-side* state was missing. A re-login resolves the immediate issue.
