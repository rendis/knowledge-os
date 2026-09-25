package discover

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"documentation-vault/internal/config"
)

// fakeCLI answers provider commands from recorded outputs, keyed by the command line prefix.
func fakeCLI(t *testing.T, outputs map[string]string, failures map[string]string) {
	t.Helper()
	orig := runCLI
	t.Cleanup(func() { runCLI = orig })
	runCLI = func(name string, args ...string) ([]byte, string, error) {
		line := name + " " + strings.Join(args, " ")
		best := ""
		for k := range failures {
			if strings.HasPrefix(line, k) && len(k) > len(best) {
				best = k
			}
		}
		if best != "" {
			return nil, failures[best], errors.New("exit status 1")
		}
		for k := range outputs {
			if strings.HasPrefix(line, k) && len(k) > len(best) {
				best = k
			}
		}
		if best == "" {
			t.Fatalf("unexpected command: %s", line)
		}
		return []byte(outputs[best]), "", nil
	}
}

func TestProvidersParseCanonicalIdentifiers(t *testing.T) {
	cases := []struct{ id, provider, scope, kind, short string }{
		{"projects/acme-orders-prd/topics/orders-in", "gcp", "acme-orders-prd", "topic", "orders-in"},
		{"projects/acme-orders-prd/subscriptions/orders-cl-sub", "gcp", "acme-orders-prd", "subscription", "orders-cl-sub"},
		{"arn:aws:sns:us-east-1:123456789012:orders-events", "aws", "123456789012/us-east-1", "topic", "orders-events"},
		{"arn:aws:sqs:eu-west-2:123456789012:orders-cl-queue", "aws", "123456789012/eu-west-2", "subscription", "orders-cl-queue"},
		{"https://sqs.us-east-1.amazonaws.com/123456789012/orders-dlq", "aws", "123456789012/us-east-1", "subscription", "orders-dlq"},
		{"/subscriptions/0b1f6471-1bf0-4dda-aec3-111122223333/resourceGroups/rg-orders/providers/Microsoft.ServiceBus/namespaces/acme-sb/topics/orders", "azure", "0b1f6471-1bf0-4dda-aec3-111122223333", "topic", "orders"},
		{"/subscriptions/0b1f6471-1bf0-4dda-aec3-111122223333/resourceGroups/rg-orders/providers/Microsoft.ServiceBus/namespaces/acme-sb/topics/orders/subscriptions/billing", "azure", "0b1f6471-1bf0-4dda-aec3-111122223333", "subscription", "billing"},
		{"/subscriptions/0b1f6471-1bf0-4dda-aec3-111122223333/resourceGroups/rg-orders/providers/Microsoft.ServiceBus/namespaces/acme-sb/queues/invoices", "azure", "0b1f6471-1bf0-4dda-aec3-111122223333", "subscription", "invoices"},
	}
	for _, c := range cases {
		p, sc, k, sh, ok := parseResource(c.id)
		if !ok || p != c.provider || sc != c.scope || k != c.kind || sh != c.short {
			t.Errorf("%s -> %s %s %s %s %v", c.id, p, sc, k, sh, ok)
		}
		if normalizeResource(c.id) != c.short {
			t.Errorf("normalize %s = %s", c.id, normalizeResource(c.id))
		}
	}
	for _, id := range []string{"orders-in", "arn:aws:sns:us-east-1:123456789012:orders-events:5f1b0310-54ef-48a1-9d36-9f8453a360c0", "https://api.example.test/orders"} {
		if _, _, _, _, ok := parseResource(id); ok {
			t.Errorf("%s is not a canonical messaging resource", id)
		}
	}
}

func TestReferencedScopesUseOnlyConfiguredProvidersAndRealScopes(t *testing.T) {
	dir := t.TempDir()
	svc := gitRepo(t, filepath.Join(dir, "SVC-orders"), map[string]string{
		"go.mod": "module example.com/orders\n",
		"config/app.env": "PROJECT_ID=acme-orders-prd\nZONE=us-east4-a\nSAMPLE_PROJECT=your-project-id\n" +
			"OUT=projects/acme-billing-uat/topics/invoices\nQUEUE=https://sqs.us-east-1.amazonaws.com/123456789012/orders-cl-queue\n" +
			"BUS=/subscriptions/0b1f6471-1bf0-4dda-aec3-111122223333/resourceGroups/rg/providers/Microsoft.ServiceBus/namespaces/sb/topics/orders\n",
	})
	st := &store{Dependencies: map[string]judgment{}, ConfigKeys: map[string]judgment{}, ConfigValues: map[string]judgment{}}
	s := scan(t, "SVC-orders", svc)
	for _, e := range s.entries {
		choice := "other"
		if e.Key == "PROJECT_ID" || e.Key == "ZONE" || e.Key == "SAMPLE_PROJECT" {
			choice = "cloud_project_or_region"
		}
		st.ConfigKeys[keySignature(e)] = judgment{Choice: choice, Confidence: 0.95}
	}
	a := &assembly{scans: []*repoScan{s}, st: st, platform: buildPlatformIndex(nil)}
	if got := a.platformScopes([]string{"gcp"}); !slices.Equal(got, []string{"gcp:acme-billing-uat", "gcp:acme-orders-prd"}) {
		t.Fatalf("gcp scopes exclude zones and placeholders: %v", got)
	}
	want := []string{"aws:123456789012/us-east-1", "azure:0b1f6471-1bf0-4dda-aec3-111122223333", "gcp:acme-billing-uat", "gcp:acme-orders-prd"}
	if got := a.platformScopes([]string{"aws", "azure", "gcp"}); !slices.Equal(got, want) {
		t.Fatalf("scopes of several clouds: %v", got)
	}
	if got := a.platformScopes(nil); len(got) != 0 {
		t.Fatalf("no configured provider, no referenced scope: %v", got)
	}
}

