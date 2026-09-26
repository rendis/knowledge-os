#!/usr/bin/env sh
# Build the synthetic "Acme Orders" demo cell used by the README recordings.
# Everything is fictional and offline: local Git repositories with fake origins
# and a hand-written platform snapshot. Usage: setup.sh DEMO_DIR
set -eu

DIST=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
DEMO=${1:?usage: setup.sh DEMO_DIR}
rm -rf "$DEMO" && mkdir -p "$DEMO/bin" "$DEMO/repos" "$DEMO/worktrees"
ln -s "$DIST/dist/kos-$(go env GOOS)-$(go env GOARCH)" "$DEMO/bin/kos"
ln -s "$DIST/docs/demo/stage.sh" "$DEMO/bin/demo-stage"
export PATH="$DEMO/bin:$PATH" KOS_NO_UPDATE_CHECK=1
# Fixed identity and dates keep commit hashes, and so the recordings, stable.
export GIT_AUTHOR_NAME=demo GIT_AUTHOR_EMAIL=demo@example.com GIT_COMMITTER_NAME=demo GIT_COMMITTER_EMAIL=demo@example.com
export GIT_AUTHOR_DATE=2026-09-01T10:00:00Z GIT_COMMITTER_DATE=2026-09-01T10:00:00Z

repo() { # repo NAME FILE CONTENT [FILE CONTENT]...
  dir="$DEMO/repos/$1"; name=$1; shift
  mkdir -p "$dir"
  while [ $# -gt 0 ]; do mkdir -p "$dir/$(dirname "$1")"; printf '%b' "$2" > "$dir/$1"; shift 2; done
  git -C "$dir" init -q -b main
  git -C "$dir" remote add origin "https://github.com/acme/$name.git"
  git -C "$dir" add -A && git -C "$dir" commit -q -m "feat: initial service"
}

repo acme-orders \
  go.mod 'module example.com/orders\n\nrequire cloud.google.com/go/pubsub v1.36.0\n' \
  cmd/main.go 'package main\n\nimport "cloud.google.com/go/pubsub"\n\nvar _ = pubsub.NewClient\n\nconst paid = "orderPaid"\n' \
  k8s/prod/env 'ORDER_EVENTS_TOPIC=projects/acme-prd/topics/order-events\n'
repo acme-ledger \
  go.mod 'module example.com/ledger\n\nrequire cloud.google.com/go/pubsub v1.36.0\n' \
  cmd/main.go 'package main\n\nimport "cloud.google.com/go/pubsub"\n\nvar _ = pubsub.NewClient\n' \
  k8s/prod/env 'ORDER_EVENTS_SUB=projects/acme-prd/subscriptions/ledger-order-events-sub\n'
repo acme-notifier \
  go.mod 'module example.com/notifier\n\nrequire cloud.google.com/go/pubsub v1.36.0\n' \
  cmd/main.go 'package main\n\nimport "cloud.google.com/go/pubsub"\n\nvar _ = pubsub.NewClient\n' \
  k8s/prod/env 'ORDER_EVENTS_SUB=projects/acme-prd/subscriptions/notifier-order-events-sub\n'

V="$DEMO/acme-vault"
"$DIST/install.sh" init --dest "$V" --yes --cell-name "Acme Orders" \
  --purpose "Order intake, payment and settlement." --system orders:Orders \
  --locale en --repo-prefix acme --platform gcp >/dev/null
kos config --vault "$V" workspace-init --repository-root "$DEMO/repos" \
  --development-worktree-root "$DEMO/worktrees" >/dev/null
git -C "$V" init -q -b main && git -C "$V" add -A && git -C "$V" commit -q -m "chore: create the Acme Orders vault"

# A read-only stand-in for the project's Google Cloud: the provider calls gcloud and bq.
cat > "$DEMO/bin/gcloud" <<'STUB'
#!/usr/bin/env sh
p=projects/acme-prd
case "$1 $2 $3" in
  "pubsub topics list") echo "[{\"name\":\"$p/topics/order-events\"}]" ;;
  "pubsub subscriptions list") echo "[{\"name\":\"$p/subscriptions/ledger-order-events-sub\",\"topic\":\"$p/topics/order-events\"},{\"name\":\"$p/subscriptions/notifier-order-events-sub\",\"topic\":\"$p/topics/order-events\"}]" ;;
  *) echo "[]" ;;
esac
STUB
printf '#!/usr/bin/env sh\necho "[]"\n' > "$DEMO/bin/bq"
chmod +x "$DEMO/bin/gcloud" "$DEMO/bin/bq"

mkdir -p "$V/.scratch"
cp "$DIST/docs/demo/answers.json" "$V/.scratch/answers.json"
printf 'Orders publish `order-events`, consumed by `ledger-order-events-sub` and `notifier-order-events-sub`.\nRefunds arrive on `refund-requested`.\n' > "$V/.scratch/answer.md"
