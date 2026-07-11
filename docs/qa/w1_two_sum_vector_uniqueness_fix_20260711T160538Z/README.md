# W1 (R41-F review finding) — deterministic-correctness proof

**Finding**: `challenges/scripts/helixllm_coder_live_e2e_challenge.sh`'s [1/8]
freshly-randomised 4th `two_sum` test vector picked an answer pair `(a, b)`
with `a + b == target`, then filled the two remaining slots with independent
random fillers that were **not excluded** from also summing to `target` (with
each other, or with `a`/`b`). A genuinely-correct `two_sum` implementation
that returned that *other* valid pair would then fail the harness's index-set
assertion — a false-RED. The construction could never let bad code through
(false-PASS), so this was a robustness/flakiness defect, not a correctness
hole in the pass/fail boundary itself.

**Fix**: the vector is now built by a retry loop that only accepts a
candidate once an **independent brute-force check over all `C(4,2)=6` index
pairs** of the generated 4-element list confirms **exactly one** pair sums to
`target`. This also makes the generated vector honestly satisfy the
assumption already stated to the model in `USER_PROMPT` ("assume exactly one
valid pair exists").

**Proof method (no live coder needed)**: `verify_two_sum_vector_uniqueness.sh`
extracts the *live* `[1/8]` python body straight out of the challenge script
(via the unique `VECGEN_PYEOF` heredoc delimiter — zero drift, it always
checks whatever actually ships) and separately embeds the *historical*
(pre-fix, commit `125f73b`) body verbatim as regression-evidence data. Both
constructions are run N times as real `python3` subprocesses; every printed
vector is independently brute-force-checked (never trusting the generator's
own claims) for how many valid pairs it contains.

## Runs captured here

| Run | Construction under test | N | second-pair count | Verdict |
|---|---|---|---|---|
| `run1_RED_pre_fix_N1000/` | HISTORICAL (embedded) vs. CURRENT = **still pre-fix** (delimiter renamed only, logic unchanged) | 1000 | HISTORICAL: **12/1000** · CURRENT (pre-fix): **1/1000** | FAIL (expected — RED reproduction) |
| `run2_GREEN_post_fix_N1000/` | HISTORICAL (embedded) vs. CURRENT = **post-fix** | 1000 | HISTORICAL: **4/1000** (re-confirms the historical defect independently) · CURRENT (post-fix): **0/1000** | PASS |
| `run3_GREEN_post_fix_N1500_independent/` | Same as above, independent re-run at a different N | 1500 | HISTORICAL: **8/1500** · CURRENT (post-fix): **0/1500** | PASS |

Every count above is a real, generated-vector, brute-force-verified count —
none asserted from theory. `bash -n` on the target challenge script passed
in every run (see each run's `SUMMARY.txt`).

## Files per run

- `SUMMARY.txt` — verdict + machine-parseable counts
- `historical_sweep_RED.txt` / `current_sweep_GREEN.txt` — full sweep stdout
  (per-run `RESULT[...]` line + up to 5 concrete counter-example vectors +
  the `MACHINE[...]` trailer line consumed by the verifier's exit-code logic)
- `historical_vecgen_pre_fix_125f73b.py` — the embedded historical body
- `current_vecgen_extracted.py` — the body extracted from the live script at
  that run's time (15 lines pre-fix in run1; 49 lines post-fix in run2/run3)

## Reproduce

```bash
cd submodules/challenges/challenges/scripts
bash verify_two_sum_vector_uniqueness.sh 1000
```
