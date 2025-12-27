## MODIFIED Requirements

### Requirement: Credits Data Loading State Management
The useUserCredits Hook SHALL properly manage loading state throughout all execution paths, ensuring the component transitions correctly between loading, success, and error states.

#### Scenario: Loading state on authentication failure
- **WHEN** API returns 401 (unauthorized) response
- **THEN** loading state is set to false
- **AND** credits data is cleared
- **AND** no error message is displayed

#### Scenario: Loading state on successful fetch
- **WHEN** API returns 200 with valid credits data
- **THEN** loading state is set to false
- **AND** credits data is populated
- **AND** component displays the available credits value

#### Scenario: Loading state on API error
- **WHEN** API returns non-200, non-401 status
- **THEN** loading state is set to false
- **AND** error state is set with error details
- **AND** credits data is cleared

---

### Requirement: API Response Data Validation
The useUserCredits Hook SHALL validate the structure of API responses before using them, ensuring data integrity and providing meaningful error messages.

#### Scenario: Valid credits data response
- **WHEN** API returns valid JSON with required fields (available, total, used)
- **THEN** data is validated successfully
- **AND** each field is verified as a number type
- **AND** credits state is updated with the validated data

#### Scenario: Invalid response format
- **WHEN** API returns non-object response (null, string, array)
- **THEN** error state is set with message "API响应数据格式错误: 期望对象"
- **AND** credits data is cleared
- **AND** loading state is set to false

#### Scenario: Missing required fields
- **WHEN** API returns object missing available/total/used fields
- **THEN** error state is set with message "API响应数据格式错误: 缺少必要字段"
- **AND** credits data is cleared
- **AND** loading state is set to false

#### Scenario: Invalid field types
- **WHEN** API returns fields with non-number types (strings, objects)
- **THEN** error state is set with data type validation error
- **AND** credits data is cleared
- **AND** loading state is set to false

---

### Requirement: Credits Display UI Error Handling
The CreditsDisplay component SHALL provide clear feedback when credits data cannot be loaded, with improved error visibility and state handling.

#### Scenario: Display error state
- **WHEN** Hook returns error state
- **THEN** component displays warning icon "⚠️"
- **AND** title attribute shows "积分加载失败，请刷新页面"
- **AND** component has role="status" for accessibility
- **AND** data-testid="credits-error" for testing

#### Scenario: Display loading state
- **WHEN** Hook returns loading=true and no credits data
- **THEN** component displays skeleton screen (骨架屏)
- **AND** data-testid="credits-loading" for testing

#### Scenario: Display valid credits
- **WHEN** Hook returns valid credits data
- **THEN** component displays CreditsIcon and CreditsValue
- **AND** aria-label shows "Available credits: {available}"
- **AND** displays the available credits numeric value

---

### Requirement: Auto-refresh Credits Data
The useUserCredits Hook SHALL automatically refresh credits data at regular intervals to keep displayed values current.

#### Scenario: Initial fetch on mount
- **WHEN** component mounts and user is authenticated
- **THEN** credits data is fetched immediately
- **AND** loading state is true during fetch
- **AND** data displays when fetch completes

#### Scenario: Auto-refresh interval
- **WHEN** 30 seconds have elapsed since last fetch
- **THEN** credits data is automatically refetched
- **AND** displayed value updates if changed
- **AND** loading state does not change

#### Scenario: Cleanup on unmount
- **WHEN** component unmounts
- **THEN** auto-refresh interval is cleared
- **AND** pending fetch request is ignored
- **AND** no memory leaks occur