// Outputs recorded from gcloud against the Pub/Sub emulator.
func TestGCPCapture(t *testing.T) {
	fakeCLI(t, map[string]string{
		"gcloud pubsub topics list": `[{"name": "projects/acme-orders-prd/topics/orders-in"}]`,
		"gcloud pubsub subscriptions list": `[{"ackDeadlineSeconds": 10, "deadLetterPolicy": {"deadLetterTopic": "projects/acme-orders-prd/topics/orders-dlq", "maxDeliveryAttempts": 5},
			"filter": "attributes.eventType=\"orderConfirmed\"", "name": "projects/acme-orders-prd/subscriptions/orders-cl-sub", "pushConfig": {}, "topic": "projects/acme-orders-prd/topics/orders-in"},
			{"name": "projects/acme-orders-prd/subscriptions/orders-bq", "topic": "projects/acme-orders-prd/topics/orders-in", "bigqueryConfig": {"table": "acme.orders.raw"}}]`,
	}, nil)
	s := providers["gcp"].capture("acme-orders-prd")
	if s.Status != "ok" || len(s.Topics) != 1 || len(s.Subscriptions) != 2 {
		t.Fatalf("%+v", s)
	}
	cl := s.Subscriptions[1]
	if cl.Name != "projects/acme-orders-prd/subscriptions/orders-cl-sub" || cl.DeadLetter != "projects/acme-orders-prd/topics/orders-dlq" || len(cl.Attributes) != 1 || cl.Attributes[0] != [2]string{"eventType", "orderConfirmed"} {
		t.Fatalf("subscription %+v", cl)
	}
	if s.Subscriptions[0].Sink != "bigquery acme.orders.raw" {
		t.Fatalf("bigquery delivery is a sink: %+v", s.Subscriptions[0])
	}
}

// Outputs recorded from the AWS CLI against moto.
var awsOutputs = map[string]string{
	"aws sts get-caller-identity": `{"UserId": "AKIAIOSFODNN7EXAMPLE", "Account": "123456789012", "Arn": "arn:aws:sts::123456789012:user/moto"}`,
	"aws sns list-topics":         `{"Topics": [{"TopicArn": "arn:aws:sns:us-east-1:123456789012:orders-events"}]}`,
	"aws sqs list-queues":         `{"QueueUrls": ["http://localhost:5000/123456789012/orders-cl-queue", "http://localhost:5000/123456789012/orders-dlq", "http://localhost:5000/123456789012/invoices"]}`,
	"aws sqs get-queue-attributes --queue-url http://localhost:5000/123456789012/orders-cl-queue": `{"Attributes": {"QueueArn": "arn:aws:sqs:us-east-1:123456789012:orders-cl-queue", "RedrivePolicy": "{\"deadLetterTargetArn\": \"arn:aws:sqs:us-east-1:123456789012:orders-dlq\", \"maxReceiveCount\": 5}"}}`,
	"aws sqs get-queue-attributes --queue-url http://localhost:5000/123456789012/orders-dlq":      `{"Attributes": {"QueueArn": "arn:aws:sqs:us-east-1:123456789012:orders-dlq"}}`,
	"aws sqs get-queue-attributes --queue-url http://localhost:5000/123456789012/invoices":        `{"Attributes": {"QueueArn": "arn:aws:sqs:us-east-1:123456789012:invoices"}}`,
	"aws sns list-subscriptions": `{"Subscriptions": [
		{"SubscriptionArn": "arn:aws:sns:us-east-1:123456789012:orders-events:f417e61d-ca44-4550-b89b-e8826028abd4", "Owner": "123456789012", "Protocol": "sqs", "Endpoint": "arn:aws:sqs:us-east-1:123456789012:orders-cl-queue", "TopicArn": "arn:aws:sns:us-east-1:123456789012:orders-events"},
		{"SubscriptionArn": "arn:aws:sns:us-east-1:123456789012:orders-events:f71b0310-54ef-48a1-9d36-9f8453a360c0", "Owner": "123456789012", "Protocol": "https", "Endpoint": "https://hooks.example.test/orders", "TopicArn": "arn:aws:sns:us-east-1:123456789012:orders-events"},
		{"SubscriptionArn": "PendingConfirmation", "Protocol": "email", "Endpoint": "ops@example.test", "TopicArn": "arn:aws:sns:us-east-1:123456789012:orders-events"}]}`,
	"aws sns get-subscription-attributes --subscription-arn arn:aws:sns:us-east-1:123456789012:orders-events:f417e61d": `{"Attributes": {"Protocol": "sqs", "FilterPolicy": "{\"eventType\":[\"orderConfirmed\",\"orderCancelled\"]}", "FilterPolicyScope": "MessageAttributes"}}`,
	"aws sns get-subscription-attributes --subscription-arn arn:aws:sns:us-east-1:123456789012:orders-events:f71b0310": `{"Attributes": {"Protocol": "https"}}`,
}

