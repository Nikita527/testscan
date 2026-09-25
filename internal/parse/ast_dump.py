#!/usr/bin/env python3
"""Dump a per-test AST model as JSON (single file) or JSONL (batch stdin)."""
from __future__ import annotations

import ast
import fnmatch
import json
import sys
from typing import Any

_FUNC_PATTERNS: list[str] = ["test_*"]
_CLASS_PATTERNS: list[str] = ["Test*"]


def configure_patterns(functions: list[str] | None, classes: list[str] | None) -> None:
    """Set discovery globs. Empty list resets to pytest defaults; None leaves unchanged."""
    global _FUNC_PATTERNS, _CLASS_PATTERNS
    if functions is not None:
        _FUNC_PATTERNS = list(functions) if functions else ["test_*"]
    if classes is not None:
        _CLASS_PATTERNS = list(classes) if classes else ["Test*"]


def _is_test_func(name: str) -> bool:
    return any(fnmatch.fnmatchcase(name, p) for p in _FUNC_PATTERNS)


def _is_test_class(name: str) -> bool:
    # Default Test*: TestFoo — yes; Testimony / Test — no (pytest-like).
    if _CLASS_PATTERNS == ["Test*"]:
        return len(name) >= 5 and name.startswith("Test") and name[4].isupper()
    return any(fnmatch.fnmatchcase(name, p) for p in _CLASS_PATTERNS)


def _is_docstring_expr(stmt: ast.stmt) -> bool:
    if not isinstance(stmt, ast.Expr):
        return False
    val = stmt.value
    if isinstance(val, ast.Constant) and isinstance(val.value, str):
        return True
    return isinstance(val, getattr(ast, "Str", ()))


def _body_is_empty(body: list[ast.stmt]) -> bool:
    for stmt in body:
        if isinstance(stmt, ast.Pass):
            continue
        if _is_docstring_expr(stmt):
            continue
        return False
    return True


def _call_name(node: ast.AST) -> str:
    if isinstance(node, ast.Name):
        return node.id
    if isinstance(node, ast.Attribute):
        base = _call_name(node.value)
        if base:
            return f"{base}.{node.attr}"
        return node.attr
    return ""


def _decorator_name(dec: ast.expr) -> str:
    # Prefer full unparse so parametrize ids/args are visible to rules.
    text = _unparse(dec)
    if text:
        return text
    if isinstance(dec, ast.Call):
        return _call_name(dec.func)
    return _call_name(dec)


def _unparse(node: ast.AST) -> str:
    try:
        return ast.unparse(node)
    except Exception:  # noqa: BLE001
        return ""


def _is_call_expr(node: ast.AST) -> bool:
    return isinstance(node, ast.Call)


def _raise_exc_name(call: ast.Call) -> str:
    if not call.args:
        return ""
    return _unparse(call.args[0])


def _has_match_kw(call: ast.Call) -> bool:
    for kw in call.keywords:
        if kw.arg == "match":
            return True
    return False


def _is_raises_call(name: str) -> bool:
    return name in (
        "pytest.raises",
        "pytest.warns",
        "unittest.TestCase.assertRaises",
        "self.assertRaises",
        "assertRaises",
    )


def _is_mock_assert_name(name: str) -> bool:
    leaf = name.rsplit(".", 1)[-1]
    return leaf.startswith("assert_called") or leaf in (
        "assert_has_calls",
        "assert_any_call",
        "assert_not_called",
    )


def _is_unittest_bool_name(name: str) -> bool:
    leaf = name.rsplit(".", 1)[-1]
    return leaf in ("assertTrue", "assertFalse")


def _stmt_is_assert_like(stmt: ast.stmt) -> bool:
    if isinstance(stmt, ast.Assert):
        return True
    if isinstance(stmt, ast.Expr) and isinstance(stmt.value, ast.Call):
        return _is_mock_assert_name(_call_name(stmt.value.func))
    return False


def _body_only_asserts(body: list[ast.stmt]) -> bool:
    saw = False
    for stmt in body:
        if _is_docstring_expr(stmt):
            continue
        if isinstance(stmt, ast.Pass):
            continue
        if not _stmt_is_assert_like(stmt):
            return False
        saw = True
    return saw


