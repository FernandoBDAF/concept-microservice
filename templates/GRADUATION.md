# Template graduation runbook (ADR-010.3)

When a real project adopts a template, the template graduates to its own
repo. Procedure (draft — refine on first real graduation):

1. **Trigger:** first real adoption (not speculation). The adopting
   project's copy is already diverging — graduate the *template*, not the
   copy.
2. **History:** `git subtree split -P templates/<piece> -b split/<piece>`
   → push to the new repo (keeps the template's lab history; the lab keeps
   its copy until the next major lab change makes it stale).
3. **Versioning:** new repo tags `v0.1.0` at split; still copy-then-own —
   tags mark "known-good snapshots to copy", not a supported API.
4. **Back-pointer:** lab's `templates/<piece>/README.md` gains a banner
   (graduated → repo URL + split date); lab CI drops that piece's
   bootstrap test.
5. **Feedback flow:** hardening discovered by adopters lands in the new
   repo; if it's a *pattern* fix (not project-specific), port it back to
   the lab source service AND run the experiment that proves it there —
   the lab stays the proving ground.
6. Record the graduation in `documentation/experiments/` (date, adopter,
   friction notes — EXP-80-style honesty).
