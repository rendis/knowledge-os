#!/usr/bin/env python3
"""Platform providers against local emulators, with the clouds' real CLIs.

Google Cloud: the Pub/Sub emulator and `gcloud` (image google-cloud-cli:emulators).
AWS: moto and the AWS CLI (images motoserver/moto, amazon/aws-cli).
The CLIs run in containers on a private network; `kos` finds them on PATH as `gcloud` and `aws`,
exactly as it finds a developer's installed CLIs. Azure has no emulator of its management API (ARM),
so its provider is covered by the Go unit tests with the documented `az` output shapes.

Requires Docker and Go; skipped when Docker is unavailable. Run: make test-platform
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import unittest
import uuid

DIST = Path(__file__).resolve().parents[2]
GCLOUD_IMAGE = "gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators"
MOTO_IMAGE = "motoserver/moto:latest"
AWS_IMAGE = "amazon/aws-cli:latest"
PROJECT = "acme-orders-prd"
ACCOUNT, REGION = "123456789012", "us-east-1"  # moto's default account


def docker_available():
    return shutil.which("docker") and subprocess.run(["docker", "info"], capture_output=True).returncode == 0


def sh(*args, env=None, check=True):
    result = subprocess.run(list(args), capture_output=True, text=True, env=env, timeout=300)
    if check and result.returncode != 0:
        raise AssertionError(f"{args}: {result.stdout}{result.stderr}")
    return result


@unittest.skipUnless(docker_available(), "Docker is not available")
class PlatformEmulatorTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = Path(tempfile.mkdtemp(prefix="vaultplat-"))
        run = uuid.uuid4().hex[:8]
        cls.net, cls.pubsub, cls.moto = f"vaultplat-{run}", f"vaultplat-pubsub-{run}", f"vaultplat-moto-{run}"
        sh("docker", "network", "create", cls.net)
        sh("docker", "run", "-d", "--name", cls.pubsub, "--network", cls.net, GCLOUD_IMAGE,
           "gcloud", "beta", "emulators", "pubsub", "start", "--host-port=0.0.0.0:8085", f"--project={PROJECT}")
        sh("docker", "run", "-d", "--name", cls.moto, "--network", cls.net, MOTO_IMAGE)
        cls.bin = cls.tmp / "bin"
        cls.bin.mkdir()
        wrappers = {
            "gcloud": f"docker run --rm -i --network {cls.net} -e CLOUDSDK_API_ENDPOINT_OVERRIDES_PUBSUB=http://{cls.pubsub}:8085/ "
                      f"-e CLOUDSDK_AUTH_ACCESS_TOKEN=emulator -e CLOUDSDK_CORE_DISABLE_PROMPTS=1 {GCLOUD_IMAGE} gcloud",
            "bq": f"docker run --rm -i --network {cls.net} -e CLOUDSDK_AUTH_ACCESS_TOKEN=emulator {GCLOUD_IMAGE} bq",
            "aws": f"docker run --rm -i --network {cls.net} -e AWS_ENDPOINT_URL=http://{cls.moto}:5000 -e AWS_ACCESS_KEY_ID=test "
                   f"-e AWS_SECRET_ACCESS_KEY=test -e AWS_DEFAULT_REGION={REGION} {AWS_IMAGE}",
        }
        for name, command in wrappers.items():
            path = cls.bin / name
            path.write_text(f'#!/bin/sh\nexec {command} "$@"\n')
            path.chmod(0o755)
        cls.env = {**os.environ, "PATH": f"{cls.bin}{os.pathsep}{os.environ['PATH']}"}
        cls.cli = cls.tmp / "kos"
        subprocess.run(["go", "build", "-o", str(cls.cli), "./cmd/kos"], cwd=DIST, check=True)
        cls.wait_ready()
        cls.seed()
        cls.vault = cls.make_vault()

    @classmethod
    def tearDownClass(cls):
        subprocess.run(["docker", "rm", "-f", cls.pubsub, cls.moto], capture_output=True)
        subprocess.run(["docker", "network", "rm", cls.net], capture_output=True)
        shutil.rmtree(cls.tmp, ignore_errors=True)

    @classmethod
    def tool(cls, *args, check=True):
        return sh(*args, env=cls.env, check=check)

    @classmethod
    def wait_ready(cls):
        deadline = time.time() + 120
        while time.time() < deadline:
            gcp = cls.tool("gcloud", "pubsub", "topics", "list", "--project", PROJECT, "--format=json", check=False)
            aws = cls.tool("aws", "sns", "list-topics", check=False)
            if gcp.returncode == 0 and aws.returncode == 0:
                return
            time.sleep(2)
        raise AssertionError("emulators did not start")

    @classmethod
    def seed(cls):
        g = lambda *a: cls.tool("gcloud", "pubsub", *a, "--project", PROJECT)
        g("topics", "create", "orders-in")
        g("topics", "create", "orders-dlq")
        g("subscriptions", "create", "orders-cl-sub", "--topic", "orders-in",
          "--message-filter", 'attributes.eventType="orderConfirmed"', "--dead-letter-topic", "orders-dlq")
        a = lambda *x: json.loads(cls.tool("aws", *x, "--output", "json").stdout or "{}")
        topic = a("sns", "create-topic", "--name", "orders-events")["TopicArn"]
        queue = a("sqs", "create-queue", "--queue-name", "orders-cl-queue")["QueueUrl"]
        dlq = a("sqs", "create-queue", "--queue-name", "orders-dlq")["QueueUrl"]
        arn = lambda url: a("sqs", "get-queue-attributes", "--queue-url", url, "--attribute-names", "QueueArn")["Attributes"]["QueueArn"]
        redrive = json.dumps({"deadLetterTargetArn": arn(dlq), "maxReceiveCount": "5"})
        a("sqs", "set-queue-attributes", "--queue-url", queue, "--attributes", json.dumps({"RedrivePolicy": redrive}))
        a("sns", "subscribe", "--topic-arn", topic, "--protocol", "sqs", "--notification-endpoint", arn(queue),
          "--attributes", json.dumps({"FilterPolicy": json.dumps({"eventType": ["orderConfirmed", "orderCancelled"]})}))
        a("dynamodb", "create-table", "--table-name", "orders-state", "--attribute-definitions", "AttributeName=id,AttributeType=S",
          "--key-schema", "AttributeName=id,KeyType=HASH", "--billing-mode", "PAY_PER_REQUEST")
        a("rds", "create-db-instance", "--db-instance-identifier", "orders-db", "--db-instance-class", "db.t3.micro",
          "--engine", "postgres", "--db-name", "orders", "--master-username", "admin", "--master-user-password", "emulator-only-1")
        a("s3api", "create-bucket", "--bucket", "acme-orders-exports")

    @classmethod
    def make_vault(cls):
        sources = cls.tmp / "sources"
        repo = sources / "SVC-orders"
        (repo / "config").mkdir(parents=True)
        (repo / "go.mod").write_text("module example.com/orders\n")
        (repo / "main.go").write_text('package main\n\nimport "github.com/aws/aws-sdk-go-v2/service/dynamodb"\n\n'
                                      'var _ = dynamodb.NewFromConfig\n\nconst event = "orderConfirmed"\nconst table = "orders-state"\n')
        (repo / "config" / "app.env").write_text(
            f"GCP_SUBSCRIPTION=projects/{PROJECT}/subscriptions/orders-cl-sub\n"
            f"QUEUE_URL=https://sqs.{REGION}.amazonaws.com/{ACCOUNT}/orders-cl-queue\n"
            f"LEGACY_TOPIC=arn:aws:sns:{REGION}:{ACCOUNT}:ghost-events\n"
            "EXPORT_BUCKET=acme-orders-exports\nDB_HOST=orders-db.emulator.local\n")
        git = lambda *x: sh("git", "-C", str(repo), "-c", "user.email=t@t", "-c", "user.name=t", *x)
        git("init", "-q", "-b", "main")
        git("add", "-A")
        git("commit", "-qm", "init")
        vault = cls.tmp / "vault"
        sh(str(cls.cli), "init", "--vault", str(vault), "--yes", "--cell-name", "Orders",
           "--purpose", "Order processing.", "--system", "orders:Orders", "--repo-prefix", "SVC-",
           "--platform", "gcp", "--platform", "aws")
        subprocess.run([str(cls.cli), "config", "--vault", str(vault), "workspace-init", "--repository-root", str(sources)],
                       check=True, capture_output=True)
        return vault

    def kos(self, *args):
        result = subprocess.run([str(self.cli), *args, "--vault", str(self.vault)], capture_output=True, text=True, env=self.env, timeout=600)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return json.loads(result.stdout)

    def test_referenced_scopes_are_captured_and_wire_the_facts(self):
        report = self.kos("discover", "run", "--classify", "off")
        self.assertEqual(sorted(report["platform_scopes_referenced"]), [f"aws:{ACCOUNT}/{REGION}", f"gcp:{PROJECT}"])

        captured = self.kos("discover", "platform", "--referenced")["captured"]
        self.assertEqual(captured[f"aws:{ACCOUNT}/{REGION}"], "ok")
        # The Pub/Sub emulator serves messaging only; the other Google services are recorded as unreadable.
        self.assertTrue(captured[f"gcp:{PROJECT}"].startswith("ok ("), captured)
        snapshots = {p.name: json.loads(p.read_text()) for p in (self.vault / "90-Meta/discovery/platform").glob("*.json")}
        self.assertLessEqual({f"aws-{ACCOUNT}-{REGION}.json", f"gcp-{PROJECT}.json"}, set(snapshots))

        # Without Jev the agent answers the judgments; here only the DynamoDB client matters.
        questions = self.kos("discover", "questions", "--kind", "dependency")["questions"]
        answers = [{"id": q["id"], "choice": "document_db", "confidence": 1} for q in questions
                   if any("dynamodb" in path for path in q["state"].get("imported_paths", []))]
        self.assertTrue(answers, questions)
        answer_file = self.tmp / "answers.json"
        answer_file.write_text(json.dumps(answers))
        self.kos("discover", "answer", "--file", str(answer_file))
        self.kos("discover", "run", "--classify", "off")
        facts = self.kos("discover", "report", "--repo", "SVC-orders")["facts"]
        resources = {r["name"].rsplit("/", 1)[-1]: r for r in facts["resources"]}
        gcp = resources["orders-cl-sub"]
        self.assertEqual((gcp["type"], gcp["direction"], gcp["topic"], gcp.get("events")),
                         ("message_subscription", "consume", f"projects/{PROJECT}/topics/orders-in", ["orderConfirmed"]))
        aws = resources["orders-cl-queue"]
        self.assertEqual((aws["type"], aws["direction"], aws["topic"]),
                         ("message_subscription", "consume", f"arn:aws:sns:{REGION}:{ACCOUNT}:orders-events"))
        self.assertEqual(sorted(aws["events"]), ["orderCancelled", "orderConfirmed"])
        aws_snap = snapshots[f"aws-{ACCOUNT}-{REGION}.json"]
        self.assertEqual(aws_snap["kinds"], {"messaging": "ok", "document_db": "ok", "sql_db": "ok", "object_storage": "ok"})
        data = {(r["kind"], r["type"]): r for r in aws_snap["resources"]}
        self.assertTrue(data[("document_db", "table")]["name"].endswith(":table/orders-state"))
        self.assertIn("orders-db", data[("sql_db", "instance")]["aliases"])
        self.assertIn("acme-orders-exports", data[("object_storage", "bucket")]["aliases"])
        linked = {r["name"]: r for r in facts["resources"] if r["type"] in ("database_object", "storage_bucket")}
        for name, kind in (("orders-state", "database_object"), ("acme-orders-exports", "storage_bucket")):
            self.assertEqual(linked[name]["type"], kind)
            self.assertIn(f"aws:{ACCOUNT}/{REGION}", [e.get("scope") for e in linked[name]["evidence"] if e["kind"] == "platform"])
        pending = {p["subject"]: p["kind"] for p in facts["pending"]}
        self.assertEqual(pending.get(f"arn:aws:sns:{REGION}:{ACCOUNT}:ghost-events"), "not-in-platform")
        self.assertIn({"name": "orderConfirmed", "role": "publish-candidate"},
                      [{"name": e["name"], "role": e["role"]} for e in facts.get("events", [])])

    def test_credentials_of_another_account_are_recorded_as_denied(self):
        captured = self.kos("discover", "platform", "--provider", "aws", "--scope", f"210987654321/{REGION}")["captured"]
        self.assertEqual(captured, {f"aws:210987654321/{REGION}": "denied"})
        snap = json.loads((self.vault / f"90-Meta/discovery/platform/aws-210987654321-{REGION}.json").read_text())
        self.assertIn("belong to account", snap["detail"])
        self.assertTrue(snap["confirm_with"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
