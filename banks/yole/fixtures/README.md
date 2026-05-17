# HelixQA Test Fixtures

## Fixtures in this directory

| File | Used by | Description |
|------|---------|-------------|
| `hello-world.kt` | feature-1, feature-3 | Kotlin file for syntax-highlight + autocomplete tests |
| `sample-class.py` | feature-2 | Python file with class + methods for Outline test |
| `hello-world.rs` | feature-4a | Rust file for LSP completion test |
| `type-error.rs` | feature-4b | Rust file with E0308 type error for diagnostics/hover/gotodef |
| `rename-target.rs` | feature-4c | Rust file with `old_name` at 2 sites for rename refactoring |
| `test-import.docx` | feature-5 | .docx import fixture — see note below |

## test-import.docx

**Tracker: #iter-76-import-fixture-docx-committed**

`test-import.docx` is a binary fixture that must be committed to this directory.
It contains:
- Heading 1: "Hello Import"
- Paragraph: "This is a test paragraph."

To generate it on any machine with LibreOffice (or Python-docx):

```bash
# Python-docx method (pip install python-docx)
python3 - <<'EOF'
from docx import Document
d = Document()
d.add_heading("Hello Import", level=1)
d.add_paragraph("This is a test paragraph.")
d.save("Challenges/banks/yole/fixtures/test-import.docx")
EOF
```

**Status as of iter-76:** deferred — the file must be generated and committed on a
machine with python-docx or LibreOffice before feature-5-import.yaml can run.
The scenario is authored and ready; only the binary fixture is pending.
