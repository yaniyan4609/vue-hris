"""Compare documented API operations with the product specification.

The CSV is an implementation queue, not a list of live routes. OpenAPI only
contains active operations. Run from any working directory with Python 3.
"""

import csv
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SPEC = ROOT / "full-docs" / "Spesifikasi API digaji.id — REST + WebSocket.md"
OPENAPI = ROOT / "docs" / "openapi.yaml"
OUTPUT = ROOT / "docs" / "API_ENDPOINT_COVERAGE.csv"
METHODS = {"GET", "POST", "PUT", "PATCH", "DELETE"}
RESOURCES = ("regions", "offices", "divisions", "departments", "positions")


def active_operations():
    path = None
    operations = set()
    for line in OPENAPI.read_text(encoding="utf-8").splitlines():
        match = re.match(r"^  (/[^:]+):\s*$", line)
        if match:
            path = match.group(1)
            continue
        match = re.match(r"^    (get|post|put|patch|delete):\s*$", line)
        if path and match:
            operations.add((match.group(1).upper(), path))
    return operations


def specified_operations():
    heading = ""
    operations = set()
    for line in SPEC.read_text(encoding="utf-8").splitlines():
        if line.startswith("## ") or line.startswith("### "):
            heading = line.lstrip("# ")
        if not line.startswith("| "):
            continue
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        if len(cells) < 2:
            continue
        methods = re.findall(r"\b(?:GET|POST|PUT|PATCH|DELETE)\b", cells[0])
        if not methods or not set(methods).issubset(METHODS):
            continue
        for raw_path in re.findall(r"`(/[^`]+)`", cells[1]):
            expanded = [raw_path.replace("{resource}", name) for name in RESOURCES] if "{resource}" in raw_path else [raw_path]
            for path in expanded:
                full_path = path if path in ("/healthz", "/readyz") else "/api/v1" + path
                for method in methods:
                    operations.add((method, full_path, heading))
    return sorted(operations)


def main():
    active = active_operations()
    specified = specified_operations()
    with OUTPUT.open("w", newline="", encoding="utf-8") as file:
        writer = csv.writer(file)
        writer.writerow(("method", "path", "section", "status"))
        for method, path, heading in specified:
            writer.writerow((method, path, heading, "active" if (method, path) in active else "planned"))
    implemented = sum((method, path) in active for method, path, _ in specified)
    print(f"Specification: {len(specified)} operations; active exact matches: {implemented}; planned: {len(specified)-implemented}; OpenAPI: {len(active)} operations")
    print(f"Wrote {OUTPUT}")


if __name__ == "__main__":
    main()
