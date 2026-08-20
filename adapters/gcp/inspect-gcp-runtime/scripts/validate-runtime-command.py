#!/usr/bin/env python3
"""Validate a read-only gcloud or kubectl command without executing it."""

from __future__ import annotations

import argparse
import json
import re
import shlex
import sys
from dataclasses import dataclass


class CommandRejected(ValueError):
    """Raised when a candidate command violates the inspection boundary."""


@dataclass(frozen=True)
class Validation:
    tool: str
    action: str
    reason: str


VALUE_FLAGS = {
    "--account",
    "--access-token-file",
    "--as",
    "--as-group",
    "--billing-project",
    "--configuration",
    "--container",
    "--context",
    "--database",
    "--field-selector",
    "--filter",
    "--format",
    "--freshness",
    "--impersonate-service-account",
    "--kubeconfig",
    "--limit",
    "--location",
    "--namespace",
    "--output",
    "--project",
    "--raw",
    "--region",
    "--request-timeout",
    "--selector",
    "--since",
    "--tail",
    "--timeout",
    "--token",
    "--watch",
    "--zone",
    "-n",
    "-o",
}

DENIED_IDENTITY_FLAGS = {
    "--account",
    "--access-token-file",
    "--as",
    "--as-group",
    "--billing-project",
    "--certificate-authority",
    "--client-certificate",
    "--client-key",
    "--configuration",
    "--credential-file-override",
    "--flags-file",
    "--impersonate-service-account",
    "--kubeconfig",
    "--log-http",
    "--password",
    "--server",
    "--token",
    "--trace-token",
    "--user",
    "--username",
}

GCLOUD_ACTIONS = {
    "add",
    "apply",
    "connect",
    "create",
    "delete",
    "deploy",
    "describe",
    "disable",
    "enable",
    "export",
    "get-credentials",
    "help",
    "import",
    "insert",
    "list",
    "login",
    "patch",
    "read",
    "remove",
    "replace",
    "reset",
    "restart",
    "resume",
    "run",
    "set",
    "start",
    "stop",
    "suspend",
    "update",
}

KUBECTL_ACTIONS = {
    "annotate",
    "apply",
    "attach",
    "auth",
    "autoscale",
    "config",
    "cordon",
    "cp",
    "create",
    "debug",
    "delete",
    "describe",
    "drain",
    "edit",
    "exec",
    "expose",
    "get",
    "label",
    "logs",
    "patch",
    "port-forward",
    "proxy",
    "replace",
    "rollout",
    "run",
    "scale",
    "set",
    "taint",
    "top",
    "uncordon",
}

SAFE_KUBERNETES_RESOURCES = {
    "cronjob",
    "cronjobs",
    "daemonset",
    "daemonsets",
    "deployment",
    "deployments",
    "deploy",
    "ds",
    "endpoint",
    "endpoints",
    "endpointslice",
    "endpointslices",
    "event",
    "events",
    "horizontalpodautoscaler",
    "horizontalpodautoscalers",
    "hpa",
    "ingress",
    "ingresses",
    "job",
    "jobs",
    "namespace",
    "namespaces",
    "node",
    "nodes",
    "ns",
    "pod",
    "pods",
    "replicaset",
    "replicasets",
    "rs",
    "service",
    "services",
    "statefulset",
    "statefulsets",
    "sts",
    "svc",
}

CLUSTER_SCOPED_RESOURCES = {"namespace", "namespaces", "node", "nodes", "ns"}
SENSITIVE_RESOURCES = {
    "secret",
    "secrets",
    "serviceaccounttoken",
    "serviceaccounttokens",
}


def tokenize(command: str) -> list[str]:
    if "\n" in command or "\r" in command:
        raise CommandRejected("multi-line commands are not allowed")
    if re.search(r"<[A-Za-z][A-Za-z0-9_-]*>", command):
        raise CommandRejected("unresolved placeholders are not allowed")
    if "`" in command or "$(" in command or "${" in command:
        raise CommandRejected("shell expansion is not allowed")
    lexer = shlex.shlex(command, posix=True, punctuation_chars="|&;<>")
    lexer.whitespace_split = True
    lexer.commenters = ""
    try:
        tokens = list(lexer)
    except ValueError as error:
        raise CommandRejected(f"invalid shell quoting: {error}") from error
    if not tokens:
        raise CommandRejected("command is empty")
    if any(token and set(token) <= set("|&;<>") for token in tokens):
        raise CommandRejected("shell operators and redirection are not allowed")
    return tokens