class _NormLiterals(ast.NodeTransformer):
    def visit_Constant(self, node: ast.Constant) -> ast.AST:
        self.generic_visit(node)
        if isinstance(node.value, str):
            return ast.copy_location(ast.Constant(value="STR"), node)
        if isinstance(node.value, bool):
            return node
        if isinstance(node.value, (int, float, complex)):
            return ast.copy_location(ast.Constant(value=0), node)
        if node.value is None:
            return node
        return node


def _body_norm(body: list[ast.stmt]) -> str:
    parts: list[str] = []
    for stmt in body:
        if _is_docstring_expr(stmt):
            continue
        try:
            cloned = _NormLiterals().visit(ast.parse(ast.unparse(stmt)).body[0])
            text = ast.unparse(cloned)
        except Exception:  # noqa: BLE001
            text = _unparse(stmt)
        if text:
            parts.append(" ".join(text.split()))
    return "\n".join(parts)


def _assignment_targets(node: ast.AST) -> list[ast.expr]:
    if isinstance(node, ast.Assign):
        return list(node.targets)
    if isinstance(node, ast.AnnAssign) and node.value is not None and node.target is not None:
        return [node.target]
    return []


def _classify_assert(test: ast.expr) -> dict[str, Any]:
    info: dict[str, Any] = {
        "kind": "other",
        "text": _unparse(test),
        "left": "",
        "right": "",
        "left_is_call": False,
        "right_is_call": False,
    }
    # assert (x == 1, "msg")
    if isinstance(test, ast.Tuple) and len(test.elts) >= 1:
        info["kind"] = "tuple"
        return info

    if isinstance(test, ast.Call):
        cname = _call_name(test.func)
        if cname == "isinstance" or cname.endswith(".isinstance"):
            info["kind"] = "isinstance"
            return info
        if _is_mock_assert_name(cname):
            info["kind"] = "mock_method"
            return info

    if isinstance(test, ast.Compare) and test.ops:
        op = test.ops[0]
        left = test.left
        right = test.comparators[0] if test.comparators else None
        info["left"] = _unparse(left)
        info["right"] = _unparse(right) if right is not None else ""
        info["left_is_call"] = _is_call_expr(left)
        info["right_is_call"] = _is_call_expr(right) if right is not None else False
        if isinstance(op, (ast.Eq, ast.Is, ast.NotEq, ast.IsNot)):
            info["kind"] = "compare"
            return info
        info["kind"] = "compare"
        return info

    # bare truthy: assert x / assert not x
    if isinstance(test, (ast.Name, ast.Attribute, ast.UnaryOp, ast.BoolOp, ast.Subscript)):
        info["kind"] = "truthy"
        return info
    if isinstance(test, ast.UnaryOp) and isinstance(test.op, ast.Not):
        info["kind"] = "truthy"
        return info
    info["kind"] = "truthy" if not isinstance(test, ast.Compare) else "other"
    return info


