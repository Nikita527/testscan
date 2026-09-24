#!/usr/bin/env python3
"""Dump a minimal test-function model as one JSON object on stdout."""
from __future__ import annotations

import ast
import json
import sys


def _is_test_name(name: str) -> bool:
    if name.startswith("test_"):
        return True
    # TestFoo — да; Testimony / Test — нет
    return (
        len(name) >= 5
        and name.startswith("Test")
        and name[4].isupper()
    )


def _is_docstring_expr(stmt: ast.stmt) -> bool:
    if not isinstance(stmt, ast.Expr):
        return False
    val = stmt.value
    if isinstance(val, ast.Constant) and isinstance(val.value, str):
        return True
    # Python < 3.8
    return isinstance(val, getattr(ast, "Str", ()))


def _body_is_empty(body: list[ast.stmt]) -> bool:
    for stmt in body:
        if isinstance(stmt, ast.Pass):
            continue
        if _is_docstring_expr(stmt):
            continue
        return False
    return True


def _has_assert(fn: ast.AST) -> bool:
    for node in ast.walk(fn):
        if isinstance(node, ast.Assert):
            return True
    return False


def _call_name(node: ast.AST) -> str:
    if isinstance(node, ast.Name):
        return node.id
    if isinstance(node, ast.Attribute):
        base = _call_name(node.value)
        if base:
            return f"{base}.{node.attr}"
        return node.attr
    return ""


def _has_raises(fn: ast.AST) -> bool:
    for node in ast.walk(fn):
        if not isinstance(node, ast.Call):
            continue
        name = _call_name(node.func)
        if name in (
            "pytest.raises",
            "pytest.warns",
            "unittest.TestCase.assertRaises",
            "self.assertRaises",
            "assertRaises",
        ):
            return True
    return False


def _collect(tree: ast.AST) -> list[dict]:
    tests: list[dict] = []
    for node in ast.walk(tree):
        if not isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            continue
        if not _is_test_name(node.name):
            continue
        end = getattr(node, "end_lineno", None) or node.lineno
        tests.append(
            {
                "name": node.name,
                "lineno": node.lineno,
                "end_lineno": end,
                "is_empty": _body_is_empty(node.body),
                "has_assert": _has_assert(node),
                "has_raises": _has_raises(node),
            }
        )
    return tests


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: ast_dump.py <file.py>", file=sys.stderr)
        return 2
    path = sys.argv[1]
    try:
        with open(path, encoding="utf-8") as f:
            src = f.read()
        tree = ast.parse(src, filename=path)
    except Exception as exc:  # noqa: BLE001 — surface to Go as non-zero exit
        print(f"ast_dump: {exc}", file=sys.stderr)
        return 1
    json.dump({"tests": _collect(tree)}, sys.stdout, ensure_ascii=False)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