def flag_value(tokens: list[str], *names: str) -> str | None:
    for index, token in enumerate(tokens):
        for name in names:
            if token.startswith(name + "="):
                return token.split("=", 1)[1]
            if token == name and index + 1 < len(tokens):
                value = tokens[index + 1]
                if not value.startswith("-"):
                    return value
    return None


def has_flag(tokens: list[str], *names: str) -> bool:
    return any(
        token == name or token.startswith(name + "=")
        for token in tokens
        for name in names
    )


def command_words(tokens: list[str]) -> list[str]:
    words: list[str] = []
    skip_value = False
    for token in tokens[1:]:
        if skip_value:
            skip_value = False
            continue
        if token.startswith("-"):
            name = token.split("=", 1)[0]
            if "=" not in token and name in VALUE_FLAGS:
                skip_value = True
            continue
        words.append(token)
    return words


def first_action(words: list[str], actions: set[str]) -> str | None:
    return next((word for word in words if word in actions), None)


def require_projected_format(tokens: list[str]) -> None:
    value = flag_value(tokens, "--format")
    if value is None:
        raise CommandRejected("a projected --format is required")
    if not re.fullmatch(r"(?:json|table|value|csv)\(.+\)", value):
        raise CommandRejected("--format must project explicit fields")
    lowered = value.lower()
    sensitive = (
        "access_token",
        "authorization",
        "credential",
        "id_token",
        "password",
        "private_key",
        "privatekey",
        "secret",
        "stringdata",
        ".env",
        "environmentvariables",
    )
    if any(marker in lowered for marker in sensitive):
        raise CommandRejected("--format projects a sensitive field")


def require_positive_limit(tokens: list[str], *, maximum: int) -> None:
    raw = flag_value(tokens, "--limit")
    if raw is None:
        raise CommandRejected("--limit is required")
    try:
        value = int(raw)
    except ValueError as error:
        raise CommandRejected("--limit must be an integer") from error
    if value < 1 or value > maximum:
        raise CommandRejected(f"--limit must be between 1 and {maximum}")


def require_quiet(tokens: list[str]) -> None:
    if not has_flag(tokens, "--quiet", "-q"):
        raise CommandRejected("--quiet is required for gcloud execution")


def ensure_no_identity_override(tokens: list[str]) -> None:
    for flag in DENIED_IDENTITY_FLAGS:
        if has_flag(tokens, flag):
            raise CommandRejected(f"{flag} is outside the current-identity boundary")


