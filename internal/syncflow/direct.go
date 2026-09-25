package syncflow

import (
	"bytes"
	"documentation-vault/internal/audit"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

type directSourceArg struct {
	repository, checkout, productionRef, oldOID, newOID, outcome string
}

func directArguments(args []string) ([]string, []directSourceArg, map[string]string, error) {
	normalized := []string{args[0]}
	sources := []directSourceArg{}
	checkouts := map[string]string{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--source":
			if i+6 >= len(args) {
				return nil, nil, nil, fail("source-arguments-required")
			}
			sources = append(sources, directSourceArg{args[i+1], args[i+2], args[i+3], args[i+4], args[i+5], args[i+6]})
			i += 6
		case "--checkout":
			if i+2 >= len(args) {
				return nil, nil, nil, fail("checkout-arguments-required")
			}
			if checkouts[args[i+1]] != "" {
				return nil, nil, nil, fail("duplicate-checkout")
			}
			checkouts[args[i+1]] = args[i+2]
			i += 2
		default:
			normalized = append(normalized, args[i])
		}
	}
	return normalized, sources, checkouts, nil
}

var directPrivateKeyRE = regexp.MustCompile(`-----BEGIN (?:[A-Z0-9 ]+ )?PRIVATE KEY-----`)
var directTokenRE = regexp.MustCompile(`(?:gh[pousr]_[A-Za-z0-9]{10,}|xox[baprs]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16}|Bearer[ \t]+[A-Za-z0-9._~-]{10,}|sk-(?:proj-)?[A-Za-z0-9_-]{20,})`)
var directCommitLineRE = regexp.MustCompile(`^[ \t]*commit-analizado[ \t]*:[ \t]*['"]?([^'"# \t]+)`)

func productionBranch(ref string) (string, bool) {
	for _, prefix := range []string{"refs/heads/", "refs/remotes/origin/"} {
		if strings.HasPrefix(ref, prefix) {
			name := strings.TrimPrefix(ref, prefix)
			return name, branch(name)
		}
	}
	return "", false
}

func sourceBinding(arg directSourceArg, date string) (map[string]any, error) {
	if !nameRE.MatchString(arg.repository) || !oidRE.MatchString(arg.newOID) || !one(arg.outcome, "write", "no-documentation-change", "no-durable-node") {
		return nil, fail("source-binding-invalid")
	}
	if arg.oldOID != "" && !oidRE.MatchString(arg.oldOID) {
		return nil, fail("source-binding-invalid")
	}
	branchName, ok := productionBranch(arg.productionRef)
	parsed, dateErr := time.Parse("2006-01-02", date)
	if !ok || dateErr != nil || parsed.Format("2006-01-02") != date {
		return nil, fail("source-binding-invalid")
	}
	source, e := inspectSource(arg.checkout)
	if e != nil {
		return nil, e
	}
	if !strings.EqualFold(strings.TrimSuffix(source.name, ".git"), arg.repository) {
		return nil, fail("source-identity-mismatch")
	}
	current, e := sourceResolve(source.path, arg.productionRef)
	if e != nil || current != arg.newOID {
		return nil, fail("source-ref-stale")
	}
	if arg.oldOID != "" {
		if _, e = sourceResolve(source.path, arg.oldOID); e != nil {
			return nil, fail("source-old-oid-unavailable")
		}
		if _, e = sourceGit(source.path, nil, "merge-base", "--is-ancestor", arg.oldOID, arg.newOID); e != nil {
			return nil, fail("source-range-invalid")
		}
	}
	return map[string]any{"repository": arg.repository, "identity": source.identity, "old_oid": arg.oldOID, "new_oid": arg.newOID, "production_ref": arg.productionRef, "branch": branchName, "analysis_date": date, "outcome": arg.outcome}, nil
}

