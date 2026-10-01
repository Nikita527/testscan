#!/usr/bin/env python3
"""Dump a per-test AST model as JSON (single file) or JSONL (batch stdin)."""
from __future__ import annotations

import ast
import base64
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


_FIELD_CACHE: dict[type, tuple[str, ...]] = {}


def _kids(node: ast.AST) -> list[ast.AST]:
    """Child nodes (faster than ast.iter_child_nodes; expression contexts are skipped)."""
    cls = node.__class__
    fields = _FIELD_CACHE.get(cls)
    if fields is None:
        fields = _FIELD_CACHE[cls] = tuple(f for f in cls._fields if f != "ctx")
    out: list[ast.AST] = []
    ast_type = ast.AST
    for f in fields:
        v = getattr(node, f, None)
        if v.__class__ is list:
            for x in v:
                if isinstance(x, ast_type):
                    out.append(x)
        elif isinstance(v, ast_type):
            out.append(v)
    return out


def _walk(node: ast.AST):
    """ast.walk with the same breadth-first order, without the deque / generator overhead."""
    todo = [node]
    for n in todo:
        yield n
        todo.extend(_kids(n))


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


def _decorator_is_fixture(dec: ast.expr) -> bool:
    if isinstance(dec, ast.Call):
        return _call_name(dec.func).rsplit(".", 1)[-1] == "fixture"
    return _call_name(dec).rsplit(".", 1)[-1] == "fixture"


def _is_fixture_function(fn: ast.FunctionDef | ast.AsyncFunctionDef) -> bool:
    return any(_decorator_is_fixture(d) for d in fn.decorator_list)


def _class_has_init(cls: ast.ClassDef) -> bool:
    for stmt in cls.body:
        if isinstance(stmt, (ast.FunctionDef, ast.AsyncFunctionDef)) and stmt.name == "__init__":
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


def _is_const_name(name: str) -> bool:
    """True for UPPER_CASE / UPPERCASE modular constants (not single lowercase)."""
    if not name or not name.replace("_", "").isalnum():
        return False
    letters = [c for c in name if c.isalpha()]
    return bool(letters) and all(c.isupper() for c in letters)


