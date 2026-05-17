# Challenge: P1-F20 — Theme System

## Purpose

Prove HelixCode's Phase 1 / Feature 20 theme system actually renders the
real spec 3.4 byte tables for built-in themes, honours the zero-color
invariant for `DepthOff`, resolves `DetectColorDepth` correctly across
the six canonical env-state branches, and merges a real on-disk YAML
override over the dark baseline producing a full 5-role custom theme.

Per Article XI 11.9, every PASS must carry positive runtime evidence
captured during execution. The harness emits **byte-level captures**
of the dark and light truecolor opens (printed in hex), a
**zero-ESC-byte invariant** for the plain-mode path, **per-branch
depth assertions** for env-driven detection, and a **5/5 role count
plus byte-equality inheritance check** for the YAML merge phase.

The harness wires together the F20 surface area:

- `theme.Theme` / `theme.Role` / `theme.Color` / `theme.ColorDepth`
  (T02) — validated input/output types + sentinels + `Reset` const.
- `theme.BuiltinDarkTheme` / `theme.BuiltinLightTheme` /
  `theme.BuiltinNoneTheme` (T03) — pinned byte tables.
- `theme.DetectColorDepth` (T04) — pure env-driven resolver.
- `theme.NewThemeRegistry` / `theme.ThemeRegistry.LoadFromFile` /
  `theme.ThemeRegistry.Custom` / `theme.NewStyler` (T05) — registry
  + real-YAML loader + Styler decorator.

Every always-runs phase is hermetic: no real shell, no real OS env
beyond `tempdir`, no real terminal — every signal fed to detection
or styling is supplied by the harness.

## Procedure

1. Build the F20 challenge harness from
   `helix_code/tests/integration/cmd/p1f20_challenge`.
2. Run the harness; it executes five phases:

   a. **Phase A — BUILT-IN-DARK (always runs).** Construct
      `theme.NewThemeRegistry()` and `Get(ThemeDark)`. Assert the
      returned theme's `Name == ThemeDark`. Wrap it in
      `NewStyler(theme, DepthTruecolor)`. For each canonical role
      (info/warn/error/highlight/dim) call `Stylize(role, "X")` and
      assert the bytes are exactly `Open + "X" + Reset` where `Open`
      matches spec 3.4 dark row (e.g., error =
      `\x1b[38;2;255;64;64m`). Assert the trailing `\x1b[0m` Reset is
      present. Print evidence per role: hex of the open sequence and
      total byte count.
   b. **Phase B — BUILT-IN-LIGHT (always runs).** Same as Phase A
      against `ThemeLight`. Bytes are pinned to spec 3.4 light row;
      light error specifically is `\x1b[38;2;175;0;0m`. Cross-theme
      sanity: assert light error open does NOT equal dark error open
      (load-bearing distinguisher — a regression that swapped the
      built-in tables would survive a single-theme assertion but
      fails this one).
   c. **Phase C — PLAIN-ZERO-COLOR (always runs, load-bearing).**
      Construct `NewStyler(dark, DepthOff)`. For every role assert
      `Stylize(role, "X") == "X"` byte-equal AND that the output
      contains zero `\x1b` bytes. Print a single line confirming all
      5 roles returned plain text.
   d. **Phase D — DEPTH-DETECT (always runs).** Six synthesised
      `envLookup` closures fed to `DetectColorDepth`:
      - `NO_COLOR=1` + `COLORTERM=truecolor` + `TERM=xterm-256color`
        → `DepthOff` (NO_COLOR overrides everything).
      - `COLORTERM=truecolor` + `TERM=xterm-256color` →
        `DepthTruecolor`.
      - `TERM=xterm-256color` (no COLORTERM) → `DepthANSI256`.
      - `TERM=xterm` → `DepthANSI16`.
      - `TERM=dumb` → `DepthOff`.
      - All unset → `DepthOff`.
   e. **Phase E — YAML-MERGE (always runs).** Create a tempdir; write
      a `theme.yaml` overriding only `error` with truecolor
      `\x1b[38;2;255;0;255m` and ansi256 `\x1b[38;5;201m`. Construct
      a registry, call `LoadFromFile(path)`. Retrieve the custom
      theme via `Custom()`. Assert: `Name == "my-custom"`,
      `len(Colors) == 5`, the error role's `OpenTruecolor` and
      `OpenANSI256` exactly equal the YAML override bytes, and the
      info/warn/highlight/dim roles are byte-equal to the
      `BuiltinDarkTheme()` baseline (proves real merge, not silent
      drop). Cross-check: the merged error truecolor MUST NOT equal
      the dark error truecolor — if it did, the override silently
      collapsed.
3. Anti-bluff smoke clean over the harness file + this CHALLENGE.md
   + run.sh (the smoke regex is built from string fragments so the
   script does not match itself).
4. Cross-compile linux/amd64 clean.

## Pass criteria

- Harness exits 0 with `==> P1-F20 challenge harness PASS` final line.
- **Phase A**: each of the 5 roles renders to `Open + "X" + Reset`
  where `Open` matches the dark truecolor row of spec 3.4
  byte-for-byte; harness prints the hex of each open sequence.
- **Phase B**: same shape against `ThemeLight`; light-error open
  bytes differ from dark-error open bytes.
- **Phase C**: all 5 roles return `"X"` byte-equal under
  `DepthOff`; output contains zero `\x1b` bytes.
- **Phase D**: all six DetectColorDepth branches return the
  spec-mandated depth.
- **Phase E**: custom theme has `Name == "my-custom"` and exactly
  5 roles, error matches the YAML override, the other four roles
  are byte-equal to `BuiltinDarkTheme()`, and the merged error
  truecolor differs from the dark error truecolor.
- Anti-bluff smoke clean over harness file + this CHALLENGE.md +
  run.sh (regex built from fragments).
- Cross-compile linux/amd64 clean.

## Anti-bluff anchors

- **Phase A & B — pinned byte tables.** A regression that
  paraphrased a color (e.g., wrote `255;65;64` instead of
  `255;64;64`) would compile and pass a "looks colorful" eyeball
  check, but the byte-equality assertion in this harness rejects it
  outright. The hex-printed open sequences are positive runtime
  evidence (the load-bearing equivalent of F18's byte-count and
  F19's byte-offset evidence), not a metadata flag.
- **Phase B cross-theme distinguisher.** Asserting light error
  bytes !== dark error bytes guards against the failure mode where
  both built-ins share a single underlying table. The dark/light
  divide would silently disappear without surfacing in single-theme
  tests.
- **Phase C zero-ANSI invariant.** A regression that "almost"
  honoured `DepthOff` (e.g., emitted only the open without the
  reset, or vice versa) would still inject an ESC byte. The
  `ContainsRune(out, 0x1b)` assertion catches every form of partial
  emission. This is the load-bearing invariant for users with
  `NO_COLOR=1` or `TERM=dumb`.
- **Phase D coverage.** Six branches cover the cartesian product of
  the three signals DetectColorDepth consults (NO_COLOR, COLORTERM,
  TERM). A regression that re-ordered the layers (e.g., checked
  COLORTERM before NO_COLOR) trips the first case.
- **Phase E real-YAML / real-merge.** The harness writes a real
  file to a real tempdir and lets `gopkg.in/yaml.v3` parse it
  through `LoadFromFile`. A regression that short-circuited the
  loader would either fail to populate Custom() (caught by the
  nil-check) or skip the merge (caught by the 5/5 role-count and
  byte-equality assertions). The "merged error must differ from
  dark error" check guards against the failure mode where the
  override key was read but discarded.
