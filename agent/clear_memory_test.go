package agent

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"nofx/store"
)

func TestClearRemovesActiveAndPendingConversationState(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "agent-clear.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	a := New(nil, st, DefaultConfig(), slog.Default())
	userID := int64(42)

	a.history.Add(userID, "assistant", "之前的回复")
	_ = a.saveTaskState(userID, TaskState{CurrentGoal: "配置模型"})
	a.saveActiveSkillSession(ActiveSkillSession{
		SessionID:  "as_test",
		UserID:     userID,
		SkillName:  "model_management",
		ActionName: "create",
		PendingHint: &PendingHint{
			Prompt:   "请选择 provider",
			HintType: "question",
		},
	})
	a.savePendingProposalSession(PendingProposalSession{
		UserID:         userID,
		SourceUserText: "帮我配置模型",
		ProposalText:   "推荐 claw402，你要继续吗？",
	})
	a.saveSetupState(userID, &SetupState{
		Step:       "await_ai_model",
		AIProvider: "claw402",
	})
	if err := st.SetSystemConfig(skillSessionConfigKey(userID), `{"name":"model_management","action":"create"}`); err != nil {
		t.Fatalf("seed skill session: %v", err)
	}
	a.saveWorkflowSession(userID, WorkflowSession{
		Tasks: []WorkflowTask{{
			ID:      "task_1",
			Skill:   "model_management",
			Action:  "create",
			Request: "帮我配置模型",
			Status:  workflowTaskPending,
		}},
	})
	if err := st.SetSystemConfig(ExecutionStateConfigKey(userID), `{"user_id":42,"session_id":"exec_1"}`); err != nil {
		t.Fatalf("seed execution state: %v", err)
	}
	a.saveReferenceMemory(userID, &CurrentReferences{
		Model: &EntityReference{ID: "m1", Name: "claw402", Source: "context"},
	}, nil)
	a.SnapshotManager(userID).Save(SuspendedTask{ResumeHint: "旧任务"})

	reply, err := a.HandleMessage(context.Background(), userID, "/clear")
	if err != nil {
		t.Fatalf("clear returned error: %v", err)
	}
	if reply == "" {
		t.Fatalf("expected clear reply")
	}

	if got := a.history.Get(userID); len(got) != 0 {
		t.Fatalf("history not cleared: %+v", got)
	}
	if got := a.buildRecentConversationContext(userID, "你好"); got != "" {
		t.Fatalf("recent conversation context not cleared: %q", got)
	}
	if got := a.currentPendingHintText(userID); got != "" {
		t.Fatalf("pending hint not cleared: %q", got)
	}
	if got := a.buildCurrentTurnContext(userID, "zh", "你好"); got != "" {
		if strings.Contains(got, "Previous assistant reply:") || strings.Contains(got, "Recent conversation:") {
			t.Fatalf("current turn context still contains prior chat memory: %q", got)
		}
	}
	if got := a.buildActiveTaskStateContext(userID, "zh"); got != "" {
		t.Fatalf("active task state context not cleared: %q", got)
	}
	if state := a.getTaskState(userID); state.CurrentGoal != "" || state.ActiveFlow != "" {
		t.Fatalf("task state not cleared: %+v", state)
	}
	if _, ok := a.getActiveSkillSession(userID); ok {
		t.Fatalf("active skill session not cleared")
	}
	if _, ok := a.getPendingProposalSession(userID); ok {
		t.Fatalf("pending proposal session not cleared")
	}
	if session := a.getSkillSession(userID); session.Name != "" {
		t.Fatalf("legacy skill session not cleared: %+v", session)
	}
	if session := a.getWorkflowSession(userID); len(session.Tasks) != 0 {
		t.Fatalf("workflow session not cleared: %+v", session)
	}
	if state := a.getExecutionState(userID); state.SessionID != "" {
		t.Fatalf("execution state not cleared: %+v", state)
	}
	if memory := a.getReferenceMemory(userID); memory.CurrentReferences != nil || len(memory.ReferenceHistory) != 0 {
		t.Fatalf("reference memory not cleared: %+v", memory)
	}
	if stack := a.SnapshotManager(userID).List(); len(stack) != 0 {
		t.Fatalf("snapshots not cleared: %+v", stack)
	}
	if setup := a.getSetupState(userID); setup.Step != "" || setup.AIProvider != "" {
		t.Fatalf("setup state not cleared: %+v", setup)
	}
}
