package discover

import (
	"errors"
	"strings"
)

// Validate the observed identifier before adding its namespace prefix or using it in a child read.
func requirePlatformName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("unreadable platform inventory: a resource identifier is missing")
	}
	return nil
}

// Data services of each provider: document and relational databases, object storage and the
// warehouse. Names and relations only; no command reads data.

func (s *platformSnapshot) add(kind, typ, name string, aliases ...string) {
	clean := []string{}
	for _, a := range aliases {
		if a = strings.TrimSpace(a); a != "" && a != name {
			clean = appendUnique(clean, a)
		}
	}
	s.Resources = append(s.Resources, platformResource{Kind: kind, Type: typ, Name: name, Aliases: clean})
}

// --- Google Cloud

// gcpFirestore lists Firestore databases and the collection groups their indexes name (collections
// without an index or field override do not appear: an absence proves nothing).
func gcpFirestore(snap *platformSnapshot, project string) (string, error) {
	var dbs []struct{ Name, Type string }
	if msg, e := jsonCLI(&dbs, "gcloud", "firestore", "databases", "list", "--project", project, "--format=json"); e != nil {
		return msg, e
	}
	for _, db := range dbs {
		if e := requirePlatformName(db.Name); e != nil {
			return e.Error(), e
		}
		id := db.Name[strings.LastIndex(db.Name, "/")+1:]
		snap.add("document_db", "database", db.Name, id)
		groups := map[string]bool{}
		for _, list := range [][]string{{"indexes", "composite", "list"}, {"indexes", "fields", "list"}} {
			var idx []struct{ Name string }
			args := append(append([]string{"firestore"}, list...), "--project", project, "--database", id, "--format=json")
			if _, e := jsonCLI(&idx, "gcloud", args...); e != nil {
				continue // an index listing the account cannot read leaves the database itself captured
			}
			for _, x := range idx {
				if i := strings.Index(x.Name, "/collectionGroups/"); i >= 0 {
					g := strings.SplitN(x.Name[i+len("/collectionGroups/"):], "/", 2)[0]
					if g != "__default__" {
						groups[g] = true
					}
				}
			}
		}
		for _, g := range sortedKeys(groups) {
			snap.add("document_db", "collection", db.Name+"/collectionGroups/"+g, g)
		}
	}
	return "", nil
}

// gcpCloudSQL lists Cloud SQL instances (by connection name) and their databases.
func gcpCloudSQL(snap *platformSnapshot, project string) (string, error) {
	var instances []struct {
		Name, ConnectionName, DatabaseVersion, State string
		IPAddresses                                  []struct{ IPAddress string } `json:"ipAddresses"`
	}
	if msg, e := jsonCLI(&instances, "gcloud", "sql", "instances", "list", "--project", project, "--format=json"); e != nil {
		return msg, e
	}
	for _, in := range instances {
		if e := requirePlatformName(in.Name); e != nil {
			return e.Error(), e
		}
		snap.add("sql_db", "instance", in.ConnectionName, in.Name)
		snap.Resources[len(snap.Resources)-1].State = in.State
		var dbs []struct{ Name string }
		if _, e := jsonCLI(&dbs, "gcloud", "sql", "databases", "list", "--instance", in.Name, "--project", project, "--format=json"); e != nil {
			continue // a stopped instance cannot list its databases
		}
		for _, d := range dbs {
			snap.add("sql_db", "database", in.ConnectionName+"/"+d.Name, d.Name)
		}
	}
	return "", nil
}

func gcpStorage(snap *platformSnapshot, project string) (string, error) {
	var buckets []struct {
		Name       string `json:"name"`
		StorageURL string `json:"storage_url"`
	}
	if msg, e := jsonCLI(&buckets, "gcloud", "storage", "buckets", "list", "--project", project, "--format=json"); e != nil {
		return msg, e
	}
	for _, b := range buckets {
		if e := requirePlatformName(b.Name); e != nil {
			return e.Error(), e
		}
		snap.add("object_storage", "bucket", "gs://"+b.Name, b.Name)
	}
	return "", nil
}

