# Feature Proposal: AI Learning System - Phase 3 & 4

## 1. Context & Objectives
Having established the data foundation (Phase 1) and the AI "Brain" (Phase 2), we now move to the user interface and automatic execution.
-   **Phase 3**: Visualize the learning process so users can trust and monitor the AI.
-   **Phase 4**: Close the loop by automatically applying high-confidence improvements.

## 2. Phase 3: Frontend Dashboard (Visualization)

### 2.1 Design Principles
-   **Transparency**: Clearly show *why* a suggestion is made (Root Cause).
-   **Actionability**: "Apply" buttons for manual interventions.
-   **Simplicity**: Clean cards for each reflection.

### 2.2 Components
1.  **`LearningDashboard`**: Main container.
    -   Fetches data from `GET /api/traders/:id/analysis` and `GET /api/traders/:id/reflections`.
    -   Displays `TradeStatsPanel` (Top) and `ReflectionsList` (Bottom).
2.  **`TradeStatsPanel`**:
    -   Visualizes Win Rate, Profit Factor, and "Best Trading Hour" using simple charts or stat cards.
3.  **`ReflectionCard`**:
    -   Displays a single `LearningReflection`.
    -   Color-coded severity (Red/Orange/Green).
    -   **Interactive**: "Apply" button (calls API) if not applied. "Applied" badge if done.

## 3. Phase 4: Reflection Executor (Automation)

### 3.1 Technical Architecture (`decision/learning`)

**1. `ReflectionExecutor` Struct**
-   **Responsibility**: Takes a `LearningReflection` and executes the `RecommendedAction`.
-   **Input**: `LearningReflection`.
-   **Output**: `Result` (Success/Fail), updates `is_applied` status.

**2. `ParameterOptimizer` Struct**
-   **Responsibility**: The "Hand" that actually modifies the trader's config.
-   **Supported Actions**:
    -   `ADJUST_LEVERAGE`: Update `btc_eth_leverage` or `altcoin_leverage`.
    -   `UPDATE_PROMPT`: Append instruction to `custom_prompt`.
    -   `STOP_TRADING`: If severity is CRITICAL.

### 3.2 Automation Logic
-   **Trigger**: `LearningCoordinator` (from Phase 2 design) calls Executor after generating reflections.
-   **Threshold**: Only execute if `Priority >= 8` (High Confidence).
-   **Safety**:
    -   Leverage cannot exceed system max (e.g., 50x).
    -   Cannot stop trading if already stopped.

### 3.3 Data Flow
1.  `ReflectionGenerator` -> `Reflection` (Priority 9).
2.  `LearningCoordinator` -> Checks Priority -> Calls `ReflectionExecutor`.
3.  `ReflectionExecutor` -> Calls `ParameterOptimizer` -> Updates `traders` table.
4.  `ReflectionExecutor` -> Records change in `parameter_change_history` table.
5.  `ReflectionExecutor` -> Updates `learning_reflections.is_applied = true`.

## 4. Testing Strategy

### 4.1 Executor Tests
-   **Unit Test**: Mock `ParameterOptimizer` and verify `ReflectionExecutor` calls it correctly for different action types.
-   **Safety Test**: Verify boundaries (e.g. leverage limits) are respected.

### 4.2 Frontend Tests
-   **Component Test**: Verify `ReflectionCard` renders correct colors and text based on props.
-   **Interaction Test**: Verify "Apply" button triggers the correct API call.

## 5. Implementation Plan
1.  **Backend**: Implement `decision/learning/parameter_optimizer.go` and `reflection_executor.go`.
2.  **Backend**: Update `LearningHandler` to support `POST /apply` (for manual frontend trigger).
3.  **Frontend**: Implement React components.
