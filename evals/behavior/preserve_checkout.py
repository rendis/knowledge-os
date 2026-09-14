"""Fingerprint a consumer without copying its content or Git configuration."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess


def fingerprint(root):
    root = Path(root).resolve()
    def git(*args):
        return subprocess.check_output(['git', '-C', str(root), *args])
    git('rev-parse', '--git-dir')
    head = subprocess.run(['git', '-C', str(root), 'rev-parse', '--verify', '--quiet', 'HEAD'], capture_output=True, check=False)
    if head.returncode not in (0, 1):
        raise RuntimeError('Cannot resolve checkout HEAD')
    files = {}
    for directory, dirs, names in os.walk(root, followlinks=False):
        dirs[:] = sorted(d for d in dirs if d != '.git')
        for name in sorted(names + [d for d in dirs if (Path(directory) / d).is_symlink()]):
            path = Path(directory) / name
            if name == '.git':
                continue
            content = os.fsencode(os.readlink(path)) if path.is_symlink() else path.read_bytes()
            files[str(path.relative_to(root))] = {
                'sha256': hashlib.sha256(content).hexdigest(),
                'mode': path.lstat().st_mode,
            }
    return {
        'head': head.stdout.decode().strip() if head.returncode == 0 else None,
        'status_sha256': hashlib.sha256(git('status', '--porcelain=v1', '-z', '--untracked-files=all')).hexdigest(),
        'local_config_sha256': hashlib.sha256(git('config', '--local', '--null', '--list')).hexdigest(),
        'files': files,
    }


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True)
    parser.add_argument('--snapshot', type=Path, required=True)
    parser.add_argument('--verify', action='store_true')
    args = parser.parse_args()
    current = fingerprint(args.root)
    if args.verify:
        previous = json.loads(args.snapshot.read_text())
        if previous != current:
            raise SystemExit('NO-GO: checkout files, Git status, HEAD or local configuration changed')
        print(f'PRESERVED: {len(current["files"])} file fingerprints, HEAD, status and local Git configuration')
    else:
        with args.snapshot.open('x') as output:
            json.dump(current, output, sort_keys=True, indent=2)
        print(f'SNAPSHOTTED: {len(current["files"])} file fingerprints; configuration values not retained')
