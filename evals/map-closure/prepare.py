"""Create isolated, committed sources; never copies the evaluator oracle."""
import argparse
from pathlib import Path
import shutil
import subprocess

HERE = Path(__file__).resolve().parent


def prepare(output):
    output = Path(output)
    if output.exists():
        raise ValueError("Output must not exist")
    shutil.copytree(HERE / "fixture", output)
    for source in sorted((output / "sources").iterdir()):
        def git(*args):
            return subprocess.run(["git", *args], cwd=source, check=True, capture_output=True, text=True).stdout.strip()
        git("init", "-q")
        git("add", ".")
        git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture: freeze source")
        (output / f"{source.name}-revision.txt").write_text(git("rev-parse", "HEAD") + "\n")
    (output / "task.txt").write_text(
        "Use the candidate distribution's map-ecosystem completion procedure. "
        "This isolated workspace has two already accepted service-local maps in vault/. "
        "Complete the campaign's bounded local reconciliation and visible closure. "
        "Use sources/ read-only and preserve valid mapped knowledge; do not remap the services. "
        "There is no deployment or live infrastructure access. Do not request/install access for this run. "
        "Update vault/ as needed and write a concise result.md with resolved and remaining questions, "
        "evidence references, coverage and completion limits. Do not alter sources/.\n"
    )
    return output


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True)
    prepare(parser.parse_args().output)
