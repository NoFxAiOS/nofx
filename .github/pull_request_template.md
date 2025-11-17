# Pull Request Checklist

Please use this checklist to keep specs and implementation in sync.

- [ ] **Scope is clear**
  - [ ] PR title and description explain *what* and *why*.

- [ ] **Specs updated (if behaviour changes)**
  - [ ] `docs/specs/requirements.md` updated when product behaviour or user journeys change.
  - [ ] `docs/specs/api-spec.md` updated when HTTP endpoints, paths, or payloads change.
  - [ ] `docs/specs/trading-engine.md` updated when trading loop or risk logic changes.
  - [ ] `docs/specs/data-model.md` updated when DB schema or persistence behaviour changes.
  - [ ] `docs/design/*` updated if architecture or subsystem design changes.

- [ ] **Code matches specs**
  - [ ] API handlers and frontend client (`web/src/lib/api.ts`) align with documented endpoints.
  - [ ] Risk and trading behaviour in `trader/`, `decision/`, `market/`, and `manager/` matches specs.

- [ ] **Tests & validation**
  - [ ] Existing tests pass locally (where applicable).
  - [ ] New tests added or updated for critical behavioural changes (API, trading engine, DB migrations).

- [ ] **Security / safety**
  - [ ] No secrets, keys, or credentials committed.
  - [ ] Changes to auth, encryption, or beta access are described in the PR.

Add any extra context, screenshots, or migration notes below:

---

**Additional Notes:**