def _collect_from_function(
    fn: ast.FunctionDef | ast.AsyncFunctionDef,
    class_name: str,
) -> dict[str, Any]:
    qual = f"{class_name}.{fn.name}" if class_name else fn.name
    end = getattr(fn, "end_lineno", None) or fn.lineno

    decorators = [_decorator_name(d) for d in fn.decorator_list]
    fixtures: list[str] = []
    for arg in fn.args.args + fn.args.kwonlyargs:
        if arg.arg in ("self", "cls"):
            continue
        fixtures.append(arg.arg)

    asserts: list[dict[str, Any]] = []
    calls: list[dict[str, Any]] = []
    raises: list[dict[str, Any]] = []
    try_except: list[dict[str, Any]] = []
    assignments: list[dict[str, Any]] = []
    for_loops: list[dict[str, Any]] = []

    nested_node_ids: set[int] = set()
    for node in ast.walk(fn):
        if node is fn:
            continue
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            for child in ast.walk(node):
                nested_node_ids.add(id(child))

    for node in ast.walk(fn):
        if id(node) in nested_node_ids:
            continue

        if isinstance(node, ast.Assert):
            item = _classify_assert(node.test)
            item["lineno"] = node.lineno
            if node.msg is not None:
                msg = _unparse(node.msg)
                if msg:
                    item["text"] = f"{item['text']}, {msg}" if item["text"] else msg
            asserts.append(item)
        elif isinstance(node, ast.Expr) and isinstance(node.value, ast.Call):
            cname = _call_name(node.value.func)
            if _is_mock_assert_name(cname):
                asserts.append(
                    {
                        "kind": "mock_method",
                        "lineno": node.lineno,
                        "text": _unparse(node.value),
                        "left": "",
                        "right": "",
                        "left_is_call": False,
                        "right_is_call": False,
                    }
                )
            elif _is_unittest_bool_name(cname) and node.value.args:
                arg0 = node.value.args[0]
                if isinstance(arg0, ast.Compare):
                    asserts.append(
                        {
                            "kind": "unittest_bool",
                            "lineno": node.lineno,
                            "text": _unparse(node.value),
                            "left": "",
                            "right": "",
                            "left_is_call": False,
                            "right_is_call": False,
                        }
                    )

        if isinstance(node, ast.Call):
            cname = _call_name(node.func)
            if cname:
                calls.append({"name": cname, "lineno": node.lineno})

        if isinstance(node, (ast.Assign, ast.AnnAssign)):
            value_node = node.value if isinstance(node, ast.Assign) else node.value
            if value_node is None:
                continue
            for tgt in _assignment_targets(node):
                tname = _unparse(tgt)
                leaf = tname.rsplit(".", 1)[-1] if tname else ""
                if leaf in ("return_value", "side_effect"):
                    assignments.append(
                        {
                            "target": tname,
                            "value": _unparse(value_node),
                            "lineno": node.lineno,
                        }
                    )

        if isinstance(node, (ast.For, ast.AsyncFor)):
            end_l = getattr(node, "end_lineno", None) or node.lineno
            for_loops.append(
                {
                    "lineno": node.lineno,
                    "end_lineno": end_l,
                    "only_asserts": _body_only_asserts(node.body),
                }
            )

        if isinstance(node, ast.With):
            for item in node.items:
                ctx = item.context_expr
                if not isinstance(ctx, ast.Call):
                    continue
                cname = _call_name(ctx.func)
                if not _is_raises_call(cname):
                    continue
                body = node.body
                raises.append(
                    {
                        "exc": _raise_exc_name(ctx),
                        "has_match": _has_match_kw(ctx),
                        "body_stmt_count": len(body),
                        "lineno": node.lineno,
                    }
                )

        if isinstance(node, ast.Try):
            for handler in node.handlers:
                bare = handler.type is None
                catches_exc = False
                if handler.type is not None:
                    tname = _unparse(handler.type)
                    catches_exc = tname in ("Exception", "BaseException") or tname.endswith(
                        ".Exception"
                    ) or tname.endswith(".BaseException")
                has_raise = any(isinstance(s, ast.Raise) for s in ast.walk(handler))
                try_except.append(
                    {
                        "bare": bare,
                        "catches_exception": catches_exc or bare,
                        "has_raise": has_raise,
                        "lineno": handler.lineno,
                    }
                )

    has_assert = len(asserts) > 0
    # unittest-style self.assertEqual etc. count as asserts
    if not has_assert:
        for c in calls:
            leaf = c["name"].rsplit(".", 1)[-1]
            if leaf.startswith("assert") and leaf != "assert_":
                has_assert = True
                break

    has_raises = len(raises) > 0
    if not has_raises:
        for c in calls:
            if _is_raises_call(c["name"]):
                has_raises = True
                break

    return {
        "name": fn.name,
        "qualname": qual,
        "class_name": class_name,
        "lineno": fn.lineno,
        "end_lineno": end,
        "decorators": decorators,
        "fixtures": fixtures,
        "asserts": asserts,
        "calls": calls,
        "raises": raises,
        "try_except": try_except,
        "assignments": assignments,
        "for_loops": for_loops,
        "body_norm": _body_norm(fn.body),
        "is_empty": _body_is_empty(fn.body),
        "has_assert": has_assert,
        "has_raises": has_raises,
    }


