# Fork & Branch Sync Guide (tommy-ca)

This guide documents how to work with:

- **Upstream repo**: the original NOFX repository.
- **Fork**: your personal fork under `tommy-ca/nofx`.
- **Feature branches**: e.g. `feature/specs-sync`.

Adjust names if you use a different GitHub account or branch name.

---

## 1. Remote Layout

Recommended remote configuration:

- `origin` → upstream (original) repository  
  e.g. `https://github.com/tinkle-community/nofx`
- `tommy-ca` → your fork  
  e.g. `https://github.com/tommy-ca/nofx.git`

You can check remotes with:

```bash
git remote -v
```

To set this up (once):

```bash
# If origin currently points to upstream, keep it:
git remote set-url origin https://github.com/tinkle-community/nofx

# Add your fork as a separate remote
git remote add tommy-ca https://github.com/tommy-ca/nofx.git
```

If you prefer to have the fork as `origin` and upstream as `upstream`, you can:

```bash
git remote rename origin upstream
git remote add origin https://github.com/tommy-ca/nofx.git
```

Pick one layout and stick with it consistently.

---

## 2. Fork Creation (GitHub UI)

In the browser:

1. Navigate to the upstream repo (original NOFX).
2. Click **Fork**.
3. Choose your account **tommy-ca**.
4. Accept defaults so the fork is created as:
   - `https://github.com/tommy-ca/nofx`

After this, the `tommy-ca` remote in your local repo will point to a real repository.

---

## 3. Creating & Pushing a Feature Branch

Example: `feature/specs-sync`.

1. Make sure `main` is up to date with upstream:

```bash
git checkout main
git pull origin main        # or git pull upstream main if using upstream/origin layout
```

2. Create the feature branch:

```bash
git checkout -b feature/specs-sync
```

3. Commit your changes locally as usual:

```bash
git status
git add ...
git commit -m "Describe your changes"
```

4. Push the branch to your fork:

```bash
git push -u tommy-ca feature/specs-sync
```

Now the branch exists at `tommy-ca/nofx:feature/specs-sync` and is tracked by your local branch.

---

## 4. Keeping in Sync with Upstream

To pull new changes from upstream and keep both your fork and feature branch up to date:

1. Update local `main` from upstream:

```bash
git checkout main
git pull origin main        # if origin is upstream
# or: git pull upstream main  # if upstream is the original repo
```

2. Push updated `main` to your fork:

```bash
git push tommy-ca main
```

3. Rebase or merge your feature branch onto updated `main`:

```bash
git checkout feature/specs-sync
# Option A: merge
git merge main

# Option B: rebase (keeps history linear)
# git rebase main
```

4. Push updated feature branch to your fork:

```bash
git push tommy-ca feature/specs-sync
# If you rebased:
# git push --force-with-lease tommy-ca feature/specs-sync
```

---

## 5. Opening Pull Requests

With `feature/specs-sync` pushed to your fork, you can:

- Open a PR **within your fork**:
  - `tommy-ca/nofx:feature/specs-sync` → `tommy-ca/nofx:main`
  - Useful for personal review/checkpoints.

- Or open a PR **to upstream** (when ready to contribute back):
  - `tommy-ca:feature/specs-sync` → `upstream:main` (original NOFX repo).

Use the repo’s pull request template to verify:

- Specs (`docs/specs/*`) are updated for any behaviour changes.
- Design docs (`docs/design/*`) reflect architecture changes.
- Tests and security checks are in place.

---

## 6. Quick Command Summary

**Initial setup:**

```bash
git remote add tommy-ca https://github.com/tommy-ca/nofx.git
```

**Create & push feature branch:**

```bash
git checkout main
git pull origin main
git checkout -b feature/specs-sync
git push -u tommy-ca feature/specs-sync
```

**Sync with upstream later:**

```bash
git checkout main
git pull origin main
git push tommy-ca main

git checkout feature/specs-sync
git merge main           # or git rebase main
git push tommy-ca feature/specs-sync
```

This keeps your fork (`tommy-ca/nofx`) and your feature branches aligned with the upstream project.

