// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: CC0-1.0
//
// HelixQA fixture: Rust file with deliberate type mismatch for LSP diagnostics test.
// rust-analyzer will emit E0308 (mismatched types) on the return statement.
// Used by: feature-4b-diagnostics-hover-gotodef.yaml

fn foo() -> i32 {
    "not an int"  // E0308: mismatched types — expected i32, found &str
}

fn main() {
    let _result = foo();
}