def validate_gcloud(tokens: list[str]) -> Validation:
    words = command_words(tokens)
    if not words:
        raise CommandRejected("gcloud leaf command is missing")
    if words[0] == "help":
        if len(words) < 3:
            raise CommandRejected("help must target a leaf command")
        return Validation("gcloud", "help", "leaf-level syntax lookup")

    ensure_no_identity_override(tokens)
    if words[0] in {"alpha", "beta"}:
        raise CommandRejected("alpha and beta command surfaces are outside this skill")

    action = first_action(words, GCLOUD_ACTIONS)
    if action is None:
        raise CommandRejected("only explicit read actions are allowed")

    if words[:2] == ["auth", "list"]:
        require_quiet(tokens)
        if flag_value(tokens, "--filter") != "status:ACTIVE":
            raise CommandRejected("auth list must filter to the active identity")
        if flag_value(tokens, "--format") != "value(account)":
            raise CommandRejected("auth list must project only the account field")
        return Validation("gcloud", "auth-list", "active identity read")

    if words[0] in {
        "auth",
        "billing",
        "components",
        "config",
        "iam",
        "kms",
        "organizations",
        "secrets",
        "services",
    }:
        raise CommandRejected(f"gcloud group {words[0]} is outside runtime inspection")

    if action not in {"list", "describe", "read"}:
        raise CommandRejected(f"gcloud action {action} can change state or data")
    if flag_value(tokens, "--project") is None:
        raise CommandRejected("an explicit --project is required")
    require_quiet(tokens)
    require_projected_format(tokens)

    if action == "list":
        if words[:2] == ["projects", "list"]:
            raise CommandRejected("project discovery is outside the resolved target card")
        if words[:2] == ["sql", "users"]:
            raise CommandRejected("Cloud SQL user inventory is outside runtime inspection")
        require_positive_limit(tokens, maximum=200)
        return Validation("gcloud", "list", "bounded control-plane collection read")

    if action == "describe":
        action_index = words.index("describe")
        has_flag_target = flag_value(tokens, "--database") is not None
        if len(words) <= action_index + 1 and not has_flag_target:
            raise CommandRejected("describe requires an exact resource name")
        if words[:2] == ["projects", "describe"]:
            if words[action_index + 1] != flag_value(tokens, "--project"):
                raise CommandRejected("projects describe target must match --project")
        location_sensitive = (
            words[:2] == ["container", "clusters"]
            or words[:2] == ["run", "services"]
            or words[0] == "functions"
            or words[:2] == ["scheduler", "jobs"]
            or words[:2] == ["eventarc", "triggers"]
        )
        if location_sensitive and not has_flag(tokens, "--zone", "--region", "--location"):
            raise CommandRejected("an explicit location flag is required")
        return Validation("gcloud", "describe", "scoped control-plane resource read")

    if words[:2] != ["logging", "read"]:
        raise CommandRejected("read is allowed only for Cloud Logging")
    action_index = words.index("read")
    if len(words) <= action_index + 1:
        raise CommandRejected("logging read requires an LQL filter")
    query = words[action_index + 1]
    if re.search(r'resource\.type\s*=\s*"[^"]+"', query) is None:
        raise CommandRejected("logging read must bind one exact resource.type")
    if "protoPayload.methodName" in query:
        raise CommandRejected("audit methodName filters require independent verification")
    exact_label = re.search(
        r'resource\.labels\.[A-Za-z0-9_]+\s*=\s*"[^"]+"',
        query,
    )
    exact_log_name = re.search(r'logName\s*=\s*"[^"]+"', query)
    exact_log_id = re.search(r'log_id\("[^"]+"\)', query)
    if exact_label is None and exact_log_name is None and exact_log_id is None:
        raise CommandRejected("logging read must bind a target resource label or log")
    if flag_value(tokens, "--freshness") is None:
        raise CommandRejected("--freshness is required for logging read")
    require_positive_limit(tokens, maximum=200)
    return Validation("gcloud", "logging-read", "bounded target-specific log read")


def resource_bases(resource_token: str) -> set[str]:
    return {
        item.split("/", 1)[0].lower()
        for item in resource_token.split(",")
        if item
    }


def require_context_and_timeout(tokens: list[str]) -> None:
    if flag_value(tokens, "--context") is None:
        raise CommandRejected("an explicit --context is required")
    if flag_value(tokens, "--request-timeout") is None:
        raise CommandRejected("--request-timeout is required")


def require_namespace(tokens: list[str]) -> None:
    if not has_flag(tokens, "--namespace", "-n", "--all-namespaces", "-A"):
        raise CommandRejected("a namespace or intentional all-namespace scope is required")


def validate_resource(resource_token: str) -> set[str]:
    bases = resource_bases(resource_token)
    if bases & SENSITIVE_RESOURCES:
        raise CommandRejected("secret and token resources are not readable by this skill")
    unsupported = bases - SAFE_KUBERNETES_RESOURCES
    if unsupported:
        raise CommandRejected(
            "unsupported Kubernetes resource: " + ", ".join(sorted(unsupported))
        )
    return bases