// gcpBigQuery lists datasets and their tables with the bq tool of the Cloud SDK.
func gcpBigQuery(snap *platformSnapshot, project string) (string, error) {
	var datasets []struct {
		DatasetReference struct{ DatasetID string } `json:"datasetReference"`
	}
	if msg, e := jsonCLI(&datasets, "bq", "ls", "--project_id="+project, "--format=json", "--max_results=1000"); e != nil {
		return msg, e
	}
	for i, d := range datasets {
		ds := d.DatasetReference.DatasetID
		if e := requirePlatformName(ds); e != nil {
			return e.Error(), e
		}
		snap.add("warehouse", "dataset", project+":"+ds, ds)
		if i >= 200 {
			continue // tables of the first 200 datasets; the rest are listed by name only
		}
		var tables []struct {
			TableReference struct{ TableID string } `json:"tableReference"`
			Type           string                   `json:"type"`
		}
		if _, e := jsonCLI(&tables, "bq", "ls", "--project_id="+project, "--format=json", "--max_results=1000", project+":"+ds); e != nil {
			continue
		}
		for _, t := range tables {
			if e := requirePlatformName(t.TableReference.TableID); e != nil {
				return e.Error(), e
			}
			snap.add("warehouse", strings.ToLower(t.Type), project+":"+ds+"."+t.TableReference.TableID, ds+"."+t.TableReference.TableID, t.TableReference.TableID)
		}
	}
	return "", nil
}

// --- AWS

func awsDynamoDB(snap *platformSnapshot, r []string) (string, error) {
	var out struct{ TableNames []string }
	if msg, e := jsonCLI(&out, "aws", append([]string{"dynamodb", "list-tables"}, r...)...); e != nil {
		return msg, e
	}
	account, region, _ := strings.Cut(snap.Scope, "/")
	for _, t := range out.TableNames {
		if e := requirePlatformName(t); e != nil {
			return e.Error(), e
		}
		snap.add("document_db", "table", "arn:aws:dynamodb:"+region+":"+account+":table/"+t, t)
	}
	return "", nil
}

func awsRDS(snap *platformSnapshot, r []string) (string, error) {
	var inst struct {
		DBInstances []struct {
			DBInstanceIdentifier, DBInstanceArn, DBName, Engine, DBInstanceStatus string
			Endpoint                                                              struct{ Address string }
		}
	}
	if msg, e := jsonCLI(&inst, "aws", append([]string{"rds", "describe-db-instances"}, r...)...); e != nil {
		return msg, e
	}
	for _, i := range inst.DBInstances {
		snap.add("sql_db", "instance", i.DBInstanceArn, i.DBInstanceIdentifier, i.Endpoint.Address)
		snap.Resources[len(snap.Resources)-1].State = i.DBInstanceStatus
		if i.DBName != "" {
			snap.add("sql_db", "database", i.DBInstanceArn+"/"+i.DBName, i.DBName)
		}
	}
	var clusters struct {
		DBClusters []struct {
			DBClusterIdentifier, DBClusterArn, DatabaseName, Endpoint, ReaderEndpoint, Status string
		}
	}
	if msg, e := jsonCLI(&clusters, "aws", append([]string{"rds", "describe-db-clusters"}, r...)...); e != nil {
		return msg, e
	}
	for _, c := range clusters.DBClusters {
		snap.add("sql_db", "cluster", c.DBClusterArn, c.DBClusterIdentifier, c.Endpoint, c.ReaderEndpoint)
		snap.Resources[len(snap.Resources)-1].State = c.Status
		if c.DatabaseName != "" {
			snap.add("sql_db", "database", c.DBClusterArn+"/"+c.DatabaseName, c.DatabaseName)
		}
	}
	return "", nil
}

// awsS3 lists the account's buckets (S3 lists them for every region at once).
func awsS3(snap *platformSnapshot) (string, error) {
	var out struct{ Buckets []struct{ Name string } }
	if msg, e := jsonCLI(&out, "aws", "s3api", "list-buckets", "--output", "json"); e != nil {
		return msg, e
	}
	for _, b := range out.Buckets {
		if e := requirePlatformName(b.Name); e != nil {
			return e.Error(), e
		}
		snap.add("object_storage", "bucket", "arn:aws:s3:::"+b.Name, b.Name, "s3://"+b.Name)
	}
	return "", nil
}

