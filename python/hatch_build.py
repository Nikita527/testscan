"""Hatch build hook: platform wheels embed a native Go binary."""

from hatchling.builders.hooks.plugin.interface import BuildHookInterface


class CustomBuildHook(BuildHookInterface):
    def initialize(self, version, build_data):
        # Wheel ships testscan_py/bin/testscan(.exe) — not a purelib.
        # Keep default py3-none-any tag; release script retags to the CI platform.
        build_data["pure_python"] = False
        build_data["infer_tag"] = False
        build_data["tag"] = "py3-none-any"