func validateSourceBindings(value any) error {
	bindings, ok := value.([]any)
	if !ok || len(bindings) == 0 {
		return fail("source-binding-invalid")
	}
	prev := ""
	for _, value := range bindings {
		binding := obj(value)
		if !exact(binding, "repository identity old_oid new_oid production_ref branch analysis_date outcome") {
			return fail("source-binding-invalid")
		}
		repository, identity := str(binding["repository"]), str(binding["identity"])
		branchName, validRef := productionBranch(str(binding["production_ref"]))
		date := str(binding["analysis_date"])
		parsed, dateErr := time.Parse("2006-01-02", date)
		if !nameRE.MatchString(repository) || repository <= prev || !nonempty(identity) || localPath.MatchString(identity) || strings.Contains(identity, "@") || !oidRE.MatchString(str(binding["new_oid"])) || str(binding["old_oid"]) != "" && !oidRE.MatchString(str(binding["old_oid"])) || !validRef || branchName != binding["branch"] || dateErr != nil || parsed.Format("2006-01-02") != date || !one(binding["outcome"], "write", "no-documentation-change", "no-durable-node") {
			return fail("source-binding-invalid")
		}
		prev = repository
	}
	return nil
}

func validateBoundSources(bindings []any, checkouts map[string]string) error {
	if len(checkouts) != len(bindings) {
		return fail("source-checkout-coverage-mismatch")
	}
	for _, value := range bindings {
		binding := obj(value)
		repository := str(binding["repository"])
		checkout := checkouts[repository]
		if checkout == "" {
			return fail("source-checkout-coverage-mismatch")
		}
		source, e := inspectSource(checkout)
		if e != nil {
			return e
		}
		if !strings.EqualFold(strings.TrimSuffix(source.name, ".git"), repository) || source.identity != binding["identity"] {
			return fail("source-identity-mismatch")
		}
		current, e := sourceResolve(source.path, str(binding["production_ref"]))
		if e != nil || current != binding["new_oid"] {
			return fail("source-ref-stale")
		}
		old := str(binding["old_oid"])
		if old != "" {
			if _, e = sourceResolve(source.path, old); e != nil {
				return fail("source-old-oid-unavailable")
			}
			if _, e = sourceGit(source.path, nil, "merge-base", "--is-ancestor", old, current); e != nil {
				return fail("source-range-invalid")
			}
		}
	}
	return nil
}

func directScanIssues(candidate string, deleted []string, expectedCommits map[string]string) ([]any, error) {
	deletedSet := map[string]bool{}
	for _, path := range deleted {
		deletedSet[path] = true
	}
	issues := []any{}
	e := filepath.WalkDir(candidate, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fail("unsafe-path")
		}
		if entry.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(candidate, path)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		if deletedSet[rel] {
			return nil
		}
		body, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		structureIssues, e := audit.CandidateStructure(candidate, rel)
		if e != nil {
			return e
		}
		for _, issue := range structureIssues {
			value := map[string]any{"code": issue.Code, "path": rel, "line": issue.Line}
			if issue.Field != "" {
				value["field"] = issue.Field
			}
			issues = append(issues, value)
		}
		lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
		frontmatterEnd := -1
		if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
			for i := 1; i < len(lines); i++ {
				if strings.TrimSpace(lines[i]) == "---" {
					frontmatterEnd = i
					break
				}
			}
		}
		for lineNo, line := range lines {
			codes := map[string]bool{}
			if strings.HasPrefix(rel, "20-Repos/") && lineNo > 0 && lineNo < frontmatterEnd {
				if match := directCommitLineRE.FindStringSubmatch(line); match != nil {
					if shortOIDRE.MatchString(match[1]) {
						stem := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
						if expected := expectedCommits[strings.ToLower(stem)]; expected != "" && match[1] != expected {
							codes["repo-commit-source-mismatch"] = true
						}
					}
				}
			}
			if directPrivateKeyRE.MatchString(line) || directTokenRE.MatchString(line) {
				codes["credential-literal"] = true
			}
			for _, match := range credentialAssignRE.FindAllStringSubmatch(line, -1) {
				if len(match) == 3 && credentialName(match[1]) && len(literalValues(match[2], true)) > 0 {
					codes["credential-assignment"] = true
				}
			}
			if processRE.MatchString(line) {
				codes["sync-process-language"] = true
			}
			for _, code := range []string{"repo-commit-source-mismatch", "credential-literal", "credential-assignment", "sync-process-language"} {
				if codes[code] {
					issues = append(issues, map[string]any{"code": code, "path": rel, "line": lineNo + 1})
				}
			}
		}
		return nil
	})
	return issues, e
}

