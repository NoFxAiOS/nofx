# Trader Management - Bug Fix Specification

## MODIFIED Requirements

### Requirement: Trader Creation Form State Stability
The system SHALL preserve user input in the trader creation form when the user interacts with dropdown selections, and SHALL only re-initialize form fields when the modal is newly opened or when switching between create and edit modes.

#### Scenario: User enters name then selects AI model
- **WHEN** user has entered a trader name in "交易员名称" field
- **AND** user clicks on "AI模型" dropdown to select a different model
- **THEN** the trader name input field SHALL retain the previously entered name (NOT be cleared)
- **AND** the AI model dropdown SHALL update to reflect the newly selected model

#### Scenario: Fresh modal opening initializes correctly
- **WHEN** user clicks "创建交易员" button to open the trader creation modal for the first time
- **AND** no previous form state exists
- **THEN** trader name field SHALL be empty
- **AND** AI model dropdown SHALL show the first enabled model
- **AND** Exchange dropdown SHALL show the first enabled exchange

#### Scenario: Form reinitializes when editing different trader
- **WHEN** user opens edit modal for one trader
- **AND** user closes the modal and opens edit modal for a different trader
- **THEN** the form fields SHALL be populated with the new trader's data
- **AND** previously edited data from the first trader SHALL NOT appear in the form

#### Scenario: Parent model updates don't clear user input
- **WHEN** user is creating a new trader in the modal
- **AND** parent component refreshes available models (due to user configuring a new model elsewhere)
- **AND** user has already entered trader name, selected AI model, and selected exchange
- **THEN** all user-entered data SHALL be preserved (NOT cleared or reset)
- **AND** form SHALL remain in the exact state the user left it

## IMPLEMENTATION NOTES

### Root Causes Addressed:
1. **Removed unstable prop dependencies**: `availableModels` and `availableExchanges` removed from useEffect dependency array, preventing re-initialization on every parent update
2. **Explicit open state**: Added `isOpen` to dependency array to explicitly track when modal is opened/closed
3. **Protected initialization**: Added guards to prevent unconditional field reset when user has already provided input
4. **Consolidated state management**: Ensured all form initialization happens through a single, predictable code path

### Key Changes:
- useEffect hook dependency array: from `[traderData, isEditMode, availableModels, availableExchanges]` to `[traderData, isEditMode, isOpen]`
- Added conditional check to only reset form when: (a) no traderData exists AND (b) modal is opening fresh
- Preserved user input by not resetting formData on parent prop changes

### Testing Focus:
- Form state preservation during dropdown interactions
- Correct initialization on fresh modal open
- Correct re-initialization when switching between create/edit modes
- No unintended re-initialization when parent component updates
