#!/usr/bin/env python3
"""
Seed KPI JSON registries by POSTing to mirador-core KPI bulk JSON API.
Usage: BASE_URL=http://localhost:8010 python3 scripts/localdev_seed_kpis.py
"""
import os
import sys
import json
import glob
import argparse
from urllib import request, parse

BASE_DIR = os.path.join(os.path.dirname(__file__), "..")
KPI_DIR = os.path.normpath(os.path.join(BASE_DIR, "deployments/localdev/kpi-seeding-definitions"))
SIG_DIR = os.path.normpath(os.path.join(BASE_DIR, "deployments/localdev/signal-defnitions"))


def normalize_item(raw):
    # raw may use different keys like kpi_name, kpi_formula, signal_type, etc.
    out = {}
    # simple key mappings
    mapping = {
        'kpi_name': 'name',
        'kpi_formula': 'formula',
        'kpi_definition': 'definition',
        'signal_type': 'signalType',
        'query_type': 'queryType',
        'datastore': 'datastore',
        'layer': 'layer',
        'classifier': 'classifier',
        'sentiment': 'sentiment',
        'kind': 'kind',
        'tags': 'tags',
        'namespace': 'namespace',
        'source': 'source',
        'sourceId': 'sourceId',
        'source_id': 'sourceId',
    }
    for k, v in raw.items():
        lk = k.lower()
        if k in mapping:
            out[mapping[k]] = v
        elif lk in mapping:
            out[mapping[lk]] = v
        else:
            # try snake to camel for common names
            if lk == 'kpi_name':
                out['name'] = v
            else:
                out[k] = v
    # defaults
    if 'kind' not in out or not out.get('kind'):
        out['kind'] = 'tech'
    # ensure tags is list
    if 'tags' in out and isinstance(out['tags'], str):
        parts = [p.strip() for p in out['tags'].replace(';',',').split(',') if p.strip()]
        out['tags'] = parts
    return out


def extract_items_from_file(path):
    with open(path, 'r') as fh:
        data = json.load(fh)
    items = []
    if isinstance(data, list):
        items = data
    elif isinstance(data, dict):
        # find first list value or merge all lists
        for v in data.values():
            if isinstance(v, list):
                items.extend(v)
        # if still empty, maybe the dict itself is a single item
        if not items:
            items = [data]
    else:
        items = [data]
    # normalize each
    out = [normalize_item(it) for it in items]
    return out


def post_payload(base_url, payload, resource='kpi'):
    # resource: 'kpi' or 'signal' -> build endpoint accordingly
    resource = resource.lower()
    if resource == 'signal' or resource == 'signals':
        path = '/api/v1/signal/defs/bulk-json'
    else:
        path = '/api/v1/kpi/defs/bulk-json'
    url = base_url.rstrip('/') + path
    body = json.dumps({'items': payload}).encode('utf-8')
    req = request.Request(url, data=body, headers={'Content-Type': 'application/json'}, method='POST')
    try:
        with request.urlopen(req, timeout=30) as resp:
            resp_body = resp.read().decode('utf-8')
            try:
                return resp.getcode(), json.loads(resp_body)
            except Exception:
                return resp.getcode(), resp_body
    except Exception as e:
        return None, str(e)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--base-url', default=os.environ.get('BASE_URL','http://localhost:8010'))
    parser.add_argument('--dir', default=KPI_DIR, help='Directory containing KPI JSON files')
    parser.add_argument('--sig-dir', default=SIG_DIR, help='Directory containing signal JSON files')
    parser.add_argument('--seed-signals', action='store_true', help='Also seed signal definitions from --sig-dir')
    args = parser.parse_args()

    files = sorted(glob.glob(os.path.join(args.dir, '*.json')))
    if not files:
        print('No KPI JSON files found in', args.dir, file=sys.stderr)
        sys.exit(1)

    total = 0
    total_success = 0
    total_fail = 0

    for f in files:
        print('Seeding', f)
        items = extract_items_from_file(f)
        total += len(items)
        code, resp = post_payload(args.base_url, items, resource='kpi')
        if code is None:
            print('  POST failed:', resp)
            total_fail += len(items)
            continue
        # Treat any 2xx as success; prefer BulkSummary when present
        if 200 <= code < 300:
            if code == 204:
                # No content — assume all items were accepted (no-change or similar)
                print('  result: HTTP', code, '- no content (treated as success for', len(items), 'items)')
                total_success += len(items)
                continue
            if isinstance(resp, dict):
                sc = resp.get('successCount') or resp.get('success_count') or 0
                fc = resp.get('failureCount') or resp.get('failure_count') or 0
                print('  result: HTTP', code, '- success:', sc, ' failure:', fc)
                total_success += sc
                total_fail += fc
            else:
                # unknown response body but 2xx status: treat as all succeeded
                print('  result: HTTP', code, '- response:', resp)
                total_success += len(items)
        else:
            print('  result: HTTP', code, '- response:', resp)
            total_fail += len(items)

    # optionally seed signals
    if args.seed_signals:
        s_files = sorted(glob.glob(os.path.join(args.sig_dir, '*.json')))
        if not s_files:
            print('No signal JSON files found in', args.sig_dir, file=sys.stderr)
        else:
            print('\nSeeding signal definitions from', args.sig_dir)
            s_total = 0
            s_succ = 0
            s_fail = 0
            for f in s_files:
                print('Seeding', f)
                items = extract_items_from_file(f)
                s_total += len(items)
                code, resp = post_payload(args.base_url, items, resource='signal')
                if code is None:
                    print('  POST failed:', resp)
                    s_fail += len(items)
                    continue
                if 200 <= code < 300:
                    if code == 204:
                        print('  result: HTTP', code, '- no content (treated as success for', len(items), 'items)')
                        s_succ += len(items)
                        continue
                    if isinstance(resp, dict):
                        sc = resp.get('successCount') or resp.get('success_count') or 0
                        fc = resp.get('failureCount') or resp.get('failure_count') or 0
                        print('  result: HTTP', code, '- success:', sc, ' failure:', fc)
                        s_succ += sc
                        s_fail += fc
                    else:
                        print('  result: HTTP', code, '- response:', resp)
                        s_succ += len(items)
                else:
                    print('  result: HTTP', code, '- response:', resp)
                    s_fail += len(items)
            print('Signal Summary: total items:', s_total, 'succeeded:', s_succ, 'failed:', s_fail)

    print('KPI Summary: total items:', total, 'succeeded:', total_success, 'failed:', total_fail)

if __name__ == '__main__':
    main()
