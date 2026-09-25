package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCandidateStructureUsesRequiredSectionContracts(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		path    string
		body    string
		missing string
	}{
		{
			path:    "25-Topics/event.md",
			body:    "---\nnombre-raw: event\nsistema: SYS\ntags: [topic]\ntipo: topic\n---\n## Contrato histórico\n## Infraestructura verificada\n## Limitaciones y desconocimientos\n## Qué representa\n",
			missing: "Contrato",
		},
		{
			path:    "30-Flujos/process.md",
			body:    "---\nsistema: SYS\ntags: [flow]\ntipo: flujo\n---\n## Diagrama de componentes\n```mermaid\nflowchart LR\nsubgraph S\nA\nend\n```\n## Diagrama de flujo\n```mermaid\nflowchart LR\nA --> B\n```\n## Cómo se dispara\n## Participantes\n## Paso a paso\n## Pendientes\n## Qué resuelve\n",
			missing: "Disparador",
		},
	}
	for _, tc := range tests {
		t.Run(tc.missing, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(tc.path))
			if e := os.MkdirAll(filepath.Dir(path), 0o755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(path, []byte(tc.body), 0o644); e != nil {
				t.Fatal(e)
			}
			issues, e := CandidateStructure(root, tc.path)
			if e != nil {
				t.Fatal(e)
			}
			for _, issue := range issues {
				if issue.Code == "missing-required-section" && issue.Field == tc.missing {
					return
				}
			}
			t.Fatalf("missing required-section issue for %q: %#v", tc.missing, issues)
		})
	}
}
