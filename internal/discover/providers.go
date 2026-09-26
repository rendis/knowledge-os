package discover

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Each provider reads its cloud's platform with the cloud's own CLI, read-only: names and relations of
// messaging, databases, storage and (Google Cloud) the warehouse, never their data.

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

func (gcpProvider) kinds() []string {
	return []string{"messaging", "document_db", "sql_db", "object_storage", "warehouse"}
}

func (gcpProvider) confirm(project string) string {
	return fmt.Sprintf("gcloud pubsub topics list --project %[1]s; gcloud firestore databases list --project %[1]s; gcloud sql instances list --project %[1]s; gcloud storage buckets list --project %[1]s; bq ls --project_id=%[1]s", project)
}

func (gcpProvider) commands() string {
	return "gcloud pubsub topics/subscriptions list; firestore databases/indexes list; sql instances/databases list; storage buckets list; bq ls (read-only, names only)"
}

func (p gcpProvider) capture(project string) platformSnapshot {
	return captureKinds(newSnapshot("gcp", project), map[string]kindCapture{
		"messaging":      func(s *platformSnapshot) (string, error) { return gcpMessaging(s, project) },
		"document_db":    func(s *platformSnapshot) (string, error) { return gcpFirestore(s, project) },
		"sql_db":         func(s *platformSnapshot) (string, error) { return gcpCloudSQL(s, project) },
		"object_storage": func(s *platformSnapshot) (string, error) { return gcpStorage(s, project) },
		"warehouse":      func(s *platformSnapshot) (string, error) { return gcpBigQuery(s, project) },
	}, p.kinds())
}

func gcpMessaging(snap *platformSnapshot, project string) (string, error) {
	var topics []struct {
		Name string `json:"name"`
	}
	if msg, e := jsonCLI(&topics, "gcloud", "pubsub", "topics", "list", "--project", project, "--format=json"); e != nil {
		return msg, e
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
			DeadLetterTopic     string `json:"deadLetterTopic"`
			MaxDeliveryAttempts int    `json:"maxDeliveryAttempts"`
		} `json:"deadLetterPolicy"`
		AckDeadlineSeconds int `json:"ackDeadlineSeconds"`
		RetryPolicy        *struct {
			MinimumBackoff string `json:"minimumBackoff"`
			MaximumBackoff string `json:"maximumBackoff"`
		} `json:"retryPolicy"`
		MessageRetentionDuration string `json:"messageRetentionDuration"`
	}
	if msg, e := jsonCLI(&subs, "gcloud", "pubsub", "subscriptions", "list", "--project", project, "--format=json"); e != nil {
		return msg, e
	}
	for _, s := range subs {
		sink := ""
		switch {
		case s.BigqueryConfig.Table != "":
			sink = "bigquery " + s.BigqueryConfig.Table
		case s.CloudStorageConfig.Bucket != "":
			sink = "storage " + s.CloudStorageConfig.Bucket
		}
		// How a message comes back: the ack deadline, the backoff between redeliveries (none set
		// means immediate), the attempts before the dead letter and how long an unacked one is kept.
		delivery := []string{}
		if s.AckDeadlineSeconds > 0 {
			delivery = append(delivery, fmt.Sprintf("ack deadline %ds", s.AckDeadlineSeconds))
		}
		if s.RetryPolicy != nil {
			delivery = append(delivery, "redelivery backoff "+s.RetryPolicy.MinimumBackoff+"–"+s.RetryPolicy.MaximumBackoff)
		} else {
			delivery = append(delivery, "redelivery immediate (no retry policy)")
		}
		if s.DeadLetterPolicy.MaxDeliveryAttempts > 0 {
			delivery = append(delivery, fmt.Sprintf("dead letter after %d attempts", s.DeadLetterPolicy.MaxDeliveryAttempts))
		}
		if s.MessageRetentionDuration != "" {
			delivery = append(delivery, "retention "+s.MessageRetentionDuration)
		}
		snap.Subscriptions = append(snap.Subscriptions, platformSubscription{Name: s.Name, Topic: s.Topic, Filter: s.Filter,
			Attributes: filterAttributes(s.Filter), Push: s.PushConfig.PushEndpoint, Sink: sink, DeadLetter: s.DeadLetterPolicy.DeadLetterTopic,
			Delivery: strings.Join(delivery, ", ")})
	}
	return "", nil
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

func (awsProvider) kinds() []string {
	return []string{"messaging", "document_db", "sql_db", "object_storage"}
}

func (awsProvider) confirm(scope string) string {
	account, region, _ := strings.Cut(scope, "/")
	return fmt.Sprintf("aws sns list-topics / sqs list-queues / dynamodb list-tables / rds describe-db-instances --region %s; aws s3api list-buckets (credentials of account %s)", region, account)
}