def _range_const_len(call: ast.Call) -> int | None:
    """Return iteration length for range(...) with int-literal bounds, else None."""
    if _call_name(call.func) != "range" or call.keywords:
        return None
    args = call.args
    vals: list[int] = []
    for a in args:
        if isinstance(a, ast.UnaryOp) and isinstance(a.op, ast.USub) and isinstance(
            a.operand, ast.Constant
        ) and isinstance(a.operand.value, int):
            vals.append(-a.operand.value)
        elif isinstance(a, ast.Constant) and isinstance(a.value, int):
            vals.append(a.value)
        else:
            return None
    if len(vals) == 1:
        start, stop, step = 0, vals[0], 1
    elif len(vals) == 2:
        start, stop, step = vals[0], vals[1], 1
    elif len(vals) == 3:
        start, stop, step = vals[0], vals[1], vals[2]
    else:
        return None
    if step == 0:
        return None
    if step > 0:
        n = max(0, (stop - start + step - 1) // step)
    else:
        n = max(0, (start - stop - step - 1) // (-step))
    return n


def _classify_for_iter(node: ast.AST) -> tuple[str, str]:
    """Classify for-loop iterable: literal_nonempty|range_const|upper_name|attr_const|other."""
    text = _unparse(node)
    if isinstance(node, (ast.List, ast.Tuple, ast.Set)):
        if len(node.elts) >= 1:
            return "literal_nonempty", text
        return "other", text
    if isinstance(node, ast.Dict):
        if len(node.keys) >= 1:
            return "literal_nonempty", text
        return "other", text
    if isinstance(node, ast.Call):
        n = _range_const_len(node)
        if n is not None and n >= 1:
            return "range_const", text
        return "other", text
    if isinstance(node, ast.Name) and _is_const_name(node.id):
        return "upper_name", text
    if isinstance(node, ast.Attribute) and _is_const_name(node.attr):
        return "attr_const", text
    return "other", text


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


# --- SUT / project-awareness facts (consumed by rules/sut.go, rules/sleep.go) ---

_WALL_LEAVES = ("now", "today", "utcnow")
_MODEL_CALL_LEAVES = (
    "create",
    "bulk_create",
    "get_or_create",
    "update_or_create",
    "save",
    "update",
    "filter",
    "get",
    "exclude",
)
_LITERAL_NODES = (
    ast.Constant,
    ast.JoinedStr,
    ast.List,
    ast.Tuple,
    ast.Dict,
    ast.Set,
    ast.ListComp,
    ast.SetComp,
    ast.DictComp,
    ast.GeneratorExp,
)
_STDLIB_FALLBACK = frozenset(
    "abc argparse array ast asyncio base64 binascii bisect builtins calendar collections "
    "contextlib copy csv dataclasses datetime decimal enum functools gc glob hashlib heapq "
    "hmac html http importlib inspect io itertools json logging math multiprocessing "
    "operator os pathlib pickle platform pprint queue random re secrets shlex shutil signal "
    "socket sqlite3 ssl statistics string struct subprocess sys tempfile textwrap threading "
    "time traceback types typing unittest urllib uuid warnings weakref xml zipfile zlib "
    "__future__".split()
)


def _stdlib_names() -> frozenset[str]:
    names = getattr(sys, "stdlib_module_names", None)
    return frozenset(names) if names else _STDLIB_FALLBACK


def _build_parents(root: ast.AST) -> dict[int, ast.AST]:
    parents: dict[int, ast.AST] = {}
    for node in _walk(root):
        for child in _kids(node):
            parents[id(child)] = node
    return parents


def _recv_is_literal(call: ast.Call) -> bool:
    """`"x".join(...)`, `[].append(...)`: the receiver is a literal of a builtin type."""
    func = call.func
    return isinstance(func, ast.Attribute) and isinstance(func.value, _LITERAL_NODES)


def _is_arg_of_call(node: ast.AST, parents: dict[int, ast.AST]) -> bool:
    """True when node's value is consumed as an argument of an enclosing call.

    Walking up through expression parents (dict/tuple/list/keyword...): reaching a Call
    as one of its args/keywords -> True; reaching it via `.func` (node is the receiver
    or callee chain) or a statement -> False.
    """
    cur = node
    while True:
        parent = parents.get(id(cur))
        if parent is None or isinstance(parent, ast.stmt):
            return False
        if isinstance(parent, ast.Call):
            return cur is not parent.func
        cur = parent


def _in_raises_block(node: ast.AST, parents: dict[int, ast.AST]) -> bool:
    cur = node
    while True:
        parent = parents.get(id(cur))
        if parent is None:
            return False
        if isinstance(parent, (ast.With, ast.AsyncWith)) and cur in parent.body:
            for item in parent.items:
                ctx = item.context_expr
                if isinstance(ctx, ast.Call) and _is_raises_call(_call_name(ctx.func)):
                    return True
        cur = parent


def _value_origin(value: ast.AST | None) -> str:
    """Coarse origin of a local variable: callee name, `<literal>` or `<other>`."""
    if value is None:
        return "<other>"
    if isinstance(value, ast.Await):
        value = value.value
    if isinstance(value, _LITERAL_NODES):
        return "<literal>"
    if isinstance(value, ast.Call):
        return _call_name(value.func) or "<other>"
    if isinstance(value, (ast.Name, ast.Attribute)):
        return _call_name(value) or "<other>"
    return "<other>"


def _var_origins(fn: ast.AST, nested_ids: set[int]) -> dict[str, str]:
    """name -> origin for simple `name = expr` / `with expr as name` (first binding wins)."""
    origins: dict[str, str] = {}
    for node in _walk(fn):
        if id(node) in nested_ids:
            continue
        if isinstance(node, ast.Assign):
            for tgt in node.targets:
                if isinstance(tgt, ast.Name):
                    origins.setdefault(tgt.id, _value_origin(node.value))
                elif isinstance(tgt, (ast.Tuple, ast.List)):
                    for elt in tgt.elts:
                        if isinstance(elt, ast.Name):
                            origins.setdefault(elt.id, "<other>")
        elif isinstance(node, ast.AnnAssign) and isinstance(node.target, ast.Name):
            origins.setdefault(node.target.id, _value_origin(node.value))
        elif isinstance(node, (ast.With, ast.AsyncWith)):
            for item in node.items:
                if isinstance(item.optional_vars, ast.Name):
                    origins.setdefault(item.optional_vars.id, _value_origin(item.context_expr))
    return origins


def _wall_call_facts(
    call: ast.Call,
    fn: ast.AST,
    parents: dict[int, ast.AST],
    nested_ids: set[int],
) -> dict[str, Any]:
    """Flow facts for datetime.now()/date.today()/utcnow(): where the value goes.

    flow: subset of {"assert", "model_arg", "call_arg"} (see _walk_flow).
    aware_chain: `.astimezone(...)` / `.replace(tzinfo=...)` applied directly.
    in_raises: the call sits inside a `with pytest.raises(...)` body.
    """
    facts: dict[str, Any] = {
        "argc": len(call.args) + len(call.keywords),
        "flow": [],
        "aware_chain": False,
        "in_raises": _in_raises_block(call, parents),
    }
    parent = parents.get(id(call))
    if isinstance(parent, ast.Attribute):
        grand = parents.get(id(parent))
        if isinstance(grand, ast.Call) and grand.func is parent:
            if parent.attr == "astimezone":
                facts["aware_chain"] = True
            elif parent.attr == "replace" and any(k.arg == "tzinfo" for k in grand.keywords):
                facts["aware_chain"] = True
    out: set[str] = set()
    if not facts["in_raises"]:
        loads: dict[str, list[ast.Name]] = {}
        for n in _walk(fn):
            if id(n) in nested_ids:
                continue
            if isinstance(n, ast.Name) and isinstance(n.ctx, ast.Load):
                loads.setdefault(n.id, []).append(n)
        _walk_flow(call, parents, loads, out, set(), 0)
    facts["flow"] = sorted(out)
    return facts


def _walk_flow(
    node: ast.AST,
    parents: dict[int, ast.AST],
    loads: dict[str, list[ast.Name]],
    out: set[str],
    seen: set[str],
    depth: int,
) -> None:
    """Follow a value up through expressions; follow `name = value` to the name's uses."""
    cur = node
    while True:
        parent = parents.get(id(cur))
        if parent is None:
            return
        if isinstance(parent, ast.Call) and cur is not parent.func:
            cname = _call_name(parent.func)
            leaf = cname.rsplit(".", 1)[-1]
            if leaf.startswith("assert") or _is_mock_assert_name(cname):
                out.add("assert")
            elif leaf in _MODEL_CALL_LEAVES or leaf[:1].isupper():
                out.add("model_arg")
            else:
                out.add("call_arg")
            # The value is consumed by the call; what the call returns is not
            # the wall-clock value any more, so stop following it.
            return
        elif isinstance(parent, ast.Assert):
            out.add("assert")
            return
        elif isinstance(parent, (ast.Assign, ast.AnnAssign, ast.AugAssign, ast.NamedExpr)):
            if getattr(parent, "value", None) is not cur:
                return
            if isinstance(parent, ast.Assign):
                targets = list(parent.targets)
            else:
                targets = [parent.target]
            for tgt in targets:
                if isinstance(tgt, ast.Name):
                    if depth < 3 and tgt.id not in seen:
                        seen.add(tgt.id)
                        for use in loads.get(tgt.id, []):
                            if not _in_raises_block(use, parents):
                                _walk_flow(use, parents, loads, out, seen, depth + 1)
                elif isinstance(tgt, (ast.Attribute, ast.Subscript)):
                    out.add("model_arg")
            return
        elif isinstance(parent, ast.stmt):
            return
        cur = parent


# --- def-use facts (consumed by rules/dataflow.go; SUT is decided in Go only) ---

_SELF_NAMES = ("self", "cls")


_MOCK_STATE_ATTRS = frozenset(
    ("call_args", "call_args_list", "call_count", "called", "mock_calls", "method_calls")
)


def _self_path(node: ast.AST) -> str:
    """Tracked root of an attribute/subscript chain: `a.b[0].c` -> "a", `self.m.x` -> "self.m"."""
    cur: ast.AST = node
    path = ""
    while isinstance(cur, (ast.Attribute, ast.Subscript)):
        if isinstance(cur, ast.Attribute) and isinstance(cur.value, ast.Name) and cur.value.id in _SELF_NAMES:
            return f"{cur.value.id}.{cur.attr}"
        cur = cur.value
    if isinstance(cur, ast.Name) and cur.id not in _SELF_NAMES:
        return cur.id
    return path


class _Facts:
    """Names read, call indices and mock-state reads below an anchor node."""

    __slots__ = ("reads", "calls", "mstate")

    def __init__(self) -> None:
        self.reads: set[str] = set()
        self.calls: list[int] = []
        self.mstate: set[str] = set()


_EXIT = object()
_SCOPE_NODES = (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)


def _fact_walk(
    fn: ast.AST,
    call_idx: dict[int, int],
    anchors: dict[int, _Facts],
) -> dict[int, set[str]]:
    """One traversal of the function body (nested defs/classes skipped).

    Fills every anchor's reads / calls / mstate and returns, per call index, the
    names loaded anywhere in that call's arguments (nested calls included).
    `self.x` / `cls.x` is tracked as the path "self.x" (never bare "self"), so a
    store to one attribute does not taint every self.assert*.
    """
    call_reads: dict[int, set[str]] = {}
    open_anchors: list[_Facts] = []
    stack: list[Any] = [(stmt, ()) for stmt in reversed(fn.body)]
    while stack:
        item = stack.pop()
        if item is _EXIT:
            open_anchors.pop()
            continue
        n, argsets = item
        if isinstance(n, _SCOPE_NODES):
            continue
        col = anchors.get(id(n))
        if col is not None:
            open_anchors.append(col)
            stack.append(_EXIT)
        if isinstance(n, ast.Name):
            if isinstance(n.ctx, ast.Load) and n.id not in _SELF_NAMES:
                name = n.id
                for c in open_anchors:
                    c.reads.add(name)
                for rs in argsets:
                    rs.add(name)
            continue
        if isinstance(n, ast.Attribute):
            if n.attr in _MOCK_STATE_ATTRS and isinstance(n.ctx, ast.Load) and open_anchors:
                root = _self_path(n.value)
                if root:
                    for c in open_anchors:
                        c.mstate.add(root)
            v = n.value
            if isinstance(v, ast.Name) and v.id in _SELF_NAMES:
                if isinstance(n.ctx, ast.Load):
                    path = f"{v.id}.{n.attr}"
                    for c in open_anchors:
                        c.reads.add(path)
                    for rs in argsets:
                        rs.add(path)
                continue
        elif isinstance(n, ast.Call):
            i = call_idx.get(id(n))
            inner = argsets
            if i is not None:
                for c in open_anchors:
                    c.calls.append(i)
                rs = call_reads.get(i)
                if rs is None:
                    rs = call_reads[i] = set()
                inner = argsets + (rs,)
            for kw in reversed(n.keywords):
                stack.append((kw.value, inner))
            for a in reversed(n.args):
                stack.append((a, inner))
            stack.append((n.func, argsets))
            continue
        for ch in _kids(n):
            stack.append((ch, argsets))
    return call_reads


def _nested_facts(fn: ast.AST, scopes: list[ast.AST]) -> tuple[list[str], list[str], bool]:
    """(names written by nested functions/lambdas/callbacks, nested function names, nested assert).

    A callback (nested def, lambda) that appends to / stores into / rebinds an outer
    name (`built.append(1)`, `calls["n"] += 1`, `nonlocal n`) mutates state the
    assertions may read after the code under test invoked it. `scopes` are the
    nested def/lambda nodes in walk order; inner scopes are merged into the
    outermost one that contains them.
    """
    defs: set[str] = set()
    has_assert = False
    writes: set[str] = set()
    seen: set[int] = set()
    for scope in scopes:
        if id(scope) in seen:
            continue
        local: set[str] = set()
        declared: set[str] = set()
        cand: list[str] = []
        for sub in _walk(scope):
            seen.add(id(sub))
            if isinstance(sub, (ast.FunctionDef, ast.AsyncFunctionDef, ast.Lambda)):
                if not isinstance(sub, ast.Lambda):
                    defs.add(sub.name)
                a = sub.args
                for arg in list(a.args) + list(a.kwonlyargs) + list(getattr(a, "posonlyargs", [])):
                    local.add(arg.arg)
                if a.vararg:
                    local.add(a.vararg.arg)
                if a.kwarg:
                    local.add(a.kwarg.arg)
            elif isinstance(sub, ast.Name):
                if isinstance(sub.ctx, ast.Store):
                    local.add(sub.id)
            elif isinstance(sub, (ast.Nonlocal, ast.Global)):
                declared.update(sub.names)
            elif isinstance(sub, ast.Assert):
                has_assert = True
            elif isinstance(sub, (ast.Assign, ast.AugAssign, ast.AnnAssign)):
                targets = sub.targets if isinstance(sub, ast.Assign) else [sub.target]
                for tgt in targets:
                    if isinstance(tgt, (ast.Attribute, ast.Subscript)):
                        cand.extend(_target_roots(tgt))
            elif isinstance(sub, ast.Call) and isinstance(sub.func, ast.Attribute):
                cand.extend(_target_roots(sub.func.value))
        writes.update(declared)
        local -= declared
        for r in cand:
            if r.split(".", 1)[0] not in local:
                writes.add(r)
    for node in _walk(fn):
        if isinstance(node, ast.Assign) and isinstance(node.value, ast.Lambda):
            for tgt in node.targets:
                if isinstance(tgt, ast.Name):
                    defs.add(tgt.id)
    return sorted(writes), sorted(defs), has_assert


def _target_roots(tgt: ast.AST) -> list[str]:
    """Names written by an assignment target; attribute/subscript stores write their root."""
    if isinstance(tgt, ast.Name):
        return [tgt.id]
    if isinstance(tgt, (ast.Tuple, ast.List)):
        out: list[str] = []
        for elt in tgt.elts:
            out.extend(_target_roots(elt))
        return out
    if isinstance(tgt, ast.Starred):
        return _target_roots(tgt.value)
    cur: ast.AST | None = tgt
    while isinstance(cur, (ast.Attribute, ast.Subscript)):
        if isinstance(cur, ast.Attribute):
            if isinstance(cur.value, ast.Name) and cur.value.id in _SELF_NAMES:
                return [f"{cur.value.id}.{cur.attr}"]
            cur = cur.value
        else:
            cur = cur.value
    if isinstance(cur, ast.Name) and cur.id not in _SELF_NAMES:
        return [cur.id]
    return []


def _collect_from_function(
    fn: ast.FunctionDef | ast.AsyncFunctionDef,
    class_name: str,
    light: bool = False,
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
    call_idx: dict[int, int] = {}
    assert_nodes: list[ast.AST] = []
    pending_flows: list[tuple[str, int, list[str], ast.AST | None, int, list[str], int]] = []

    nested_node_ids: set[int] = set()
    scope_nodes: list[ast.AST] = []
    for node in _walk(fn):
        if node is fn:
            continue
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.Lambda)):
            scope_nodes.append(node)
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            for child in _walk(node):
                nested_node_ids.add(id(child))

    parents = _build_parents(fn)
    bare_call_ids: set[int] = set()
    for node in _walk(fn):
        if id(node) in nested_node_ids:
            continue
        if isinstance(node, ast.Expr) and isinstance(node.value, ast.Call):
            bare_call_ids.add(id(node.value))

    for node in _walk(fn):
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
            assert_nodes.append(node.test)
        elif isinstance(node, ast.Expr) and isinstance(node.value, ast.Call):
            cname = _call_name(node.value.func)
            if _is_mock_assert_name(cname):
                assert_nodes.append(node.value)
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
                    assert_nodes.append(node.value)
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
                entry: dict[str, Any] = {
                    "name": cname,
                    "lineno": node.lineno,
                    "bare": id(node) in bare_call_ids,
                }
                if _recv_is_literal(node):
                    entry["recv_literal"] = True
                if _is_arg_of_call(node, parents):
                    entry["arg_of_call"] = True
                if cname.rsplit(".", 1)[-1] in _WALL_LEAVES:
                    entry.update(_wall_call_facts(node, fn, parents, nested_node_ids))
                call_idx[id(node)] = len(calls)
                calls.append(entry)

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

        # def-use flow nodes (targets <- reads/calls of the value expression)
        if light:
            pass
        elif isinstance(node, ast.Assign):
            roots: list[str] = []
            for tgt in node.targets:
                roots.extend(_target_roots(tgt))
            pending_flows.append(("assign", node.lineno, roots, node.value, -1, [], 0))
        elif isinstance(node, ast.AnnAssign) and node.value is not None:
            pending_flows.append(("assign", node.lineno, _target_roots(node.target), node.value, -1, [], 0))
        elif isinstance(node, ast.AugAssign):
            pending_flows.append(
                ("assign", node.lineno, _target_roots(node.target), node.value, -1, _target_roots(node.target), 0)
            )
        elif isinstance(node, ast.NamedExpr):
            pending_flows.append(("assign", node.lineno, _target_roots(node.target), node.value, -1, [], 0))
        elif isinstance(node, (ast.For, ast.AsyncFor)):
            pending_flows.append(("for", node.lineno, _target_roots(node.target), node.iter, -1, [], 0))
        elif isinstance(node, (ast.With, ast.AsyncWith)):
            w_end = getattr(node, "end_lineno", None) or node.lineno
            for witem in node.items:
                w_targets = _target_roots(witem.optional_vars) if witem.optional_vars is not None else []
                pending_flows.append(
                    ("with", node.lineno, w_targets, witem.context_expr, -1, [], w_end)
                )
        elif isinstance(node, ast.Expr):
            val = node.value
            if isinstance(val, ast.Await):
                val = val.value
            if isinstance(val, ast.Call):
                recv: list[str] = []
                if isinstance(val.func, ast.Attribute):
                    recv = _target_roots(val.func.value)
                pending_flows.append(("expr", node.lineno, recv, val, id(val), recv, 0))

        if isinstance(node, (ast.For, ast.AsyncFor)):
            end_l = getattr(node, "end_lineno", None) or node.lineno
            iter_kind, iter_text = _classify_for_iter(node.iter)
            for_loops.append(
                {
                    "lineno": node.lineno,
                    "end_lineno": end_l,
                    "only_asserts": _body_only_asserts(node.body),
                    "iter_kind": iter_kind,
                    "iter_text": iter_text,
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
                asname = ""
                if item.optional_vars is not None:
                    asname = _unparse(item.optional_vars)
                end_l = getattr(node, "end_lineno", None) or node.lineno
                raises.append(
                    {
                        "exc": _raise_exc_name(ctx),
                        "has_match": _has_match_kw(ctx),
                        "body_stmt_count": len(body),
                        "lineno": node.lineno,
                        "end_lineno": end_l,
                        "asname": asname,
                    }
                )

        if isinstance(node, ast.Try):
            t_end = max((getattr(b, "end_lineno", None) or b.lineno) for b in node.body) if node.body else node.lineno
            for handler in node.handlers:
                if handler.name:
                    pending_flows.append(("except", node.lineno, [handler.name], None, -1, [], t_end))
                bare = handler.type is None
                catches_exc = False
                if handler.type is not None:
                    tname = _unparse(handler.type)
                    catches_exc = tname in ("Exception", "BaseException") or tname.endswith(
                        ".Exception"
                    ) or tname.endswith(".BaseException")
                has_raise = any(isinstance(s, ast.Raise) for s in _walk(handler))
                try_except.append(
                    {
                        "bare": bare,
                        "catches_exception": catches_exc or bare,
                        "has_raise": has_raise,
                        "lineno": handler.lineno,
                    }
                )

    flows: list[dict[str, Any]] = []
    if not light:
        anchors: dict[int, _Facts] = {}
        for anode in assert_nodes:
            anchors.setdefault(id(anode), _Facts())
        for fl in pending_flows:
            if fl[3] is not None:
                anchors.setdefault(id(fl[3]), _Facts())
        call_reads = _fact_walk(fn, call_idx, anchors)
        for ci, rs in call_reads.items():
            if rs:
                calls[ci]["reads"] = sorted(rs)
        for item, anode in zip(asserts, assert_nodes):
            fa = anchors[id(anode)]
            if fa.reads:
                item["reads"] = sorted(fa.reads)
            if fa.calls:
                item["calls"] = sorted(fa.calls)
            if fa.mstate:
                item["mstate"] = sorted(fa.mstate)
        for kind, line, targets, vnode, top_id, extra, f_end in pending_flows:
            fa = anchors[id(vnode)] if vnode is not None else _Facts()
            f_reads = sorted(fa.reads | set(extra)) if extra else sorted(fa.reads)
            f_calls = sorted(fa.calls)
            if not targets and kind not in ("expr", "with"):
                continue
            if not f_reads and not f_calls and kind != "except":
                continue
            flow: dict[str, Any] = {"kind": kind, "lineno": line}
            if f_end:
                flow["end_lineno"] = f_end
            if fa.mstate:
                flow["mstate"] = sorted(fa.mstate)
            if targets:
                flow["targets"] = targets
            if f_reads:
                flow["reads"] = f_reads
            if f_calls:
                flow["calls"] = f_calls
            if kind == "expr":
                top = call_idx.get(top_id)
                if top is not None:
                    flow["top"] = top
            flows.append(flow)
    flows.sort(key=lambda d: d["lineno"])
    if light or not scope_nodes:
        nested_writes, nested_defs, nested_assert = [], [], False
    else:
        nested_writes, nested_defs, nested_assert = _nested_facts(fn, scope_nodes)

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
        "flows": flows,
        "nested_writes": nested_writes,
        "nested_defs": nested_defs,
        "nested_assert": nested_assert,
        "var_origins": _var_origins(fn, nested_node_ids),
        "body_norm": _body_norm(fn.body),
        "is_empty": _body_is_empty(fn.body),
        "has_assert": has_assert,
        "has_raises": has_raises,
    }


def _helper_summary(
    fn: ast.FunctionDef | ast.AsyncFunctionDef,
    class_name: str,
) -> dict[str, Any]:
    """Lightweight helper record for one-level assert follow from tests."""
    full = _collect_from_function(fn, class_name, light=True)
    return {
        "name": full["name"],
        "qualname": full["qualname"],
        "has_assert": full["has_assert"] or full["has_raises"],
    }


def _collect_class(cls: ast.ClassDef, outer: str) -> list[dict[str, Any]]:
    class_name = f"{outer}.{cls.name}" if outer else cls.name
    tests: list[dict[str, Any]] = []
    for stmt in cls.body:
        if isinstance(stmt, (ast.FunctionDef, ast.AsyncFunctionDef)):
            if _is_test_func(stmt.name) and not _is_fixture_function(stmt):
                tests.append(_collect_from_function(stmt, class_name))
        elif isinstance(stmt, ast.ClassDef) and _is_test_class(stmt.name):
            if not _class_has_init(stmt):
                tests.extend(_collect_class(stmt, class_name))
    return tests


def _collect_class_helpers(cls: ast.ClassDef, outer: str) -> list[dict[str, Any]]:
    class_name = f"{outer}.{cls.name}" if outer else cls.name
    helpers: list[dict[str, Any]] = []
    for stmt in cls.body:
        if isinstance(stmt, (ast.FunctionDef, ast.AsyncFunctionDef)):
            if _is_fixture_function(stmt):
                continue
            if _is_test_func(stmt.name):
                continue
            helpers.append(_helper_summary(stmt, class_name))
        elif isinstance(stmt, ast.ClassDef):
            helpers.extend(_collect_class_helpers(stmt, class_name))
    return helpers


def collect_tests(tree: ast.AST) -> list[dict[str, Any]]:
    tests: list[dict[str, Any]] = []
    if not isinstance(tree, ast.Module):
        return tests
    for stmt in tree.body:
        if isinstance(stmt, (ast.FunctionDef, ast.AsyncFunctionDef)):
            # test_* or legacy module-level TestFoo-style function names
            if (_is_test_func(stmt.name) or _is_test_class(stmt.name)) and not _is_fixture_function(stmt):
                tests.append(_collect_from_function(stmt, ""))
        elif isinstance(stmt, ast.ClassDef) and _is_test_class(stmt.name):
            if not _class_has_init(stmt):
                tests.extend(_collect_class(stmt, ""))
    return tests


def collect_helpers(tree: ast.AST) -> list[dict[str, Any]]:
    """Non-test functions (module + nested in classes) for assert-helper follow."""
    helpers: list[dict[str, Any]] = []
    if not isinstance(tree, ast.Module):
        return helpers
    for stmt in tree.body:
        if isinstance(stmt, (ast.FunctionDef, ast.AsyncFunctionDef)):
            if _is_fixture_function(stmt):
                continue
            if _is_test_func(stmt.name) or _is_test_class(stmt.name):
                continue
            helpers.append(_helper_summary(stmt, ""))
        elif isinstance(stmt, ast.ClassDef):
            helpers.extend(_collect_class_helpers(stmt, ""))
    return helpers


def collect_imports(tree: ast.AST) -> list[dict[str, Any]]:
    """All Import / ImportFrom nodes in the module (including nested).

    `bindings` lists the local names each statement creates (alias-aware) with the
    absolute module they come from and whether that module is stdlib.
    """
    stdlib = _stdlib_names()
    imports: list[dict[str, Any]] = []
    for node in _walk(tree):
        if isinstance(node, ast.Import):
            names = [alias.name for alias in node.names if alias.name]
            bindings = []
            for alias in node.names:
                if not alias.name:
                    continue
                bindings.append(
                    {
                        "local": alias.asname or alias.name.split(".", 1)[0],
                        "module": alias.name,
                        "name": "",
                        "stdlib": alias.name.split(".", 1)[0] in stdlib,
                    }
                )
            imports.append(
                {
                    "kind": "import",
                    "module": "",
                    "names": names,
                    "level": 0,
                    "lineno": node.lineno,
                    "bindings": bindings,
                }
            )
        elif isinstance(node, ast.ImportFrom):
            names = [alias.name for alias in node.names if alias.name and alias.name != "*"]
            level = int(getattr(node, "level", 0) or 0)
            mod = node.module or ""
            bindings = []
            for alias in node.names:
                if not alias.name or alias.name == "*":
                    continue
                bindings.append(
                    {
                        "local": alias.asname or alias.name,
                        "module": mod,
                        "name": alias.name,
                        "stdlib": level == 0 and mod.split(".", 1)[0] in stdlib,
                    }
                )
            imports.append(
                {
                    "kind": "from",
                    "module": mod,
                    "names": names,
                    "level": level,
                    "lineno": node.lineno,
                    "bindings": bindings,
                }
            )
    return imports


def collect_module_defs(tree: ast.AST) -> list[str]:
    """Names defined at module level in this file (defs, classes, assignments).

    A call rooted in one of these is test-local, never project code under test.
    """
    names: list[str] = []

    def add_target(tgt: ast.AST) -> None:
        if isinstance(tgt, ast.Name):
            names.append(tgt.id)
        elif isinstance(tgt, (ast.Tuple, ast.List)):
            for elt in tgt.elts:
                add_target(elt)

    def visit(body: list[ast.stmt]) -> None:
        for stmt in body:
            if isinstance(stmt, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
                names.append(stmt.name)
            elif isinstance(stmt, ast.Assign):
                for tgt in stmt.targets:
                    add_target(tgt)
            elif isinstance(stmt, ast.AnnAssign):
                add_target(stmt.target)
            elif isinstance(stmt, ast.If):
                visit(stmt.body)
                visit(stmt.orelse)
            elif isinstance(stmt, ast.Try):
                visit(stmt.body)
                for h in stmt.handlers:
                    visit(h.body)
                visit(stmt.orelse)

    if isinstance(tree, ast.Module):
        visit(tree.body)
    return sorted(set(names))


def model_for_source(src: str | bytes, filename: str = "<unknown>") -> dict[str, Any]:
    if isinstance(src, str):
        data = src.encode("utf-8", errors="surrogatepass")
    else:
        data = src
    tree = ast.parse(data, filename=filename)
    return {
        "tests": collect_tests(tree),
        "imports": collect_imports(tree),
        "helpers": collect_helpers(tree),
        "module_defs": collect_module_defs(tree),
    }


def _dump_single(path: str) -> int:
    try:
        with open(path, "rb") as f:
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
            source_b64 = req.get("source_b64")
            if source_b64 is not None and source_b64 != "":
                source = base64.standard_b64decode(str(source_b64))
            else:
                source = str(req.get("source", "")).encode("utf-8", errors="surrogatepass")
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