func TestAWSCapture(t *testing.T) {
	fakeCLI(t, awsOutputs, nil)
	s := providers["aws"].capture("123456789012/us-east-1")
	if s.Status != "ok" || len(s.Topics) != 1 {
		t.Fatalf("%+v", s)
	}
	byName := map[string]platformSubscription{}
	for _, sub := range s.Subscriptions {
		byName[shortName(sub.Name)] = sub
	}
	q := byName["orders-cl-queue"]
	if q.Topic != "arn:aws:sns:us-east-1:123456789012:orders-events" || q.DeadLetter != "arn:aws:sqs:us-east-1:123456789012:orders-dlq" ||
		!slices.Equal(q.Attributes, [][2]string{{"eventType", "orderConfirmed"}, {"eventType", "orderCancelled"}}) {
		t.Fatalf("queue subscribed to the topic with its filter policy: %+v", q)
	}
	if byName["invoices"].Name != "arn:aws:sqs:us-east-1:123456789012:invoices" || byName["invoices"].Topic != "" {
		t.Fatalf("a standalone queue is a consumer without topic: %+v", s.Subscriptions)
	}
	push := 0
	for _, sub := range s.Subscriptions {
		if sub.Push == "https://hooks.example.test/orders" {
			push++
		}
	}
	if push != 1 || len(s.Subscriptions) != 4 {
		t.Fatalf("https subscription pushes; pending confirmations are skipped: %+v", s.Subscriptions)
	}
}

func TestAWSCaptureRefusesCredentialsOfAnotherAccount(t *testing.T) {
	fakeCLI(t, awsOutputs, nil)
	s := providers["aws"].capture("210987654321/us-east-1")
	if s.Status != "denied" || !strings.Contains(s.Detail, "belong to account 123456789012") || len(s.Topics) != 0 {
		t.Fatalf("%+v", s)
	}
}

// Shapes of `az servicebus ... --output json` as documented by the Azure CLI.
func TestAzureCapture(t *testing.T) {
	const sub = "0b1f6471-1bf0-4dda-aec3-111122223333"
	ns := "/subscriptions/" + sub + "/resourceGroups/rg-orders/providers/Microsoft.ServiceBus/namespaces/acme-sb"
	fakeCLI(t, map[string]string{
		"az servicebus namespace list": `[{"id": "` + ns + `", "name": "acme-sb", "resourceGroup": "rg-orders", "location": "eastus"}]`,
		"az servicebus topic list":     `[{"id": "` + ns + `/topics/orders", "name": "orders"}]`,
		"az servicebus topic subscription list": `[{"id": "` + ns + `/topics/orders/subscriptions/billing", "name": "billing", "forwardDeadLetteredMessagesTo": "billing-dlq"},
			{"id": "` + ns + `/topics/orders/subscriptions/audit", "name": "audit", "forwardTo": "audit-queue"}]`,
		"az servicebus topic subscription rule list --topic-name orders --subscription-name billing": `[{"name": "only-confirmed", "filterType": "CorrelationFilter", "correlationFilter": {"label": "orders", "properties": {"eventType": "orderConfirmed"}}}]`,
		"az servicebus topic subscription rule list --topic-name orders --subscription-name audit":   `[{"name": "$Default", "filterType": "SqlFilter", "sqlFilter": {"sqlExpression": "1=1"}}, {"name": "cl", "filterType": "SqlFilter", "sqlFilter": {"sqlExpression": "user.country = 'CL' AND eventType = 'orderCancelled'"}}]`,
		"az servicebus queue list": `[{"id": "` + ns + `/queues/invoices", "name": "invoices"}]`,
	}, nil)
	s := providers["azure"].capture(sub)
	if s.Status != "ok" || len(s.Topics) != 1 || len(s.Subscriptions) != 3 {
		t.Fatalf("%+v", s)
	}
	byName := map[string]platformSubscription{}
	for _, x := range s.Subscriptions {
		byName[shortName(x.Name)] = x
	}
	if b := byName["billing"]; b.Topic != ns+"/topics/orders" || b.DeadLetter != "billing-dlq" || !slices.Contains(b.Attributes, [2]string{"eventType", "orderConfirmed"}) {
		t.Fatalf("correlation filter: %+v", b)
	}
	if a := byName["audit"]; a.Sink != "forward audit-queue" || !slices.Equal(a.Attributes, [][2]string{{"country", "CL"}, {"eventType", "orderCancelled"}}) {
		t.Fatalf("sql filter without the default rule: %+v", a)
	}
	if byName["invoices"].Topic != "" {
		t.Fatalf("queue: %+v", byName["invoices"])
	}
}

