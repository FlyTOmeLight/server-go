"""Document omission defaults applied before validating the note request."""

import json
from pathlib import Path


def main():
    path = Path(__file__).resolve().parents[1] / 'doc/openapi/PostgreSQL_swagger.json'
    document = json.loads(path.read_text(encoding='utf-8'))
    for schema in document.get('definitions', {}).values():
        properties = schema.get('properties', {})
        required = [name for name in schema.get('required', [])
                    if 'default' not in properties.get(name, {})]
        if required:
            schema['required'] = required
        else:
            schema.pop('required', None)
    content = json.dumps(document, ensure_ascii=False, indent=4) + '\n'
    path.write_text(content, encoding='utf-8')
    # JSON is valid YAML, as in the repository's generated-project documents.
    path.with_suffix('.yaml').write_text(content, encoding='utf-8')


if __name__ == '__main__':
    main()
