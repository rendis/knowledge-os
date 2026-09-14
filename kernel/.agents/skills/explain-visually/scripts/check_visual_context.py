#!/usr/bin/env python3
"""Check a retained visual's context and file correspondence, not semantic truth."""
import argparse
import hashlib
import json
from pathlib import Path
import re
from urllib.parse import unquote, urlsplit


def markdown_content(text):
    """Ignore comments/code examples; retain real Mermaid fences as embedded views."""
    visible = re.sub(r'<!--.*?-->', '', text, flags=re.S)
    mermaid = False
    lines, opening, body = [], None, []
    for line in visible.splitlines():
        if opening:
            close = re.fullmatch(r' {0,3}([`~]+)[ \t]*', line)
            if close and set(close[1]) == {opening[0]} and len(close[1]) >= opening[1]:
                mermaid = mermaid or (opening[2] == 'mermaid' and bool(''.join(body).strip()))
                opening, body = None, []
            else:
                body.append(line)
            continue
        fence = re.match(r' {0,3}(`{3,}|~{3,})(.*)$', line)
        if fence:
            opening = (fence[1][0], len(fence[1]), fence[2].strip())
        elif not line.startswith(('    ', '\t')):
            lines.append(line)
    visible = '\n'.join(lines)
    visible = re.sub(r'(`+).*?\1', '', visible, flags=re.S)
    embedded = mermaid or bool(re.search(r'<svg\b[^>]*>.*?</svg\s*>', visible, flags=re.S))
    return visible, embedded


def markdown_links(text):
    """Read inline and reference links used by the context template and CommonMark."""
    destination = r'(<[^>]+>|[^\s]+?)(?:[ \t]+[\"\'].*?[\"\'])?'
    definitions = {m[1].strip().casefold(): m[2].strip('<>') for m in
                   re.finditer(r'^ {0,3}\[([^]\n]+)\]:[ \t]*' + destination + r'[ \t]*$', text, re.M)}
    prose = re.sub(r'^ {0,3}\[[^]\n]+\]:.*$', '', text, flags=re.M)
    links = [m[1].strip('<>') for m in re.finditer(r'\]\(' + destination + r'\)', prose)]
    for match in re.finditer(r'\[([^]\n]+)\](?:\[([^]\n]*)\])?(?!\()', prose):
        label = (match[2] or match[1]).strip().casefold()
        if label in definitions:
            links.append(definitions[label])
    return links


def check(path):
    path = Path(path)
    errors = []
    try:
        text = path.read_text()
        markers = re.findall(r'<!-- visual-context (.*?) -->', text)
        if len(markers) != 1:
            raise ValueError('Expected exactly one visual-context metadata marker.')
        meta = json.loads(markers[0])
        if not isinstance(meta, dict):
            raise ValueError('Context metadata must be an object.')
        files = meta.get('files')
        sources = meta.get('sources')
        if not isinstance(sources, list) or not sources or not all(isinstance(s, str) and s.strip() for s in sources):
            errors.append('Declare supporting sources or explicit synthetic assumptions.')
        if not isinstance(files, list):
            raise ValueError('files must be a list.')
        if not files and meta.get('embedded') is not True:
            errors.append('A separate visual requires a file entry.')
        visible, embedded = markdown_content(text)
        if meta.get('embedded') is True and not embedded:
            errors.append('Embedded mode requires a diagram in this Markdown.')
        links = markdown_links(visible)
        local_links = set()
        for link in links:
            link = link.strip('<>')
            parsed = urlsplit(link)
            if parsed.scheme or parsed.netloc:
                if parsed.scheme not in ('https', 'http', 'mailto'):
                    errors.append(f'Non-portable link: {link}')
                continue
            target = unquote(parsed.path)
            if not target:
                continue
            if Path(target).is_absolute():
                errors.append(f'Absolute local link: {target}')
                continue
            local_links.add(target)
            if not (path.parent / target).is_file():
                errors.append(f'Missing linked file: {target}')
        seen = set()
        for entry in files:
            if not isinstance(entry, dict) or not isinstance(entry.get('path'), str):
                errors.append('Each file entry needs a relative path and SHA-256.')
                continue
            name = entry['path']
            relative = Path(name)
            if not name or relative.is_absolute() or '..' in relative.parts or urlsplit(name).scheme:
                errors.append(f'Artifact must be local to the context directory: {name}')
                continue
            if name in seen:
                errors.append(f'Duplicate artifact: {name}')
            seen.add(name)
            target = path.parent / relative
            if target.resolve() == path.resolve():
                errors.append('Context must not hash itself.')
                continue
            if target.is_symlink() or not target.is_file() or not target.resolve().is_relative_to(path.parent.resolve()):
                errors.append(f'Missing or non-local artifact: {name}')
                continue
            if name not in local_links:
                errors.append(f'Artifact needs a readable Markdown link: {name}')
            if hashlib.sha256(target.read_bytes()).hexdigest() != entry.get('sha256'):
                errors.append(f'Unreviewed file drift or invalid SHA-256: {name}')
        for name in local_links:
            target = path.parent / name
            if target.resolve().is_relative_to(path.parent.resolve()) and target.resolve() != path.resolve():
                if not any((path.parent / item).resolve() == target.resolve() for item in seen):
                    errors.append(f'Local companion needs a file entry and SHA-256: {name}')
        if files and not any(Path(name).stem == path.stem for name in seen):
            errors.append('Context must share its stem with a represented file.')
    except (OSError, ValueError, TypeError) as exc:
        errors.append(str(exc))
    return {'context_pass': not errors, 'errors': errors, 'semantic_review': 'required'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('context', type=Path)
    result = check(parser.parse_args().context)
    print(json.dumps(result, indent=2))
    raise SystemExit(0 if result['context_pass'] else 1)


if __name__ == '__main__':
    main()