func directDocsChanged(m map[string]any) bool {
	present := map[string]bool{}
	for _, path := range arr(m["base_present"]) {
		present[str(path)] = true
	}
	deleted := map[string]bool{}
	for _, path := range arr(m["deleted_files"]) {
		deleted[str(path)] = true
	}
	for path, candidate := range obj(m["candidate_files"]) {
		if candidate != obj(m["base_files"])[path] || !present[path] || deleted[path] {
			return true
		}
	}
	return false
}

func directRepositoryNote(path, repository string) bool {
	if !strings.HasPrefix(path, "20-Repos/") || filepath.Ext(path) != ".md" {
		return false
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return name == repository || name == repositoryPrefixRE.ReplaceAllString(repository, "")
}

func directSourceNoteImage(vault string, m map[string]any, repository string) (int, bool, error) {
	final := map[string]bool{}
	root := filepath.Join(vault, "20-Repos")
	e := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(vault, path)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		if directRepositoryNote(rel, repository) {
			final[rel] = true
		}
		return nil
	})
	if e != nil && !os.IsNotExist(e) {
		return 0, false, e
	}
	present, deleted := map[string]bool{}, map[string]bool{}
	for _, value := range arr(m["base_present"]) {
		present[str(value)] = true
	}
	for _, value := range arr(m["deleted_files"]) {
		deleted[str(value)] = true
	}
	changed := false
	for path, candidate := range obj(m["candidate_files"]) {
		if !directRepositoryNote(path, repository) {
			continue
		}
		if candidate != obj(m["base_files"])[path] || !present[path] || deleted[path] {
			changed = true
		}
		if deleted[path] {
			delete(final, path)
		} else {
			final[path] = true
		}
	}
	return len(final), changed, nil
}

func validateSourceOutcomes(vault string, m map[string]any) error {
	changed, hasWrite := directDocsChanged(m), false
	for _, value := range arr(m["source_bindings"]) {
		binding := obj(value)
		outcome := str(binding["outcome"])
		if outcome == "write" {
			hasWrite = true
			continue
		}
		notes, noteChanged, e := directSourceNoteImage(vault, m, str(binding["repository"]))
		if e != nil {
			return e
		}
		if outcome == "no-documentation-change" && (notes != 1 || noteChanged) || outcome == "no-durable-node" && (notes != 0 || noteChanged) {
			return fail("source-outcome-note-mismatch")
		}
	}
	if changed != hasWrite {
		return fail("source-outcome-mismatch")
	}
	return nil
}

func acknowledgementRecords(vault string) ([]byte, bool, map[string]any, error) {
	base, present, e := content(vault, ackPath, true)
	if e != nil {
		return nil, false, nil, e
	}
	recordMap, e := acknowledgementDocument(base, false)
	if e != nil {
		return nil, false, nil, e
	}
	return base, present, recordMap, nil
}

func acknowledgementEntry(binding map[string]any, changed bool) any {
	if binding["outcome"] == "write" {
		return nil
	}
	return map[string]any{"repository": binding["repository"], "branch": binding["branch"], "analyzed_sha": str(binding["new_oid"])[:12], "decision": binding["outcome"], "analysis_date": binding["analysis_date"]}
}

func acknowledgementBindings(vault string, bindings []any, changed bool) (map[string]any, error) {
	_, _, records, e := acknowledgementRecords(vault)
	if e != nil {
		return nil, e
	}
	out := map[string]any{}
	for _, value := range bindings {
		binding := obj(value)
		repository := str(binding["repository"])
		base := records[repository]
		result := acknowledgementEntry(binding, changed)
		// A documentation write retires only the acknowledgement for the same analyzed SHA.
		if binding["outcome"] == "write" && obj(base) != nil && obj(base)["analyzed_sha"] != str(binding["new_oid"])[:12] {
			result = base
		}
		out[repository] = map[string]any{"base": base, "result": result}
	}
	return out, nil
}

