package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The helper executable exercises real subprocess argv/env boundaries without
// network access, stored credentials, shell syntax, or a gh installation.
func TestMain(m *testing.M) {
	if os.Getenv("KOS_INVENTORY_GH_HELPER") == "1" {
		fakeGH()
		return
	}
	os.Exit(m.Run())
}
func fakeGH() {
	args := os.Args[1:]
	if os.Getenv("GH_HOST") != "github.com" {
		fmt.Fprintln(os.Stderr, "wrong host")
		os.Exit(2)
	}
	emit := func(v any) { json.NewEncoder(os.Stdout).Encode(v) }
	if len(args) >= 2 && args[0] == "auth" && args[1] == "token" {
		if os.Getenv("GH_TOKEN") != "" || os.Getenv("GITHUB_TOKEN") != "" {
			os.Exit(3)
		}
		fmt.Print("stored-token")
		return
	}
	if len(args) >= 2 && args[0] == "auth" && args[1] == "status" {
		emit(object{"hosts": object{"github.com": []object{{"login": "chosen", "active": true, "state": "success"}}}})
		return
	}
	if len(args) >= 2 && args[0] == "api" && args[1] == "graphql" {
		token := os.Getenv("GH_TOKEN")
		if token == "bad-token" {
			os.Exit(4)
		}
		query := ""
		for _, arg := range args {
			if strings.HasPrefix(arg, "query=") {
				query = arg
			}
		}
		if strings.Contains(query, "viewer{") {
			login := "stored-user"
			if token == "ambient-token" {
				login = "ambient-user"
			}
			emit(object{"data": object{"viewer": object{"login": login}, "organization": object{"login": "example"}}})
			return
		}
		ref := object{"name": "main", "target": object{"oid": "123456789abcdef0123456789abcdef0123456789"}}
		if strings.Contains(query, "repository(owner:") {
			if os.Getenv("KOS_INVENTORY_MISSING_REF") == "1" {
				ref = nil
			}
			emit(object{"data": object{"repository": object{"ref": ref}}})
			return
		}
		emit(object{"data": object{"organization": object{"repositories": object{"nodes": []object{{"name": "team-reader", "isArchived": false, "main": ref}}, "pageInfo": object{"hasNextPage": false}}}}})
		return
	}
	os.Exit(5)
}
func fakeGHPath(t *testing.T) {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	name := "gh"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if e = os.WriteFile(filepath.Join(dir, name), b, 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", dir)
	t.Setenv("KOS_INVENTORY_GH_HELPER", "1")
	t.Setenv("GH_HOST", "enterprise.example.invalid")
}
func TestGitHubExplicitAccountOverridesAmbientAndPinsHost(t *testing.T) {
	fakeGHPath(t)
	t.Setenv("GH_TOKEN", "ambient-token")
	t.Setenv("GITHUB_TOKEN", "other-token")
	g, e := authenticate(context.Background(), "example", "chosen")
	if e != nil || g.Login != "stored-user" || g.Source != "explicit" || g.token != "stored-token" {
		t.Fatal(g.Login, g.Source, e)
	}
	b, e := json.Marshal(g)
	if e != nil || strings.Contains(string(b), "stored-token") {
		t.Fatal("credential serialized")
	}
}
func TestGitHubAmbientFailureDoesNotFallBack(t *testing.T) {
	fakeGHPath(t)
	t.Setenv("GH_TOKEN", "bad-token")
	if _, e := authenticate(context.Background(), "example", ""); e == nil {
		t.Fatal("bad explicit environment token fell back")
	}
}
func TestGitHubActiveStoredAccount(t *testing.T) {
	fakeGHPath(t)
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	g, e := authenticate(context.Background(), "example", "")
	if e != nil || g.Source != "active" || g.Login != "stored-user" {
		t.Fatal(g.Login, g.Source, e)
	}
}
func TestReferenceBranchCannotOverwriteRepositoryIdentity(t *testing.T) {
	fakeGHPath(t)
	for _, branch := range []string{"name", "sha", "branch", "main", "release/stable"} {
		s := scope{prefixes: []string{"team-"}, instance: object{"sources": object{"reference_branches": object{"team-reader": branch}}}}
		repos, e := remoteRepos(context.Background(), s, "example", github{token: "stored-token"})
		if e != nil || len(repos) != 1 || repos[0]["name"] != "team-reader" || repos[0]["branch"] != branch || repos[0]["sha"] != "123456789abc" {
			t.Fatalf("branch %s: %v %v", branch, repos, e)
		}
	}
}
func TestMissingExplicitReferenceNeverFallsBack(t *testing.T) {
	fakeGHPath(t)
	t.Setenv("KOS_INVENTORY_MISSING_REF", "1")
	s := scope{prefixes: []string{"team-"}, instance: object{"sources": object{"reference_branches": object{"team-reader": "release/stable"}}}}
	repos, e := remoteRepos(context.Background(), s, "example", github{token: "stored-token"})
	if e != nil || len(repos) != 1 || repos[0]["branch"] != nil || repos[0]["sha"] != nil {
		t.Fatal(repos, e)
	}
}