func TestCaptureFailuresAreClassified(t *testing.T) {
	cases := map[string]string{
		"ERROR: (gcloud.pubsub.topics.list) There was a problem refreshing your current auth tokens: Reauthentication failed.": "auth-required",
		"ERROR: (gcloud.pubsub.topics.list) PERMISSION_DENIED: User not authorized to perform this action.":                    "denied",
		"ERROR: (gcloud.pubsub.topics.list) Projects instance [x] not found":                                                   "not-found",
		"gcloud is not installed or not on PATH":                                                                               "unavailable",
		"ERROR: Please run 'az login' to setup account.":                                                                       "auth-required",
		"(AuthorizationFailed) The client does not have authorization to perform action":                                       "denied",
		"(SubscriptionNotFound) The subscription could not be found.":                                                          "not-found",
		"An error occurred (ExpiredToken) when calling the ListTopics operation":                                               "auth-required",
		"An error occurred (AuthorizationError) when calling the ListTopics operation: User is not authorized":                 "denied",
	}
	for msg, want := range cases {
		fakeCLI(t, map[string]string{}, map[string]string{"gcloud": msg})
		if got := providers["gcp"].capture("acme-orders-prd"); got.Status != want || got.Confirm == "" {
			t.Errorf("%q -> %s, want %s", msg, got.Status, want)
		}
	}
}

func TestAssemblyWiresAWSConsumers(t *testing.T) {
	dir := t.TempDir()
	svc := gitRepo(t, filepath.Join(dir, "SVC-orders"), map[string]string{
		"go.mod":         "module example.com/orders\n",
		"main.go":        "package main\nconst ev = \"orderConfirmed\"\n",
		"config/app.env": "QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123456789012/orders-cl-queue\nGONE=arn:aws:sns:us-east-1:123456789012:ghost-events\n",
	})
	st := &store{Dependencies: map[string]judgment{}, ConfigKeys: map[string]judgment{}, ConfigValues: map[string]judgment{}}
	s := scan(t, "SVC-orders", svc)
	for _, e := range s.entries {
		st.ConfigKeys[keySignature(e)] = judgment{Choice: "other", Confidence: 0.9}
	}
	fakeCLI(t, awsOutputs, nil)
	snap := providers["aws"].capture("123456789012/us-east-1")
	a := &assembly{scans: []*repoScan{s}, st: st, platform: buildPlatformIndex([]platformSnapshot{snap})}
	f := a.facts()[0]
	got := map[string]resource{}
	for _, r := range f.Resources {
		got[normalizeResource(r.Name)] = r
	}
	q := got["orders-cl-queue"]
	if q.Type != "message_subscription" || q.Direction != "consume" || q.Topic != "arn:aws:sns:us-east-1:123456789012:orders-events" || !slices.Contains(q.Events, "orderConfirmed") {
		t.Fatalf("queue wiring: %+v", q)
	}
	pending := map[string]string{}
	for _, p := range f.Pending {
		pending[normalizeResource(p.Subject)] = p.Kind
	}
	if pending["ghost-events"] != "not-in-platform" {
		t.Fatalf("a topic ARN absent from the captured account is pending: %v", f.Pending)
	}
	if !slices.Contains(f.Channels, "messaging") {
		t.Fatalf("channels %v", f.Channels)
	}
}

func TestConfigKnowsEveryProvider(t *testing.T) {
	if !slices.Equal(config.KnownPlatformProviders, ProviderNames()) {
		t.Fatalf("config accepts %v, discover captures %v", config.KnownPlatformProviders, ProviderNames())
	}
}
