# Feature Proposal: User Invitation System

## 1. Context & Objectives
To drive user growth, we need an invitation system where existing users can invite new users.
- **Single-layer Reward**: User A invites User B -> User A gets a reward. (User B invites C -> B gets reward, A gets nothing).
- **Tracking Structure**: We will track the full lineage (A->B->C) in the database for potential future features, even if rewards are currently single-level.
- **Reward**: 10 Credits for the inviter upon successful registration of the invitee.
- **Principles**: KISS (Keep It Simple, Stupid), High Cohesion, Low Coupling.

## 2. Technical Architecture

### 2.1 Database Schema Changes
We will modify the existing `users` table rather than creating a complex new relation, as the relationship is 1:N (User:Invitees).

**Table: `users`**
Add the following columns:
- `invite_code` (TEXT, UNIQUE, INDEX): The user's personal invitation code (8-12 chars).
- `invited_by_user_id` (TEXT, FK references users.id, NULLABLE): Who invited this user.
- `invitation_level` (INTEGER, DEFAULT 0): The depth in the invitation tree (Root=0, Invitee=1, etc.).

**Note**: We will NOT use a separate `invitation_codes` table as codes are permanent and tied to users 1:1.

### 2.2 Invitation Code Generation Rules
- **Format**: Alphanumeric [a-z, A-Z, 0-9].
- **Length**: 8 characters (sufficient entropy).
- **Uniqueness**: Guaranteed by database `UNIQUE` constraint.
- **Trigger**:
  - *New Users*: Generated automatically during registration.
  - *Existing Users*: Generated lazily (on first request) or via a one-time migration script.

### 2.3 Reward Logic
- **Trigger**: Successful registration of a new user who provided a valid `invite_code`.
- **Action**: Award 10 credits to the user identified by `invited_by_user_id`.
- **Module**: Utilize existing `config/credits.go` -> `AddCredits` method.
- **Category**: `referral_reward`.

## 3. Implementation Plan

### Phase 1: Database Migration
1. Create a migration script `database/migrations/xxxx_add_invitation_columns.sql`.
2. Add columns: `invite_code`, `invited_by_user_id`, `invitation_level`.
3. Create Index on `invite_code`.

### Phase 2: Backend Logic (`auth` & `service`)
1. **Utility**: Create `GenerateInviteCode()` in a util package.
2. **Registration Flow (`auth/register`)**:
   - Accept optional `inviteCode` in request body.
   - If provided:
     - Validate code exists in `users` table.
     - If invalid, return error or ignore (decision: return error for better UX).
     - Set `invited_by_user_id` = Inviter's ID.
     - Set `invitation_level` = Inviter's Level + 1.
   - Generate `invite_code` for the *new* user.
3. **Post-Registration Hook**:
   - If `invited_by_user_id` is present:
     - Call `credits.AddCredits(inviterID, 10, "referral_reward", "Invite Reward for user "+newUserID, newUserID)`.

### Phase 3: Public API
- `POST /auth/register`: Update DTO to include `inviteCode`.
- `GET /user/profile`: Ensure `invite_code` is returned in the response.

## 4. Verification & Testing

### 4.1 Unit Tests
- Test `GenerateInviteCode` for length and charset.
- Test uniqueness collision handling (mocking).

### 4.2 Integration Tests
- **Scenario A (Normal Register)**: User registers without code -> Success, Level 0, Own code generated.
- **Scenario B (Invited Register)**:
  - User A exists.
  - User B registers with A's code.
  - Check B's DB record: `invited_by` == A.id, `level` == A.level + 1.
  - Check A's Credits: Increased by 10.
- **Scenario C (Invalid Code)**: Register with non-existent code -> Error.
- **Scenario D (Chain)**: A invites B, B invites C. Check levels (0, 1, 2). Verify A gets reward for B. Verify B gets reward for C. Verify A does *not* get reward for C.

## 5. Security & Limits
- Rate limiting on Registration endpoint prevents brute-forcing invite codes.
- `invite_code` index ensures fast lookups.
- Transactional credit updates ensure data integrity.

