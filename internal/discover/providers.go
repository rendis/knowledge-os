package discover

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Each provider reads its cloud's messaging platform with the cloud's own CLI, read-only.

// --- Google Cloud: Pub/Sub topics and subscriptions of one project (scope = project id).

type gcpProvider struct{}

var (
	gcpResource   = regexp.MustCompile(`(?i)projects/([a-z][a-z0-9\-]+)/(topics|subscriptions)/([\w.\-{}$~%+]+)`)
	gcpScopePath  = regexp.MustCompile(`projects/([a-z][a-z0-9\-]{4,28}[a-z0-9])/`)
	gcpProjectID  = regexp.MustCompile(`^[a-z][a-z0-9\-]{4,28}[a-z0-9]$`)
	gcpLocation   = regexp.MustCompile(`^[a-z]+-[a-z]+\d+(-[a-z])?$`) // us-east4, us-east4-a
	placeholderID = regexp.MustCompile(`(^|-)(your|my|example|sample|dummy|test)-|project-?id|changeme`)
)

func (gcpProvider) parse(id string) (string, string, string, bool) {
	m := gcpResource.FindStringSubmatch(id)
	if m == nil {
		return "", "", "", false
	}
	kind := "subscription"
	if strings.EqualFold(m[2], "topics") {
		kind = "topic"
	}
	return strings.ToLower(m[1]), kind, m[3], true
}

func (gcpProvider) scopesIn(v string) []string {
	out := []string{}
	for _, m := range gcpScopePath.FindAllStringSubmatch(v, -1) {
		out = append(out, m[1])
	}
	return out
}

func (p gcpProvider) isScope(v string) bool {
	return p.validScope(v) && strings.Count(v, "-") >= 2 && !gcpLocation.MatchString(v) && !placeholderID.MatchString(v)
}

func (gcpProvider) validScope(s string) bool { return gcpProjectID.MatchString(s) }

func (gcpProvider) kinds() []string { return []string{"messaging"} }

func (gcpProvider) confirm(project string) string {
	return fmt.Sprintf("gcloud pubsub topics list --project %s && gcloud pubsub subscriptions list --project %s", project, project)
}

func (gcpProvider) commands() string {
	return "gcloud pubsub topics list / subscriptions list --format=json (read-only)"
}

func (gcpProvider) capture(project string) platformSnapshot {
	snap := newSnapshot("gcp", project)
	var topics []struct {
		Name string `json:"name"`
	}
	if msg, e := jsonCLI(&topics, "gcloud", "pubsub", "topics", "list", "--project", project, "--format=json"); e != nil {
		return snap.failed(msg)
	}
	for _, t := range topics {
		snap.Topics = append(snap.Topics, t.Name)
	}
	var subs []struct {
		Name       string `json:"name"`
		Topic      string `json:"topic"`
		Filter     string `json:"filter"`
		PushConfig struct {
			PushEndpoint string `json:"pushEndpoint"`
		} `json:"pushConfig"`
		BigqueryConfig struct {
			Table string `json:"table"`
		} `json:"bigqueryConfig"`
		CloudStorageConfig struct {
			Bucket string `json:"bucket"`
		} `json:"cloudStorageConfig"`
		DeadLetterPolicy struct {
			DeadLetterTopic string `json:"deadLetterTopic"`
		} `json:"deadLetterPolicy"`
	}
	if msg, e := jsonCLI(&subs, "gcloud", "pubsub", "subscriptions", "list", "--project", project, "--format=json"); e != nil {
		return snap.failed(msg)
	}
	for _, s := range subs {
		sink := ""
		switch {
		case s.BigqueryConfig.Table != "":
			sink = "bigquery " + s.BigqueryConfig.Table
		case s.CloudStorageConfig.Bucket != "":
			sink = "storage " + s.CloudStorageConfig.Bucket
		}
		snap.Subscriptions = append(snap.Subscriptions, platformSubscription{Name: s.Name, Topic: s.Topic, Filter: s.Filter,
			Attributes: filterAttributes(s.Filter), Push: s.PushConfig.PushEndpoint, Sink: sink, DeadLetter: s.DeadLetterPolicy.DeadLetterTopic})
	}
	return snap.sorted()
}

