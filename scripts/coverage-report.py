#!/usr/bin/env python3
"""
Parse Go coverage output and print a per-file coverage table
similar to Vitest's text reporter.

Usage:
    go tool cover -func=coverage.out > func-coverage.txt
    python3 scripts/coverage-report.py coverage.out func-coverage.txt
"""

import sys
import re
from collections import defaultdict

def parse_func_coverage(func_file):
    """Parse `go tool cover -func` output into per-file stats."""
    file_data = defaultdict(lambda: {"total": 0, "covered": 0, "funcs": [], "uncovered_lines": []})

    with open(func_file) as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("total:"):
                continue
            # Format: github.com/consoleshop/api-gateway/internal/middleware/auth.go:45:	Auth	100.0%
            m = re.match(r'^(.+\.go):(\d+):\s+(\S+)\s+([\d.]+)%$', line)
            if not m:
                continue
            filepath, lineno, funcname, pct = m.group(1), int(m.group(2)), m.group(3), float(m.group(4))
            # Strip module path prefix for display
            short = re.sub(r'^.*?/internal/', 'internal/', filepath)
            short = re.sub(r'^.*?/cmd/', 'cmd/', short)
            file_data[short]["funcs"].append((funcname, pct))
            file_data[short]["total"] += 1
            if pct > 0:
                file_data[short]["covered"] += 1
            else:
                file_data[short]["uncovered_lines"].append(str(lineno))

    return file_data

def parse_total(func_file):
    with open(func_file) as f:
        for line in f:
            m = re.match(r'^total:\s+\(statements\)\s+([\d.]+)%', line)
            if m:
                return float(m.group(1))
    return 0.0

def coverage_color(pct):
    if pct >= 80:
        return "\033[32m"   # green
    elif pct >= 60:
        return "\033[33m"   # yellow
    else:
        return "\033[31m"   # red

RESET = "\033[0m"

def main():
    if len(sys.argv) < 3:
        print("Usage: coverage-report.py coverage.out func-coverage.txt")
        sys.exit(1)

    func_file = sys.argv[2]
    file_data = parse_func_coverage(func_file)
    total_pct = parse_total(func_file)

    col_file  = max(40, max((len(f) for f in file_data), default=40))
    col_funcs = 12
    col_lines = 12
    col_uncov = 20

    header = (
        f"{'File':<{col_file}}  "
        f"{'% Funcs':>{col_funcs}}  "
        f"{'% Lines':>{col_lines}}  "
        f"{'Uncovered Line #s':<{col_uncov}}"
    )
    sep = "-" * len(header)

    print()
    print(" Coverage Report")
    print(sep)
    print(header)
    print(sep)

    for filepath in sorted(file_data.keys()):
        data   = file_data[filepath]
        funcs  = data["funcs"]
        n_total = len(funcs)
        n_covered = sum(1 for _, pct in funcs if pct > 0)
        func_pct = (n_covered / n_total * 100) if n_total else 100.0
        # Approximate line % from func %
        line_pct = sum(pct for _, pct in funcs) / n_total if n_total else 100.0
        uncov = ", ".join(data["uncovered_lines"][:5])
        if len(data["uncovered_lines"]) > 5:
            uncov += "…"

        color = coverage_color(line_pct)
        print(
            f"{color}{filepath:<{col_file}}  "
            f"{func_pct:>{col_funcs}.1f}  "
            f"{line_pct:>{col_lines}.1f}  "
            f"{uncov:<{col_uncov}}{RESET}"
        )

    print(sep)
    total_color = coverage_color(total_pct)
    print(f"{total_color}{'All files':<{col_file}}  {'':>{col_funcs}}  {total_pct:>{col_lines}.1f}{RESET}")
    print()

    if total_pct < 80:
        print(f"\033[31m✗ Coverage {total_pct:.1f}% is below the 80% threshold\033[0m")
        sys.exit(1)
    else:
        print(f"\033[32m✓ Coverage {total_pct:.1f}% meets the 80% threshold\033[0m")

if __name__ == "__main__":
    main()
