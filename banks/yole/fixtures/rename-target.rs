// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: CC0-1.0
//
// HelixQA fixture: Rust file with rename target for LSP refactoring test.
// 'old_name' appears at 2 sites (declaration + call site).
// Rename to 'new_name'; WorkspaceEdit must update both.
// Used by: feature-4c-refactoring.yaml

fn old_name() {
    // function body
}

fn main() {
    old_name();
}
