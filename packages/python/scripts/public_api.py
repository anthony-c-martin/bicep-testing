from __future__ import annotations

import argparse
import dataclasses
import inspect
from pathlib import Path

from anthonycmartin import bicep_testing
from anthonycmartin.bicep_testing import rpcclient


def generate(module: object) -> str:
    lines: list[str] = []
    for name in module.__all__:
        value = getattr(module, name)
        lines.append(f"CLASS {name}")
        if dataclasses.is_dataclass(value):
            lines.append(f"  {name}{inspect.signature(value)}")
        for member_name, member in inspect.getmembers(value):
            if member_name.startswith("_") or not callable(member):
                continue
            try:
                signature = inspect.signature(member)
            except (TypeError, ValueError):
                continue
            lines.append(f"  {member_name}{signature}")
        lines.append("")
    return "\n".join(lines).rstrip() + "\n"


parser = argparse.ArgumentParser()
mode = parser.add_mutually_exclusive_group(required=True)
mode.add_argument("--update", action="store_true")
mode.add_argument("--check", action="store_true")
args = parser.parse_args()

api_root = Path(__file__).parents[3] / "api" / "python"
targets = {
    "bicep-testing.txt": bicep_testing,
    "rpcclient.txt": rpcclient,
}
for filename, module in targets.items():
    baseline = api_root / filename
    generated = generate(module)
    if args.update:
        baseline.parent.mkdir(parents=True, exist_ok=True)
        baseline.write_text(generated, encoding="utf-8", newline="\n")
        print(f"Updated api/python/{filename}")
    elif not baseline.exists() or baseline.read_text(encoding="utf-8").replace("\r\n", "\n") != generated:
        raise SystemExit(f"Python public API has changed for {filename}. Review it and run public_api.py --update.")
    else:
        print(f"Python public API is up to date for {filename}.")