def require_safe_kubectl_output(tokens: list[str]) -> None:
    output = flag_value(tokens, "--output", "-o")
    if output is None or not (
        output == "name"
        or output.startswith("custom-columns=")
        or output.startswith("jsonpath=")
    ):
        raise CommandRejected("get must use name, custom-columns, or jsonpath output")
    lowered = output.lower()
    sensitive = (
        "authorization",
        "credential",
        "password",
        "privatekey",
        "secret",
        "stringdata",
        "token",
        ".env",
        "envfrom",
    )
    if any(marker in lowered for marker in sensitive):
        raise CommandRejected("Kubernetes output projects a sensitive field")


def validate_kubectl(tokens: list[str]) -> Validation:
    ensure_no_identity_override(tokens)
    words = command_words(tokens)
    if words[:2] == ["config", "get-contexts"]:
        if flag_value(tokens, "--output", "-o") != "name":
            raise CommandRejected("config get-contexts must use -o name")
        return Validation("kubectl", "config-get-contexts", "local context-name read")

    require_context_and_timeout(tokens)
    action = first_action(words, KUBECTL_ACTIONS)
    if action is None:
        raise CommandRejected("only explicit Kubernetes read actions are allowed")

    if action == "get" and flag_value(tokens, "--raw") == "/version":
        return Validation("kubectl", "get-version", "Kubernetes API reachability read")

    if action == "get":
        if len(words) < 2:
            raise CommandRejected("get requires a resource")
        bases = validate_resource(words[1])
        if any(base not in CLUSTER_SCOPED_RESOURCES for base in bases):
            require_namespace(tokens)
        require_safe_kubectl_output(tokens)
        return Validation("kubectl", "get", "scoped projected Kubernetes resource read")

    if action == "logs":
        if len(words) < 2:
            raise CommandRejected("logs requires an exact pod or workload target")
        require_namespace(tokens)
        if has_flag(tokens, "--follow", "-f"):
            raise CommandRejected("streaming logs are not allowed")
        if flag_value(tokens, "--since") is None:
            raise CommandRejected("--since is required for logs")
        raw_tail = flag_value(tokens, "--tail")
        if raw_tail is None:
            raise CommandRejected("--tail is required for logs")
        try:
            tail = int(raw_tail)
        except ValueError as error:
            raise CommandRejected("--tail must be an integer") from error
        if tail < 1 or tail > 500:
            raise CommandRejected("--tail must be between 1 and 500")
        return Validation("kubectl", "logs", "bounded Kubernetes workload log read")

    if action == "top":
        if len(words) < 2 or words[1] not in {"pod", "pods", "node", "nodes"}:
            raise CommandRejected("top is limited to pods or nodes")
        if words[1] in {"pod", "pods"}:
            require_namespace(tokens)
        return Validation("kubectl", "top", "point-in-time Metrics API read")

    if action == "auth" and words[:2] == ["auth", "can-i"]:
        if len(words) < 4:
            raise CommandRejected("auth can-i requires a verb and resource")
        return Validation("kubectl", "auth-can-i", "Kubernetes RBAC capability read")

    if action == "rollout" and words[:2] == ["rollout", "status"]:
        if len(words) < 3:
            raise CommandRejected("rollout status requires an exact workload")
        require_namespace(tokens)
        if flag_value(tokens, "--watch") != "false":
            raise CommandRejected("rollout status must use --watch=false")
        return Validation("kubectl", "rollout-status", "non-watching rollout state read")

    raise CommandRejected(f"kubectl action {action} can change state or access data")


def validate_command(command: str) -> Validation:
    tokens = tokenize(command)
    if tokens[0] == "gcloud":
        return validate_gcloud(tokens)
    if tokens[0] == "kubectl":
        return validate_kubectl(tokens)
    raise CommandRejected("command must start with gcloud or kubectl")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--command", required=True, help="single candidate command")
    args = parser.parse_args()
    try:
        result = validate_command(args.command)
    except CommandRejected as error:
        print(
            json.dumps(
                {"allowed": False, "reason": str(error)},
                ensure_ascii=False,
                sort_keys=True,
            )
        )
        return 2
    print(
        json.dumps(
            {
                "action": result.action,
                "allowed": True,
                "reason": result.reason,
                "tool": result.tool,
            },
            ensure_ascii=False,
            sort_keys=True,
        )
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