func acknowledgementDocumentBytes(records map[string]any) ([]byte, bool, error) {
	if len(records) == 0 {
		return nil, false, nil
	}
	values := []any{}
	for _, repository := range sortedKeys(records) {
		values = append(values, records[str(repository)])
	}
	b, e := canonical(map[string]any{"version": json.Number("1"), "repositories": values})
	if e != nil {
		return nil, false, e
	}
	return append(b, '\n'), true, nil
}

func validateAcknowledgementBindings(bindings map[string]any, sources []any) error {
	if len(bindings) != len(sources) {
		return fail("acknowledgement-binding-invalid")
	}
	for _, source := range sources {
		repository := str(obj(source)["repository"])
		entry := obj(bindings[repository])
		if !exact(entry, "base result") {
			return fail("acknowledgement-binding-invalid")
		}
		for _, key := range []string{"base", "result"} {
			if entry[key] == nil {
				continue
			}
			record := obj(entry[key])
			if !exact(record, "repository branch analyzed_sha decision analysis_date") || record["repository"] != repository {
				return fail("acknowledgement-binding-invalid")
			}
		}
		expected := acknowledgementEntry(obj(source), obj(source)["outcome"] == "write")
		base := entry["base"]
		if obj(source)["outcome"] == "write" && obj(base) != nil && obj(base)["analyzed_sha"] != str(obj(source)["new_oid"])[:12] {
			expected = base
		}
		if !reflect.DeepEqual(entry["result"], expected) {
			return fail("acknowledgement-binding-invalid")
		}
	}
	return nil
}

func directBaseManifest(vault, candidate, evidence string, evidencePaths, deleted []string, projection string) (map[string]any, error) {
	m, e := freeze(vault, candidate, evidence, evidencePaths, deleted, projection)
	if e != nil && e.Error() == "empty-candidate" && len(deleted) == 0 && projection == "" {
		evidenceRoot, safeErr := safe(evidence, true)
		if safeErr != nil {
			return nil, safeErr
		}
		evidenceFiles := map[string]any{}
		for _, path := range evidencePaths {
			if _, exists := evidenceFiles[path]; exists {
				return nil, fail("duplicate-evidence")
			}
			body, _, readErr := content(evidenceRoot, path, false)
			if readErr != nil {
				return nil, readErr
			}
			evidenceFiles[path] = hash(body)
		}
		if len(evidenceFiles) == 0 {
			return nil, fail("empty-evidence")
		}
		m = map[string]any{"version": json.Number("1"), "candidate_files": map[string]any{}, "base_files": map[string]any{}, "base_present": []any{}, "deleted_files": []any{}, "evidence_files": evidenceFiles, "connections": map[string]any{}}
		e = nil
	}
	return m, e
}

func freezeBound(vault, candidate, evidence string, evidencePaths, deleted []string, projection, date string, sources []directSourceArg) (map[string]any, []any, error) {
	if projection != "" {
		return nil, nil, fail("direct-projection-not-supported")
	}
	expected := map[string]string{}
	for _, source := range sources {
		if source.outcome == "write" && oidRE.MatchString(source.newOID) {
			expected[strings.ToLower(repositoryPrefixRE.ReplaceAllString(source.repository, ""))] = source.newOID[:12]
		}
	}
	issues, e := directScanIssues(candidate, deleted, expected)
	if e != nil || len(issues) > 0 {
		return nil, issues, e
	}
	m, e := directBaseManifest(vault, candidate, evidence, evidencePaths, deleted, projection)
	if e != nil {
		return nil, nil, e
	}
	bindings := []any{}
	for _, source := range sources {
		binding, e := sourceBinding(source, date)
		if e != nil {
			return nil, nil, e
		}
		bindings = append(bindings, binding)
	}
	sort.Slice(bindings, func(i, j int) bool { return str(obj(bindings[i])["repository"]) < str(obj(bindings[j])["repository"]) })
	m["version"] = json.Number("2")
	m["source_bindings"] = bindings
	ack, e := acknowledgementBindings(vault, bindings, directDocsChanged(m))
	if e != nil {
		return nil, nil, e
	}
	m["acknowledgements"] = ack
	if _, e = manifest(m); e != nil {
		return nil, nil, e
	}
	if e = validateSourceOutcomes(vault, m); e != nil {
		return nil, nil, e
	}
	return m, nil, nil
}