func (awsProvider) commands() string {
	return "aws sts get-caller-identity; sns list-topics/list-subscriptions/get-subscription-attributes; sqs list-queues/get-queue-attributes; dynamodb list-tables; rds describe-db-instances/clusters; s3api list-buckets (read-only, names only)"
}

func (p awsProvider) capture(scope string) platformSnapshot {
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
	return captureKinds(snap, map[string]kindCapture{
		"messaging":      func(s *platformSnapshot) (string, error) { return awsMessaging(s, r) },
		"document_db":    func(s *platformSnapshot) (string, error) { return awsDynamoDB(s, r) },
		"sql_db":         func(s *platformSnapshot) (string, error) { return awsRDS(s, r) },
		"object_storage": func(s *platformSnapshot) (string, error) { return awsS3(s) },
	}, p.kinds())
}

func awsMessaging(snap *platformSnapshot, r []string) (string, error) {
	var topics struct{ Topics []struct{ TopicArn string } }
	if msg, e := jsonCLI(&topics, "aws", append([]string{"sns", "list-topics"}, r...)...); e != nil {
		return msg, e
	}
	for _, t := range topics.Topics {
		snap.Topics = append(snap.Topics, t.TopicArn)
	}
	var queues struct{ QueueUrls []string }
	if msg, e := jsonCLI(&queues, "aws", append([]string{"sqs", "list-queues"}, r...)...); e != nil {
		return msg, e
	}
	queueDLQ, subscribed := map[string]string{}, map[string]bool{}
	queueARNs := []string{}
	for _, u := range queues.QueueUrls {
		var attrs struct{ Attributes map[string]string }
		if msg, e := jsonCLI(&attrs, "aws", append([]string{"sqs", "get-queue-attributes", "--queue-url", u, "--attribute-names", "QueueArn", "RedrivePolicy"}, r...)...); e != nil {
			return msg, e
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
		return msg, e
	}
	for _, s := range subs.Subscriptions {
		if !strings.HasPrefix(s.SubscriptionArn, "arn:") {
			continue // PendingConfirmation: not wired yet
		}
		var attrs struct{ Attributes map[string]string }
		if msg, e := jsonCLI(&attrs, "aws", append([]string{"sns", "get-subscription-attributes", "--subscription-arn", s.SubscriptionArn}, r...)...); e != nil {
			return msg, e
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
	return "", nil
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

func (azureProvider) kinds() []string {
	return []string{"messaging", "document_db", "sql_db", "object_storage"}
}

func (azureProvider) confirm(sub string) string {
	return fmt.Sprintf("az servicebus namespace list / cosmosdb list / sql server list / postgres flexible-server list / storage account list --subscription %s", sub)
}

func (azureProvider) commands() string {
	return "az servicebus namespace/topic/subscription/rule/queue list; cosmosdb list and sql database/container list; sql server/db list; postgres flexible-server/db list; storage account list (read-only, names only)"
}

func (p azureProvider) capture(sub string) platformSnapshot {
	return captureKinds(newSnapshot("azure", sub), map[string]kindCapture{
		"messaging":      func(s *platformSnapshot) (string, error) { return azureMessaging(s, sub) },
		"document_db":    func(s *platformSnapshot) (string, error) { return azureCosmos(s, sub) },
		"sql_db":         func(s *platformSnapshot) (string, error) { return azureSQL(s, sub) },
		"object_storage": func(s *platformSnapshot) (string, error) { return azureStorage(s, sub) },
	}, p.kinds())
}

func azureMessaging(snap *platformSnapshot, sub string) (string, error) {
	var namespaces []struct{ Name, ResourceGroup string }
	if msg, e := jsonCLI(&namespaces, "az", "servicebus", "namespace", "list", "--subscription", sub, "--output", "json"); e != nil {
		return msg, e
	}
	for _, ns := range namespaces {
		at := []string{"--subscription", sub, "--resource-group", ns.ResourceGroup, "--namespace-name", ns.Name, "--output", "json"}
		var topics []struct{ ID, Name string }
		if msg, e := jsonCLI(&topics, "az", append([]string{"servicebus", "topic", "list"}, at...)...); e != nil {
			return msg, e
		}
		for _, t := range topics {
			snap.Topics = append(snap.Topics, t.ID)
			var subs []struct {
				ID, Name, ForwardTo, ForwardDeadLetteredMessagesTo string
			}
			if msg, e := jsonCLI(&subs, "az", append([]string{"servicebus", "topic", "subscription", "list", "--topic-name", t.Name}, at...)...); e != nil {
				return msg, e
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
					return msg, e
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
			return msg, e
		}
		for _, q := range queues {
			ps := platformSubscription{Name: q.ID, DeadLetter: q.ForwardDeadLetteredMessagesTo}
			if q.ForwardTo != "" {
				ps.Sink = "forward " + q.ForwardTo
			}
			snap.Subscriptions = append(snap.Subscriptions, ps)
		}
	}
	return "", nil
}
