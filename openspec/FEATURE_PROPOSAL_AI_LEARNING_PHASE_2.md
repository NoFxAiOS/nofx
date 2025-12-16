# Feature Proposal: AI Learning System - Phase 2 (Reflection Generator)

## 1. Context & Objectives
Building upon the data foundation (Phase 1), Phase 2 focuses on the "Brain" of the learning system: The **Reflection Generator**.
This component uses Large Language Models (LLMs) like DeepSeek to analyze statistical data and failure patterns, generating actionable advice ("Reflections").

## 2. Technical Architecture

### 2.1 Core Components (`decision/reflection`)

**1. `AIClient` Interface**
Decouples the logic from specific AI providers.
```go
type AIClient interface {
    GenerateCompletion(prompt string) (string, error)
}
```

**2. `ReflectionGenerator` Struct**
-   **Dependencies**: `AIClient`.
-   **Input**: `TradeAnalysisResult`, `[]FailurePattern`.
-   **Output**: `[]LearningReflection`.
-   **Logic**:
    -   Constructs a structured prompt containing the analysis data.
    -   Calls AI to diagnose root causes and suggest improvements.
    -   Parses AI response (JSON) into domain objects.

### 2.2 Data Flow
1.  `LearningCoordinator` (Phase 2.5) calls `TradeAnalyzer` & `PatternDetector`.
2.  It passes results to `ReflectionGenerator`.
3.  `ReflectionGenerator` constructs prompt -> Calls AI -> Returns Reflections.
4.  Reflections are saved to DB.

## 3. Implementation Plan

1.  **Definitions**: Create `decision/reflection/types.go` (Interfaces, Structs).
2.  **AI Integration**: Implement `decision/reflection/deepseek_client.go` (Simple client).
3.  **Generator Logic**: Implement `decision/reflection/reflection_generator.go` (Prompt engineering, parsing).
4.  **Testing**: Unit test with Mock AI Client.

## 4. Verification
-   **Unit Test**: Verify prompt construction and response parsing.
-   **Mock Test**: Ensure `GenerateReflections` handles AI errors gracefully.
