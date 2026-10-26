#!/usr/bin/env python3
"""
fix-bare-fences.py — add language tags to bare opening code fences in markdown files.

Scans the given files (or all .md files under the given directories) and replaces
any opening ``` fence that has no language tag with ```text.

Closing fences are always bare ``` and are left unchanged.

Usage:
    python3 scripts/fix-bare-fences.py [paths...]

Examples:
    # Fix all markdown in documentation/ and examples/
    python3 scripts/fix-bare-fences.py documentation/ examples/

    # Fix specific files
    python3 scripts/fix-bare-fences.py docs/guide.md pkg/profiles/docs/

    # Fix all markdown in the repo
    python3 scripts/fix-bare-fences.py .
"""

import os
import re
import sys


def fix_file(path: str, dry_run: bool = False) -> bool:
    with open(path) as f:
        lines = f.readlines()

    in_block = False
    new_lines = []
    changed = False

    for line in lines:
        stripped = line.rstrip()
        if stripped == "```":
            if not in_block:
                new_lines.append("```text\n")
                in_block = True
                changed = True
            else:
                new_lines.append(line)
                in_block = False
        elif re.match(r"^```\w", stripped):
            new_lines.append(line)
            in_block = True
        else:
            new_lines.append(line)

    if changed and not dry_run:
        with open(path, "w") as f:
            f.writelines(new_lines)

    return changed


def collect_markdown(paths: list[str]) -> list[str]:
    files = []
    for p in paths:
        if os.path.isfile(p) and p.endswith(".md"):
            files.append(p)
        elif os.path.isdir(p):
            for root, _, fnames in os.walk(p):
                for name in fnames:
                    if name.endswith(".md"):
                        files.append(os.path.join(root, name))
    return sorted(files)


def main() -> None:
    args = sys.argv[1:]
    dry_run = "--dry-run" in args
    targets = [a for a in args if a != "--dry-run"] or ["."]

    files = collect_markdown(targets)

    if not files:
        print("No markdown files found.")
        return

    if dry_run:
        print("Dry run — no files will be modified.\n")

    fixed = 0
    for path in files:
        if fix_file(path, dry_run=dry_run):
            print(f"{'would fix' if dry_run else 'fixed'}: {path}")
            fixed += 1

    total = len(files)
    print(f"\n{fixed} file(s) {'would be changed' if dry_run else 'changed'}, {total - fixed} already clean ({total} scanned).")


if __name__ == "__main__":
    main()