// --- Azure

func azureCosmos(snap *platformSnapshot, sub string) (string, error) {
	var accounts []struct{ ID, Name, ResourceGroup, DocumentEndpoint string }
	if msg, e := jsonCLI(&accounts, "az", "cosmosdb", "list", "--subscription", sub, "--output", "json"); e != nil {
		return msg, e
	}
	for _, a := range accounts {
		snap.add("document_db", "account", a.ID, a.Name, hostOf(a.DocumentEndpoint))
		at := []string{"--account-name", a.Name, "--resource-group", a.ResourceGroup, "--subscription", sub, "--output", "json"}
		var dbs []struct{ ID, Name string }
		if _, e := jsonCLI(&dbs, "az", append([]string{"cosmosdb", "sql", "database", "list"}, at...)...); e != nil {
			continue // accounts of another API (MongoDB, Cassandra) are captured by name only
		}
		for _, d := range dbs {
			snap.add("document_db", "database", d.ID, d.Name)
			var cs []struct{ ID, Name string }
			if _, e := jsonCLI(&cs, "az", append([]string{"cosmosdb", "sql", "container", "list", "--database-name", d.Name}, at...)...); e == nil {
				for _, c := range cs {
					snap.add("document_db", "container", c.ID, c.Name)
				}
			}
		}
	}
	return "", nil
}

// azureSQL lists Azure SQL and PostgreSQL flexible servers with their databases.
func azureSQL(snap *platformSnapshot, sub string) (string, error) {
	for _, family := range [][]string{{"sql", "server"}, {"postgres", "flexible-server"}} {
		var servers []struct{ ID, Name, ResourceGroup, FullyQualifiedDomainName, State string }
		if msg, e := jsonCLI(&servers, "az", append(append([]string{}, family...), "list", "--subscription", sub, "--output", "json")...); e != nil {
			return msg, e
		}
		for _, sv := range servers {
			snap.add("sql_db", "server", sv.ID, sv.Name, sv.FullyQualifiedDomainName)
			snap.Resources[len(snap.Resources)-1].State = sv.State
			args := []string{"sql", "db", "list", "--server", sv.Name}
			if family[0] == "postgres" {
				args = []string{"postgres", "flexible-server", "db", "list", "--server-name", sv.Name}
			}
			var dbs []struct{ ID, Name string }
			if _, e := jsonCLI(&dbs, "az", append(args, "--resource-group", sv.ResourceGroup, "--subscription", sub, "--output", "json")...); e == nil {
				for _, d := range dbs {
					snap.add("sql_db", "database", d.ID, d.Name)
				}
			}
		}
	}
	return "", nil
}

// azureStorage lists storage accounts (their containers need data-plane access and are not read).
func azureStorage(snap *platformSnapshot, sub string) (string, error) {
	var accounts []struct {
		ID, Name         string
		PrimaryEndpoints struct{ Blob string } `json:"primaryEndpoints"`
	}
	if msg, e := jsonCLI(&accounts, "az", "storage", "account", "list", "--subscription", sub, "--output", "json"); e != nil {
		return msg, e
	}
	for _, a := range accounts {
		snap.add("object_storage", "account", a.ID, a.Name, hostOf(a.PrimaryEndpoints.Blob))
	}
	return "", nil
}

// hostOf returns the host of a URL or host:port value.
func hostOf(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, "://"); i >= 0 {
		v = v[i+3:]
	}
	if i := strings.IndexAny(v, "/?"); i >= 0 {
		v = v[:i]
	}
	if i := strings.LastIndex(v, "@"); i >= 0 {
		v = v[i+1:]
	}
	if i := strings.LastIndex(v, ":"); i >= 0 && !strings.Contains(v[i+1:], ".") {
		v = v[:i]
	}
	return strings.ToLower(v)
}
