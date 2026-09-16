"""Validate the optional, consumer-owned related-vault catalog."""
from __future__ import annotations

import re
from pathlib import Path
from urllib.parse import urlsplit

from instance import InstanceError, _parse_simple_yaml

CATALOG_PATH = Path('90-Meta/vault-catalog.yaml')
FIELDS = {'id', 'repository', 'domain', 'scope', 'summary', 'tags', 'relationship', 'consult_when'}


class CatalogError(ValueError):
    """Invalid portable catalog; messages never echo submitted values."""


def repository_identity(value: str) -> str:
    """Normalize credential-free HTTPS/SSH Git remotes for duplicate detection."""
    if re.search(r'\s|[?#%\\]', value):
        raise CatalogError('repository must be a credential-free Git remote')
    scp = re.fullmatch(r'git@([A-Za-z0-9.-]+):([A-Za-z0-9._/-]+)', value)
    if scp:
        host, path = scp.groups()
        port = None
    else:
        try:
            parsed = urlsplit(value)
            port = parsed.port
        except ValueError as error:
            raise CatalogError('invalid repository URL') from error
        if (parsed.scheme not in {'https', 'ssh'} or not parsed.hostname
                or parsed.password or (parsed.scheme == 'https' and parsed.username)
                or (parsed.scheme == 'ssh' and parsed.username not in {None, 'git'})):
            raise CatalogError('repository must be a credential-free HTTPS or SSH Git remote')
        host, path = parsed.hostname, parsed.path.lstrip('/')
        if port == (443 if parsed.scheme == 'https' else 22):
            port = None
    path = path.rstrip('/')
    if path.casefold().endswith('.git'):
        path = path[:-4]
    if not path or any(part in {'', '.', '..'} for part in path.split('/')):
        raise CatalogError('repository must identify a repository path')
    # Match workspace-config.locate_repository's case-insensitive path identity.
    return f'{host.lower()}{":" + str(port) if port else ""}/{path.lower()}'


def validate_catalog(data: object) -> dict:
    if not isinstance(data, dict) or set(data) != {'version', 'vaults'}:
        raise CatalogError('catalog requires exactly version and vaults')
    if type(data['version']) is not int or data['version'] != 1:
        raise CatalogError('unsupported catalog version')
    if not isinstance(data['vaults'], list):
        raise CatalogError('vaults must be a list')
    ids, repositories = set(), set()
    for number, entry in enumerate(data['vaults'], 1):
        if not isinstance(entry, dict) or set(entry) != FIELDS:
            raise CatalogError(f'entry {number}: unexpected or missing fields')
        for field in FIELDS - {'tags', 'consult_when'}:
            if not isinstance(entry[field], str) or not entry[field].strip():
                raise CatalogError(f'entry {number}: {field} must be nonempty text')
        if not re.fullmatch(r'[a-z0-9]+(?:-[a-z0-9]+)*', entry['id']):
            raise CatalogError(f'entry {number}: id must be lowercase kebab-case')
        for field in ('tags', 'consult_when'):
            values = entry[field]
            if (not isinstance(values, list) or not values
                    or any(not isinstance(v, str) or not v.strip() for v in values)):
                raise CatalogError(f'entry {number}: {field} must be a nonempty text list')
            if len({v.strip().casefold() for v in values}) != len(values):
                raise CatalogError(f'entry {number}: duplicate {field}')
        identity = repository_identity(entry['repository'])
        if entry['id'] in ids or identity in repositories:
            raise CatalogError(f'entry {number}: duplicate vault identity or repository')
        ids.add(entry['id'])
        repositories.add(identity)
    return data


def load_catalog(root: Path) -> dict:
    path = root / CATALOG_PATH
    if path.is_symlink():
        raise CatalogError('catalog must be a regular versionable file')
    if not path.exists():
        return {'version': 1, 'vaults': []}
    try:
        data = _parse_simple_yaml(path.read_text(encoding='utf-8'))
    except (OSError, UnicodeError, InstanceError) as error:
        raise CatalogError('cannot read catalog or invalid YAML syntax') from error
    return validate_catalog(data)
