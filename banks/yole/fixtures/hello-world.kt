// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0
//
// HelixQA fixture: minimal Kotlin file for syntax-highlighting and autocomplete tests.
// Used by: feature-1-syntax-highlighting.yaml, feature-3-autocomplete.yaml

fun main() {
    val message = "Hello, Yole!"
    println(message)
}

class Greeter(private val name: String) {
    fun greet(): String = "Hello, $name!"
}
