# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: CC0-1.0
#
# HelixQA fixture: Python file with class + methods for Outline / document-symbol tests.
# Used by: feature-2-source-code-support.yaml


class Greeter:
    """A simple greeter class."""

    def __init__(self, name: str) -> None:
        self.name = name

    def greet(self) -> str:
        """Return a greeting string."""
        return f"Hello, {self.name}!"

    def farewell(self) -> str:
        """Return a farewell string."""
        return f"Goodbye, {self.name}!"


def main() -> None:
    g = Greeter("Yole")
    print(g.greet())
    print(g.farewell())