// --- AWS: SNS topics, SQS queues and SNS subscriptions of one account and region
// (scope = "<account>/<region>"). Credentials come from the environment (AWS_PROFILE and friends).

type awsProvider struct{}

var (
	awsARN   = regexp.MustCompile(`arn:aws[a-z\-]*:(sns|sqs):([a-z]{2}(?:-[a-z]+)+-\d):(\d{12}):([A-Za-z0-9_.\-]+)(:[0-9a-f\-]{36})?`)
	awsQueue = regexp.MustCompile(`https://sqs[.\-]([a-z]{2}(?:-[a-z]+)+-\d)\.amazonaws\.com(?:\.cn)?/(\d{12})/([A-Za-z0-9_.\-]+)`)
	awsScope = regexp.MustCompile(`^(\d{12})/([a-z]{2}(?:-[a-z]+)+-\d)$`)
)

func (awsProvider) parse(id string) (string, string, string, bool) {
	if m := awsARN.FindStringSubmatch(id); m != nil {
		if m[5] != "" {
			return "", "", "", false // an SNS subscription ARN names the link, not a resource a config uses
		}
		kind := "topic"
		if m[1] == "sqs" {
			kind = "subscription" // a queue is the consumer end
		}
		return m[3] + "/" + m[2], kind, m[4], true
	}
	if m := awsQueue.FindStringSubmatch(id); m != nil {
		return m[2] + "/" + m[1], "subscription", m[3], true
	}
	return "", "", "", false
}

func (p awsProvider) scopesIn(v string) []string {
	out := []string{}
	for _, m := range awsARN.FindAllStringSubmatch(v, -1) {
		out = append(out, m[3]+"/"+m[2])
	}
	for _, m := range awsQueue.FindAllStringSubmatch(v, -1) {
		out = append(out, m[2]+"/"+m[1])
	}
	return out
}

func (awsProvider) isScope(string) bool { return false } // an account id alone does not say the region

func (awsProvider) validScope(s string) bool { return awsScope.MatchString(s) }

func (awsProvider) kinds() []string { return []string{"messaging"} }

func (awsProvider) confirm(scope string) string {
	account, region, _ := strings.Cut(scope, "/")
	return fmt.Sprintf("aws sns list-topics --region %s && aws sqs list-queues --region %s && aws sns list-subscriptions --region %s (credentials of account %s)", region, region, region, account)
}

func (awsProvider) commands() string {
	return "aws sts get-caller-identity; aws sns list-topics / list-subscriptions / get-subscription-attributes; aws sqs list-queues / get-queue-attributes (read-only)"
}