func checkBound(m map[string]any, reviewValue any, vault, candidate, evidence, projection string, checkouts map[string]string) ([]any, error) {
	if m["version"] != json.Number("2") {
		return nil, fail("direct-manifest-required")
	}
	if e := review(reviewValue, m); e != nil {
		return nil, e
	}
	if projection != "" {
		return nil, fail("direct-projection-not-supported")
	}
	if e := validateSourceOutcomes(vault, m); e != nil {
		return nil, e
	}
	if e := validateBoundSources(arr(m["source_bindings"]), checkouts); e != nil {
		return nil, e
	}
	evidencePaths := []string{}
	for path := range obj(m["evidence_files"]) {
		evidencePaths = append(evidencePaths, path)
	}
	deleted, _ := stringsFrom(m["deleted_files"])
	expected := map[string]string{}
	for _, value := range arr(m["source_bindings"]) {
		binding := obj(value)
		if binding["outcome"] == "write" {
			expected[strings.ToLower(repositoryPrefixRE.ReplaceAllString(str(binding["repository"]), ""))] = str(binding["new_oid"])[:12]
		}
	}
	issues, e := directScanIssues(candidate, deleted, expected)
	if e != nil || len(issues) > 0 {
		return issues, e
	}
	current, e := directBaseManifest(vault, candidate, evidence, evidencePaths, deleted, projection)
	if e != nil {
		return nil, e
	}
	current["version"] = json.Number("2")
	current["source_bindings"] = m["source_bindings"]
	ack, e := acknowledgementBindings(vault, arr(m["source_bindings"]), directDocsChanged(current))
	if e != nil {
		return nil, e
	}
	current["acknowledgements"] = ack
	if !reflect.DeepEqual(current, m) {
		return nil, fail("candidate-base-evidence-or-source-stale")
	}
	return nil, nil
}

type directImage struct {
	path                       string
	base, result               []byte
	basePresent, resultPresent bool
}

func directImages(vault, candidate string, m map[string]any) ([]directImage, error) {
	present, deleted := map[string]bool{}, map[string]bool{}
	for _, path := range arr(m["base_present"]) {
		present[str(path)] = true
	}
	for _, path := range arr(m["deleted_files"]) {
		deleted[str(path)] = true
	}
	images := []directImage{}
	for _, value := range sortedKeys(obj(m["candidate_files"])) {
		path := str(value)
		base, basePresent, e := content(vault, path, true)
		if e != nil {
			return nil, e
		}
		var result []byte
		resultPresent := !deleted[path]
		if resultPresent {
			result, _, e = content(candidate, path, false)
			if e != nil {
				return nil, e
			}
		}
		if basePresent != present[path] || hash(base) != obj(m["base_files"])[path] || hash(result) != obj(m["candidate_files"])[path] {
			return nil, fail("candidate-base-or-evidence-stale")
		}
		if basePresent != resultPresent || !bytes.Equal(base, result) {
			images = append(images, directImage{path, base, result, basePresent, resultPresent})
		}
	}
	base, basePresent, records, e := acknowledgementRecords(vault)
	if e != nil {
		return nil, e
	}
	bindings := obj(m["acknowledgements"])
	for repository, value := range bindings {
		if !reflect.DeepEqual(records[repository], obj(value)["base"]) {
			return nil, fail("acknowledgement-binding-stale")
		}
		if obj(value)["result"] == nil {
			delete(records, repository)
		} else {
			records[repository] = obj(value)["result"]
		}
	}
	result, resultPresent, e := acknowledgementDocumentBytes(records)
	if e != nil {
		return nil, e
	}
	if basePresent != resultPresent || !bytes.Equal(base, result) {
		images = append(images, directImage{ackPath, base, result, basePresent, resultPresent})
	}
	return images, nil
}

