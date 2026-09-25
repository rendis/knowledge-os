"""Regression check of `vaultctl discover` against installed cell vaults (local, never installed in cells).

Usage: python3 -B evals/discovery/run_real.py --vaultctl PATH VAULT [VAULT ...]

It runs discovery with stored judgments only (--classify off, no model calls), then prints, per vault,
the relations that the notes declare and discovery supports, the discrepancies, pending items and
cell gaps. Compare the totals with results.md before and after a change to the extraction code.
"""
import argparse
import json
import subprocess
import sys


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--vaultctl", required=True)
    p.add_argument("vaults", nargs="+")
    a = p.parse_args()
    totals = {"supported_relations": 0, "discrepancies": 0, "resources_without_topic_note": 0}
    for v in a.vaults:
        out = subprocess.run([a.vaultctl, "discover", "run", "--vault", v, "--classify", "off"], capture_output=True, text=True)
        if out.returncode != 0:
            print(f"{v}: discover failed: {out.stderr.strip()}", file=sys.stderr)
            return 1
        rep = json.loads(out.stdout)
        cmp = json.load(open(f"{rep['vault']}/.agents/state/discovery/comparison.json"))
        print(f"== {rep['vault'].rsplit('/', 1)[-1]}: repos={rep['repositories']} scanned={len(rep['scanned'])} failed={len(rep['failed'])} "
              f"pending_questions={sum(rep['pending_questions'].values())} duration={rep['duration']}")
        print(f"   comparison={rep['comparison']} pending_items={rep['pending_items']}")
        for c in cmp:
            for d in c["discrepancies"]:
                print(f"   DISCREPANCY {c['repo']} {d['field']} [[{d['target']}]] discovered={d['discovered'][:4]}")
        for k in totals:
            totals[k] += rep["comparison"].get(k, 0)
    print("TOTAL", json.dumps(totals))
    return 0


if __name__ == "__main__":
    sys.exit(main())