func (awsProvider) capture(scope string) platformSnapshot {
	snap := newSnapshot("aws", scope)
	account, region, _ := strings.Cut(scope, "/")
	var id struct{ Account string }
	if msg, e := jsonCLI(&id, "aws", "sts", "get-caller-identity", "--output", "json"); e != nil {
		return snap.failed(msg)
	}
	if id.Account != account {
		return snap.failed(fmt.Sprintf("the active AWS credentials belong to account %s, not %s: select a profile of %s (AWS_PROFILE)", id.Account, account, account))
	}
	r := []string{"--region", region, "--output", "json"}
	var topics struct{ Topics []struct{ TopicArn string } }
	if msg, e := jsonCLI(&topics, "aws", append([]string{"sns", "list-topics"}, r...)...); e != nil {
		return snap.failed(msg)
	}
	for _, t := range topics.Topics {
		snap.Topics = append(snap.Topics, t.TopicArn)
	}
	var queues struct{ QueueUrls []string }
	if msg, e := jsonCLI(&queues, "aws", append([]string{"sqs", "list-queues"}, r...)...); e != nil {
		return snap.failed(msg)
	}
	queueDLQ, subscribed := map[string]string{}, map[string]bool{}
	queueARNs := []string{}
	for _, u := range queues.QueueUrls {
		var attrs struct{ Attributes map[string]string }
		if msg, e := jsonCLI(&attrs, "aws", append([]string{"sqs", "get-queue-attributes", "--queue-url", u, "--attribute-names", "QueueArn", "RedrivePolicy"}, r...)...); e != nil {
			return snap.failed(msg)
		}
		arn := attrs.Attributes["QueueArn"]
		var redrive struct {
			DeadLetterTargetArn string `json:"deadLetterTargetArn"`
		}
		_ = json.Unmarshal([]byte(attrs.Attributes["RedrivePolicy"]), &redrive)
		queueDLQ[arn] = redrive.DeadLetterTargetArn
		queueARNs = append(queueARNs, arn)
	}
	var subs struct {
		Subscriptions []struct{ SubscriptionArn, Protocol, Endpoint, TopicArn string }
	}
	if msg, e := jsonCLI(&subs, "aws", append([]string{"sns", "list-subscriptions"}, r...)...); e != nil {
		return snap.failed(msg)
	}
	for _, s := range subs.Subscriptions {
		if !strings.HasPrefix(s.SubscriptionArn, "arn:") {
			continue // PendingConfirmation: not wired yet
		}
		var attrs struct{ Attributes map[string]string }
		if msg, e := jsonCLI(&attrs, "aws", append([]string{"sns", "get-subscription-attributes", "--subscription-arn", s.SubscriptionArn}, r...)...); e != nil {
			return snap.failed(msg)
		}
		policy := attrs.Attributes["FilterPolicy"]
		ps := platformSubscription{Topic: s.TopicArn, Filter: policy, Attributes: policyAttributes(policy)}
		switch s.Protocol {
		case "sqs":
			ps.Name, ps.DeadLetter = s.Endpoint, queueDLQ[s.Endpoint]
			subscribed[s.Endpoint] = true
		case "http", "https":
			ps.Name, ps.Push = s.SubscriptionArn, s.Endpoint
		default:
			ps.Name, ps.Sink = s.SubscriptionArn, s.Protocol+" "+s.Endpoint
		}
		snap.Subscriptions = append(snap.Subscriptions, ps)
	}
	for _, q := range queueARNs {
		if !subscribed[q] {
			snap.Subscriptions = append(snap.Subscriptions, platformSubscription{Name: q, DeadLetter: queueDLQ[q]})
		}
	}
	return snap.sorted()
}

// --- Azure: Service Bus topics, subscriptions (with their rules) and queues of every namespace in one
// Azure subscription (scope = subscription id).

type azureProvider struct{}

