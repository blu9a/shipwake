#!/usr/bin/env python3
"""Compare two prebuilt Go benchmark binaries on this machine, in alternating order."""
import argparse
import hashlib
import json
import os
import platform
import re
import statistics
import subprocess
from datetime import datetime, timezone
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--before', type=Path, required=True)
    parser.add_argument('--after', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--repeats', type=int, default=5)
    parser.add_argument('--gomaxprocs', type=int, default=8)
    args = parser.parse_args()
    if args.repeats < 1 or args.gomaxprocs < 1:
        parser.error('repeats and gomaxprocs must be positive')
    args.out.mkdir(parents=True, exist_ok=True)
    phases = {'before': args.before.resolve(), 'after': args.after.resolve()}
    workloads = [('history', 'BenchmarkHistoryLoad', 3), ('judge', 'BenchmarkJudgeApply', 5)]
    env = {**os.environ, 'GOMAXPROCS': str(args.gomaxprocs)}
    samples = {phase: {} for phase in phases}
    output = {phase: [] for phase in phases}
    metadata = {
        'date': datetime.now(timezone.utc).isoformat(),
        'platform': platform.platform(), 'logical_cpus': os.cpu_count(),
        'go': subprocess.check_output(['go', 'version'], text=True).strip(),
        'git': subprocess.check_output(['git', '--version'], text=True).strip(),
        'gomaxprocs': args.gomaxprocs, 'repeats': args.repeats,
        'clock': 'Go testing.B ns/op (elapsed monotonic time), fixture setup excluded by b.Loop',
        'order': 'before/after on even pairs, after/before on odd pairs',
        'cache': 'each process creates the same deterministic fixture; fixed iteration counts in both variants',
        'workloads': {name: {'benchmark': bench, 'iterations_per_sample': count} for name, bench, count in workloads},
        'binaries_sha256': {phase: {name: hashlib.sha256((directory / f'{name}.test').read_bytes()).hexdigest()
                                    for name, _, _ in workloads} for phase, directory in phases.items()},
        'load_average_start': os.getloadavg() if hasattr(os, 'getloadavg') else None,
    }
    for repetition in range(args.repeats):
        order = ['before', 'after'] if repetition % 2 == 0 else ['after', 'before']
        for name, benchmark, iterations in workloads:
            for phase in order:
                command = [str(phases[phase] / f'{name}.test'), '-test.run=^$',
                           f'-test.bench=^{benchmark}$', '-test.benchmem', f'-test.benchtime={iterations}x']
                result = subprocess.run(command, env=env, text=True, capture_output=True, check=True)
                output[phase].append(result.stdout)
                match = re.search(rf'^{benchmark}-\d+\s+(\d+)\s+([\d.]+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op',
                                  result.stdout, re.MULTILINE)
                if not match or int(match[1]) != iterations:
                    raise RuntimeError(f'Missing or incomplete benchmark: {phase} {name}\n{result.stdout}')
                sample = {'ns/op': float(match[2]), 'B/op': int(match[3]), 'allocs/op': int(match[4])}
                samples[phase].setdefault(name, []).append(sample)
                print(f'pair {repetition + 1}/{args.repeats}: {phase} {name} {sample["ns/op"] / 1e6:.3f} ms/op', flush=True)
        for phase in phases:
            (args.out / f'{phase}.txt').write_text(''.join(output[phase]))
    summary = {}
    for name, _, _ in workloads:
        summary[name] = {}
        for unit in ['ns/op', 'B/op', 'allocs/op']:
            values = {phase: [sample[unit] for sample in samples[phase][name]] for phase in phases}
            summary[name][unit] = {phase: {'median': statistics.median(v), 'min': min(v), 'max': max(v), 'samples': len(v)}
                                  for phase, v in values.items()}
            summary[name][unit]['median_change_percent'] = (statistics.median(values['after']) / statistics.median(values['before']) - 1) * 100
    metadata['load_average_end'] = os.getloadavg() if hasattr(os, 'getloadavg') else None
    (args.out / 'results.json').write_text(json.dumps({'protocol': metadata, 'samples': samples, 'summary': summary}, indent=2) + '\n')


if __name__ == '__main__':
    main()
