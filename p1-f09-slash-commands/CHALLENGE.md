# Challenge: P1-F09 — Slash Command System

## Purpose

Prove the project's user-defined Markdown slash commands actually load real `.md`
files, perform real variable substitution, hot-reload on file edit, and
unregister on file deletion. Per Article XI §11.9, every PASS must carry
positive runtime evidence.

## Procedure

1. Build the F09 challenge harness.
2. Run the harness — it:
   a. Writes a real `echo.md` with body `Got: {{ARG1}}`
   b. Loads via MarkdownLoader, asserts `echo` is in the registry
   c. Runs `echo` with arg `hello world`, captures `Got: hello world`
   d. Mutates the file to `New: {{ARG1}}`, reloads, runs again, captures `New: second-run`
   e. Removes the file, reloads, asserts registry no longer has `echo`
3. Anti-bluff smoke clean
4. Cross-compile linux clean

## Pass criteria

- Harness exits 0 with `==> P1-F09 challenge harness PASS` final line
- All 5 steps produce real captured output (load -> render -> reload-render -> unregister)
- Anti-bluff smoke clean
- Cross-compile linux clean