def _collect_class(cls: ast.ClassDef, outer: str) -> list[dict[str, Any]]:
    class_name = f"{outer}.{cls.name}" if outer else cls.name
    tests: list[dict[str, Any]] = []
    for stmt in cls.body:
        if isinstance(stmt, (ast.FunctionDef, ast.AsyncFunctionDef)):
            if _is_test_func(stmt.name):
                tests.append(_collect_from_function(stmt, class_name))
        elif isinstance(stmt, ast.ClassDef) and _is_test_class(stmt.name):
            tests.extend(_collect_class(stmt, class_name))
    return tests


def collect_tests(tree: ast.AST) -> list[dict[str, Any]]:
    tests: list[dict[str, Any]] = []
    if not isinstance(tree, ast.Module):
        return tests
    for stmt in tree.body:
        if isinstance(stmt, (ast.FunctionDef, ast.AsyncFunctionDef)):
            # test_* or legacy module-level TestFoo-style function names
            if _is_test_func(stmt.name) or _is_test_class(stmt.name):
                tests.append(_collect_from_function(stmt, ""))
        elif isinstance(stmt, ast.ClassDef) and _is_test_class(stmt.name):
            tests.extend(_collect_class(stmt, ""))
    return tests


def collect_imports(tree: ast.AST) -> list[dict[str, Any]]:
    """All Import / ImportFrom nodes in the module (including nested)."""
    imports: list[dict[str, Any]] = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            names = [alias.name for alias in node.names if alias.name]
            imports.append(
                {
                    "kind": "import",
                    "module": "",
                    "names": names,
                    "level": 0,
                    "lineno": node.lineno,
                }
            )
        elif isinstance(node, ast.ImportFrom):
            names = [alias.name for alias in node.names if alias.name and alias.name != "*"]
            imports.append(
                {
                    "kind": "from",
                    "module": node.module or "",
                    "names": names,
                    "level": int(getattr(node, "level", 0) or 0),
                    "lineno": node.lineno,
                }
            )
    return imports


def model_for_source(src: str, filename: str = "<unknown>") -> dict[str, Any]:
    tree = ast.parse(src, filename=filename)
    return {"tests": collect_tests(tree), "imports": collect_imports(tree)}


def _dump_single(path: str) -> int:
    try:
        with open(path, encoding="utf-8") as f:
            src = f.read()
        model = model_for_source(src, filename=path)
    except Exception as exc:  # noqa: BLE001
        print(f"ast_dump: {exc}", file=sys.stderr)
        return 1
    json.dump(model, sys.stdout, ensure_ascii=False)
    sys.stdout.write("\n")
    return 0


def _dump_batch() -> int:
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        path = ""
        try:
            req = json.loads(line)
            if not isinstance(req, dict):
                raise ValueError("batch request must be a JSON object")
            path = str(req.get("path", ""))
            source = str(req.get("source", ""))
            funcs = req.get("python_functions")
            classes = req.get("python_classes")
            # Always reconfigure when keys are present (incl. []) so prior globs do not stick.
            if "python_functions" in req or "python_classes" in req:
                configure_patterns(
                    [str(x) for x in funcs] if isinstance(funcs, list) else None,
                    [str(x) for x in classes] if isinstance(classes, list) else None,
                )
            model = model_for_source(source, filename=path or "<stdin>")
            out: dict[str, Any] = {"path": path, "ok": True, "model": model}
        except Exception as exc:  # noqa: BLE001
            out = {"path": path, "ok": False, "error": str(exc)}
        json.dump(out, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        sys.stdout.flush()
    return 0


def main() -> int:
    if len(sys.argv) == 1:
        return _dump_batch()
    if len(sys.argv) == 2:
        return _dump_single(sys.argv[1])
    print("usage: ast_dump.py [<file.py>]", file=sys.stderr)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
