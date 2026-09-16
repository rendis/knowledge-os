#!/usr/bin/env python3
"""Read or validate the related-vault catalog without modifying it."""
from __future__ import annotations

import argparse
import json
from pathlib import Path

from vault_catalog import CATALOG_PATH, CatalogError, load_catalog


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=['list', 'validate'])
    parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    args = parser.parse_args()
    try:
        catalog = load_catalog(args.root)
    except CatalogError as error:
        print(json.dumps({'status': 'invalid', 'error': str(error)}))
        return 2
    result = {'status': 'valid' if (args.root / CATALOG_PATH).exists() else 'absent',
              'count': len(catalog['vaults'])}
    if args.command == 'list':
        result['vaults'] = catalog['vaults']
    print(json.dumps(result, indent=2, ensure_ascii=False))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
