from __future__ import annotations

import argparse
import dataclasses
import inspect
from pathlib import Path

from anthonycmartin import bicep_testing


def generate() -> str:
    lines: list[str] = []
    for name in bicep_testing.__all__:
        value = getattr(bicep_testing, name)
        lines.append(f"CLASS {name}")
        if dataclasses.is_dataclass(value):
            lines.append(f"  {name}{inspect.signature(value)}")
        for member_name, member in inspect.getmembers(value):
            if member_name.startswith("_") or not callable(member):
                continue
            lines.append(f"  {member_name}{inspect.signature(member)}")
        lines.append("")
    return "\n".join(lines).rstrip() + "\n"


parser = argparse.ArgumentParser()
mode = parser.add_mutually_exclusive_group(required=True)
mode.add_argument("--update", action="store_true")
mode.add_argument("--check", action="store_true")
args = parser.parse_args()

baseline = Path(__file__).parents[3] / "api" / "python" / "bicep-testing.txt"
generated = generate()
if args.update:
    baseline.parent.mkdir(parents=True, exist_ok=True)
    baseline.write_text(generated, encoding="utf-8", newline="\n")
    print("Updated api/python/bicep-testing.txt")
elif not baseline.exists() or baseline.read_text(encoding="utf-8").replace("\r\n", "\n") != generated:
    raise SystemExit("Python public API has changed. Review it and run public_api.py --update.")
else:
    print("Python public API is up to date.")