var (
	azureResource = regexp.MustCompile(`(?i)/subscriptions/([0-9a-f\-]{36})/resourcegroups/[^/]+/providers/microsoft\.servicebus/namespaces/[^/]+/(topics|queues)/([^/\s"']+)(?:/subscriptions/([^/\s"']+))?`)
	azureScope    = regexp.MustCompile(`(?i)/subscriptions/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})/`)
	azureGUID     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

func (azureProvider) parse(id string) (string, string, string, bool) {
	m := azureResource.FindStringSubmatch(id)
	if m == nil {
		return "", "", "", false
	}
	switch {
	case m[4] != "":
		return strings.ToLower(m[1]), "subscription", m[4], true
	case strings.EqualFold(m[2], "queues"):
		return strings.ToLower(m[1]), "subscription", m[3], true
	}
	return strings.ToLower(m[1]), "topic", m[3], true
}

func (azureProvider) scopesIn(v string) []string {
	out := []string{}
	for _, m := range azureScope.FindAllStringSubmatch(v, -1) {
		out = append(out, strings.ToLower(m[1]))
	}
	return out
}

func (azureProvider) isScope(string) bool { return false } // a bare GUID may be a tenant or a client id

func (azureProvider) validScope(s string) bool { return azureGUID.MatchString(s) }

func (azureProvider) kinds() []string { return []string{"messaging"} }

func (azureProvider) confirm(sub string) string {
	return fmt.Sprintf("az servicebus namespace list --subscription %s, then az servicebus topic list / topic subscription list / queue list per namespace", sub)
}

func (azureProvider) commands() string {
	return "az servicebus namespace list; topic list; topic subscription list; topic subscription rule list; queue list (read-only)"
}

func (azureProvider) capture(sub string) platformSnapshot {
	snap := newSnapshot("azure", sub)
	var namespaces []struct{ Name, ResourceGroup string }
	if msg, e := jsonCLI(&namespaces, "az", "servicebus", "namespace", "list", "--subscription", sub, "--output", "json"); e != nil {
		return snap.failed(msg)
	}
	for _, ns := range namespaces {
		at := []string{"--subscription", sub, "--resource-group", ns.ResourceGroup, "--namespace-name", ns.Name, "--output", "json"}
		var topics []struct{ ID, Name string }
		if msg, e := jsonCLI(&topics, "az", append([]string{"servicebus", "topic", "list"}, at...)...); e != nil {
			return snap.failed(msg)
		}
		for _, t := range topics {
			snap.Topics = append(snap.Topics, t.ID)
			var subs []struct {
				ID, Name, ForwardTo, ForwardDeadLetteredMessagesTo string
			}
			if msg, e := jsonCLI(&subs, "az", append([]string{"servicebus", "topic", "subscription", "list", "--topic-name", t.Name}, at...)...); e != nil {
				return snap.failed(msg)
			}
			for _, s := range subs {
				var rules []struct {
					Name       string
					SQLFilter  struct{ SQLExpression string } `json:"sqlFilter"`
					Correlated struct {
						Label      string            `json:"label"`
						Properties map[string]string `json:"properties"`
					} `json:"correlationFilter"`
				}
				if msg, e := jsonCLI(&rules, "az", append([]string{"servicebus", "topic", "subscription", "rule", "list", "--topic-name", t.Name, "--subscription-name", s.Name}, at...)...); e != nil {
					return snap.failed(msg)
				}
				filters, attrs := []string{}, [][2]string{}
				for _, rl := range rules {
					if x := strings.TrimSpace(rl.SQLFilter.SQLExpression); x != "" && x != "1=1" {
						filters = append(filters, x)
						for _, m := range sqlAttr.FindAllStringSubmatch(x, -1) {
							attrs = append(attrs, [2]string{m[1], m[2]})
						}
					}
					if rl.Correlated.Label != "" {
						filters = append(filters, "label = '"+rl.Correlated.Label+"'")
						attrs = append(attrs, [2]string{"label", rl.Correlated.Label})
					}
					for _, k := range sortedKeys(rl.Correlated.Properties) {
						filters = append(filters, k+" = '"+rl.Correlated.Properties[k]+"'")
						attrs = append(attrs, [2]string{k, rl.Correlated.Properties[k]})
					}
				}
				ps := platformSubscription{Name: s.ID, Topic: t.ID, Filter: strings.Join(filters, " OR "), Attributes: attrs, DeadLetter: s.ForwardDeadLetteredMessagesTo}
				if s.ForwardTo != "" {
					ps.Sink = "forward " + s.ForwardTo
				}
				snap.Subscriptions = append(snap.Subscriptions, ps)
			}
		}
		var queues []struct{ ID, ForwardTo, ForwardDeadLetteredMessagesTo string }
		if msg, e := jsonCLI(&queues, "az", append([]string{"servicebus", "queue", "list"}, at...)...); e != nil {
			return snap.failed(msg)
		}
		for _, q := range queues {
			ps := platformSubscription{Name: q.ID, DeadLetter: q.ForwardDeadLetteredMessagesTo}
			if q.ForwardTo != "" {
				ps.Sink = "forward " + q.ForwardTo
			}
			snap.Subscriptions = append(snap.Subscriptions, ps)
		}
	}
	return snap.sorted()
}
