# Challenge: P1-F10 — Skill System

## Purpose

Prove the project's agent-invoked Skill system actually loads .helix/skills/*.md
files, compiles named-capture regex triggers, matches user input, renders
bodies with substituted captures, hot-reloads on file edit, and unregisters
on deletion. Per Article XI §11.9, every PASS must carry positive runtime
evidence.

## Procedure

1. Build the F10 challenge harness.
2. Run the harness — it:
   a. Writes refactor.md with named-capture trigger
   b. Loads via SkillLoader
   c. Calls SkillDispatcher.Match on "refactor LoginButton component" — captures comp=LoginButton, renders "Refactoring LoginButton"
   d. Mutates the file, reloads, re-matches with new arg "MainNav" — renders "Now: MainNav"
   e. Removes file, reloads, asserts no match
3. Anti-bluff smoke clean
4. Cross-compile linux clean

## Pass criteria

- Harness exits 0 with `==> P1-F10 challenge harness PASS` final line
- All 4 steps produce real captured output
- Anti-bluff smoke clean
- Cross-compile linux clean