func directRecord(image directImage, result bool) fileRecord {
	present, body := image.basePresent, image.base
	if result {
		present, body = image.resultPresent, image.result
	}
	if !present {
		return fileRecord{"missing", hash(nil)}
	}
	return fileRecord{"file", hash(body)}
}

func checkDirectInputs(m map[string]any, reviewValue any, candidate, evidence string, checkouts map[string]string) ([]any, error) {
	if e := review(reviewValue, m); e != nil {
		return nil, e
	}
	if e := validateBoundSources(arr(m["source_bindings"]), checkouts); e != nil {
		return nil, e
	}
	deleted, _ := stringsFrom(m["deleted_files"])
	expected := map[string]string{}
	for _, value := range arr(m["source_bindings"]) {
		binding := obj(value)
		if binding["outcome"] == "write" {
			expected[strings.ToLower(repositoryPrefixRE.ReplaceAllString(str(binding["repository"]), ""))] = str(binding["new_oid"])[:12]
		}
	}
	issues, e := directScanIssues(candidate, deleted, expected)
	if e != nil || len(issues) > 0 {
		return issues, e
	}
	deletedSet := map[string]bool{}
	for _, path := range deleted {
		deletedSet[path] = true
	}
	for path, digest := range obj(m["candidate_files"]) {
		if deletedSet[path] {
			if digest != emptyHash {
				return nil, fail("candidate-stale")
			}
			continue
		}
		body, _, e := content(candidate, path, false)
		if e != nil || hash(body) != digest {
			return nil, fail("candidate-stale")
		}
	}
	for path, digest := range obj(m["evidence_files"]) {
		body, _, e := content(evidence, path, false)
		if e != nil || hash(body) != digest {
			return nil, fail("evidence-stale")
		}
	}
	return nil, nil
}

func directJournal(root, manifestDigest, reviewDigest string, images []directImage) error {
	dir := []string{"direct", "active", manifestDigest}
	entries := []any{}
	for i, image := range images {
		name := itoa(i)
		if e := atomicStateBytes(root, append(dir, name+".base"), image.base); e != nil {
			return e
		}
		if e := atomicStateBytes(root, append(dir, name+".result"), image.result); e != nil {
			return e
		}
		entries = append(entries, map[string]any{"path": image.path, "base_kind": directRecord(image, false).kind, "base_digest": directRecord(image, false).digest, "result_kind": directRecord(image, true).kind, "result_digest": directRecord(image, true).digest, "file": name})
	}
	return atomicState(root, append(dir, "journal.json"), map[string]any{"version": json.Number("1"), "manifest_digest": manifestDigest, "review_digest": reviewDigest, "status": "prepared", "entries": entries})
}

func loadDirectJournal(root, manifestDigest, reviewDigest string) ([]directImage, error) {
	dir := []string{"direct", "active", manifestDigest}
	j, e := readState(root, append(dir, "journal.json")...)
	if e != nil {
		return nil, e
	}
	if !exact(j, "version manifest_digest review_digest status entries") || j["version"] != json.Number("1") || j["manifest_digest"] != manifestDigest || j["review_digest"] != reviewDigest || j["status"] != "prepared" {
		return nil, fail("direct-journal-invalid")
	}
	images := []directImage{}
	for _, value := range arr(j["entries"]) {
		entry := obj(value)
		if !exact(entry, "path base_kind base_digest result_kind result_digest file") || relative(str(entry["path"]), false) != nil || !one(entry["base_kind"], "file", "missing") || !one(entry["result_kind"], "file", "missing") {
			return nil, fail("direct-journal-invalid")
		}
		basePath, e := statePath(root, append(dir, str(entry["file"])+".base")...)
		if e != nil {
			return nil, e
		}
		resultPath, e := statePath(root, append(dir, str(entry["file"])+".result")...)
		if e != nil {
			return nil, e
		}
		base, e := os.ReadFile(basePath)
		if e != nil {
			return nil, e
		}
		result, e := os.ReadFile(resultPath)
		if e != nil {
			return nil, e
		}
		image := directImage{path: str(entry["path"]), base: base, result: result, basePresent: entry["base_kind"] == "file", resultPresent: entry["result_kind"] == "file"}
		if directRecord(image, false).digest != entry["base_digest"] || directRecord(image, true).digest != entry["result_digest"] {
			return nil, fail("direct-journal-invalid")
		}
		images = append(images, image)
	}
	return images, nil
}

