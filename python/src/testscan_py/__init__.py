"""Launcher package for the Go testscan CLI."""

try:
    from testscan_py._version import __version__
except ImportError:  # pragma: no cover - missing only in bare checkouts
    __version__ = "0.0.0"
