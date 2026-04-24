package agent

const skillDAGStepField = "_dag_step"

func currentSkillDAGStep(session skillSession) (SkillDAGStep, bool) {
	dag, ok := getSkillDAG(session.Name, session.Action)
	if !ok || len(dag.Steps) == 0 {
		return SkillDAGStep{}, false
	}
	stepID := fieldValue(session, skillDAGStepField)
	if stepID == "" {
		return dag.Steps[0], true
	}
	for _, step := range dag.Steps {
		if step.ID == stepID {
			return step, true
		}
	}
	return dag.Steps[0], true
}

func setSkillDAGStep(session *skillSession, stepID string) {
	ensureSkillFields(session)
	if stepID == "" {
		delete(session.Fields, skillDAGStepField)
		return
	}
	session.Fields[skillDAGStepField] = stepID
}

func clearSkillDAGStep(session *skillSession) {
	if session == nil || session.Fields == nil {
		return
	}
	delete(session.Fields, skillDAGStepField)
}

func advanceSkillDAGStep(session *skillSession, currentStepID string) {
	dag, ok := getSkillDAG(session.Name, session.Action)
	if !ok {
		return
	}
	for _, step := range dag.Steps {
		if step.ID != currentStepID || len(step.Next) == 0 {
			continue
		}
		setSkillDAGStep(session, step.Next[0])
		return
	}
}