func writeDirectReceipt(root, manifestDigest, reviewDigest string, images []directImage) (map[string]any, error) {
	receipt := map[string]any{"version": json.Number("1"), "code": "direct-publication-complete", "status": "complete", "manifest_digest": manifestDigest, "review_digest": reviewDigest, "result_files": map[string]any{}}
	for _, image := range images {
		obj(receipt["result_files"])[image.path] = directRecord(image, true).digest
	}
	receiptDigest, _ := digest(receipt)
	receipt["receipt_digest"] = receiptDigest
	if e := atomicState(root, []string{"direct", "receipts", manifestDigest + ".json"}, receipt); e != nil {
		return nil, e
	}
	return receipt, nil
}

func directPublish(stateRoot, vault, candidate, evidence, projection string, m map[string]any, reviewValue any, checkouts map[string]string, failAfter, crashAfter int) (map[string]any, error) {
	issues, e := checkDirectInputs(m, reviewValue, candidate, evidence, checkouts)
	if e != nil || len(issues) > 0 {
		return map[string]any{"status": "blocked", "code": "output-scan-blocked", "issues": issues}, e
	}
	if e = os.MkdirAll(stateRoot, 0700); e != nil {
		return nil, e
	}
	stateRoot, e = safe(stateRoot, true)
	if e != nil {
		return nil, e
	}
	vault, e = safe(vault, true)
	if e != nil {
		return nil, e
	}
	vaultLockRoot := filepath.Join(vault, defaultStateRoot)
	if e = os.MkdirAll(vaultLockRoot, 0700); e != nil {
		return nil, e
	}
	vaultLockRoot, e = safe(vaultLockRoot, true)
	if e != nil {
		return nil, e
	}
	lockPath, e := statePath(vaultLockRoot, ".direct-publication.lock")
	if e != nil {
		return nil, e
	}
	lock := flock.New(lockPath)
	ok, e := lock.TryLock()
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, fail("sync-state-busy")
	}
	defer lock.Unlock()
	manifestDigest, _ := digest(m)
	reviewDigest, _ := digest(reviewValue)
	if e = validateSourceOutcomes(vault, m); e != nil {
		return nil, e
	}
	journalPath, e := statePath(stateRoot, "direct", "active", manifestDigest, "journal.json")
	if e != nil {
		return nil, e
	}
	if _, journalErr := os.Lstat(journalPath); os.IsNotExist(journalErr) {
		issues, e = checkBound(m, reviewValue, vault, candidate, evidence, projection, checkouts)
		if e != nil || len(issues) > 0 {
			return map[string]any{"status": "blocked", "code": "output-scan-blocked", "issues": issues}, e
		}
	} else if journalErr != nil {
		return nil, journalErr
	}
	receiptPath := []string{"direct", "receipts", manifestDigest + ".json"}
	if receipt, readErr := readState(stateRoot, receiptPath...); readErr == nil {
		if receipt["manifest_digest"] != manifestDigest || receipt["review_digest"] != reviewDigest {
			return nil, fail("direct-receipt-conflict")
		}
		images, e := loadDirectJournal(stateRoot, manifestDigest, reviewDigest)
		if e != nil {
			return nil, e
		}
		for _, image := range images {
			if image.path == ackPath {
				continue
			}
			target, e := applyTarget(vault, image.path)
			if e != nil {
				return nil, e
			}
			actual, e := actualRecord(target)
			if e != nil || actual != directRecord(image, true) {
				return nil, fail("published-note-drift")
			}
		}
		_, _, records, e := acknowledgementRecords(vault)
		if e != nil {
			return nil, e
		}
		for repository, value := range obj(m["acknowledgements"]) {
			if !reflect.DeepEqual(records[repository], obj(value)["result"]) {
				return nil, fail("published-note-drift")
			}
		}
		return map[string]any{"status": "pass", "code": "direct-publication-complete", "manifest_digest": manifestDigest, "receipt_digest": receipt["receipt_digest"], "reused": true}, nil
	} else if !os.IsNotExist(readErr) {
		return nil, readErr
	}
	images, e := loadDirectJournal(stateRoot, manifestDigest, reviewDigest)
	if os.IsNotExist(e) {
		images, e = directImages(vault, candidate, m)
		if e == nil {
			e = directJournal(stateRoot, manifestDigest, reviewDigest, images)
		}
	}
	if e != nil {
		return nil, e
	}
	pre, post := false, false
	for _, image := range images {
		target, e := applyTarget(vault, image.path)
		if e != nil {
			return nil, e
		}
		actual, e := actualRecord(target)
		if e != nil {
			return nil, e
		}
		before, after := directRecord(image, false), directRecord(image, true)
		if actual != before && actual != after {
			return nil, fail("destination-stale")
		}
		if before != after {
			pre = pre || actual == before
			post = post || actual == after
		}
	}
	if !pre && post {
		receipt, e := writeDirectReceipt(stateRoot, manifestDigest, reviewDigest, images)
		if e != nil {
			return nil, e
		}
		return map[string]any{"status": "pass", "code": "direct-publication-complete", "manifest_digest": manifestDigest, "receipt_digest": receipt["receipt_digest"], "reused": true}, nil
	}
	if pre && post {
		// Recover a process interruption by restoring only exact postimages. Any
		// unrelated edit was rejected above and is never overwritten.
		for i := len(images) - 1; i >= 0; i-- {
			image := images[i]
			target, e := applyTarget(vault, image.path)
			if e != nil {
				return nil, e
			}
			actual, e := actualRecord(target)
			if e != nil {
				return nil, e
			}
			if actual == directRecord(image, false) {
				continue
			}
			if actual != directRecord(image, true) {
				return nil, fail("destination-stale")
			}
			kind := directRecord(image, false).kind
			if e = writeImage(vault, image.path, kind, image.base); e != nil {
				return nil, fail("rollback-failed")
			}
		}
	}
	written := []directImage{}
	rollback := func(cause error) (map[string]any, error) {
		for i := len(written) - 1; i >= 0; i-- {
			image := written[i]
			target, e := applyTarget(vault, image.path)
			if e != nil {
				return nil, fail("rollback-failed")
			}
			actual, e := actualRecord(target)
			if e != nil || actual != directRecord(image, true) {
				return nil, fail("rollback-failed")
			}
			if e := writeImage(vault, image.path, directRecord(image, false).kind, image.base); e != nil {
				return nil, fail("rollback-failed")
			}
		}
		return nil, cause
	}
	for _, image := range images {
		target, e := applyTarget(vault, image.path)
		if e != nil {
			return rollback(e)
		}
		actual, e := actualRecord(target)
		if e != nil || actual != directRecord(image, false) {
			return rollback(fail("destination-stale"))
		}
		if e = writeImage(vault, image.path, directRecord(image, true).kind, image.result); e != nil {
			return rollback(e)
		}
		written = append(written, image)
		if crashAfter == len(written) {
			return nil, fail("simulated-crash")
		}
		if failAfter == len(written) {
			return rollback(fail("simulated-write-failure"))
		}
	}
	for _, image := range images {
		target, e := applyTarget(vault, image.path)
		if e != nil {
			return rollback(e)
		}
		actual, e := actualRecord(target)
		if e != nil || actual != directRecord(image, true) {
			return rollback(fail("publication-verification-failed"))
		}
	}
	receipt, e := writeDirectReceipt(stateRoot, manifestDigest, reviewDigest, images)
	if e != nil {
		return nil, e
	}
	return map[string]any{"status": "pass", "code": "direct-publication-complete", "manifest_digest": manifestDigest, "receipt_digest": receipt["receipt_digest"], "reused": false}, nil
}
