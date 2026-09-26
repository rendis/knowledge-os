package cell

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	knowledgeos "knowledge-os"
	"knowledge-os/internal/kernel"
)

const personal = "AGENTS.personal.md"

// knowledgeHints mark a directory that already holds a cell's knowledge.
var knowledgeHints = []string{"10-Sistemas", "00-Home.md", "30-Flujos", "instance.yaml"}

// state classifies a destination: missing, empty, installed (it holds a kernel lock),
// knowledge-without-lock or nonempty.
func state(dir string) string {
	entries, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return "missing"
	}
	if _, e := os.Stat(filepath.Join(dir, kernel.LockName)); e == nil {
		return "installed"
	}
	others := 0
	for _, entry := range entries {
		if entry.Name() != ".git" && entry.Name() != ".DS_Store" {
			others++
		}
	}
	if others == 0 {
		return "empty"
	}
	for _, hint := range knowledgeHints {
		if _, e := os.Stat(filepath.Join(dir, hint)); e == nil {
			return "knowledge-without-lock"
		}
	}
	return "nonempty"
}

func template(name, locale string, values map[string]string) (string, error) {
	dir := "kernel/templates"
	if locale == "es" {
		if _, e := fs.Stat(knowledgeos.Payload, path.Join(dir, "es", name)); e == nil {
			dir = path.Join(dir, "es")
		}
	}
	b, e := fs.ReadFile(knowledgeos.Payload, path.Join(dir, name))
	if e != nil {
		return "", e
	}
	text := string(b)
	for k, v := range values {
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
	}
	return text, nil
}

func homeGuidance(locale string) string {
	if locale == "es" {
		return "Consulta la configuración local vigente desde la raíz del vault con `kos config status` (instala `kos` si falta, ver el router); consulta `.agents/skills/use-vault-cli/SKILL.md`.\n"
	}
	return "Check the current local configuration from the vault root with `kos config status` (install `kos` when missing, see the router); see `.agents/skills/use-vault-cli/SKILL.md`.\n"
}

var startingIgnores = []string{".DS_Store", ".obsidian/workspace.json", ".obsidian/workspace-mobile.json", "/" + personal,
	"/.investigations/", "/.investigations-private/", "/.operations/", "/.knowledge-os-config.yaml",
	"/.knowledge-os-config.*.tmp", "/.plan/", "/.scratch/", "/output/", "__pycache__/"}

// bootstrap writes what a new cell owns: the folders, instance.yaml, Home, one note per system, the
// index notes and the ignore rules. The kernel itself is written by kernel.Update.
func bootstrap(root, instanceText string, inst map[string]any) error {
	locale := fmt.Sprint(field(inst["locale"], "notes"))
	topics := false
	for _, t := range items(field(inst["graph"], "enabled_types")) {
		topics = topics || t == "topic"
	}
	skeleton, e := fs.ReadDir(knowledgeos.Payload, "kernel/skeleton")
	if e != nil {
		return e
	}
	write := func(rel, text string) error {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if e := os.MkdirAll(filepath.Dir(p), 0o755); e != nil {
			return e
		}
		return os.WriteFile(p, []byte(text), 0o644)
	}
	for _, d := range skeleton {
		if !d.IsDir() || (d.Name() == "25-Topics" && !topics) {
			continue
		}
		if e := write(d.Name()+"/.gitkeep", ""); e != nil {
			return e
		}
	}
	if e := write("instance.yaml", instanceText); e != nil {
		return e
	}
	systems := []string{}
	for _, s := range items(inst["systems"]) {
		systems = append(systems, fmt.Sprintf("- [[%v]] (`%v`)", field(s, "name"), field(s, "id")))
	}
	home, e := template("00-Home.md.tmpl", locale, map[string]string{
		"cell_name": fmt.Sprint(field(inst["cell"], "name")), "cell_purpose": fmt.Sprint(field(inst["cell"], "purpose")),
		"systems_list": strings.Join(systems, "\n"), "pending_inventory": homeGuidance(locale)})
	if e != nil {
		return e
	}
	if e := write("00-Home.md", home); e != nil {
		return e
	}
	for _, s := range items(inst["systems"]) {
		aliases := items(field(s, "aliases"))
		aliasText := "[]"
		if len(aliases) > 0 {
			parts := []string{}
			for _, a := range aliases {
				parts = append(parts, fmt.Sprint(a))
			}
			aliasText = "[" + strings.Join(parts, ", ") + "]"
		}
		note, e := template("sistema.md.tmpl", locale, map[string]string{"system_id": fmt.Sprint(field(s, "id")),
			"system_name": fmt.Sprint(field(s, "name")), "cell_purpose": fmt.Sprint(field(inst["cell"], "purpose")), "aliases_yaml": aliasText})
		if e != nil {
			return e
		}
		if e := write("10-Sistemas/"+fmt.Sprint(field(s, "name"))+".md", note); e != nil {
			return e
		}
		if e := write("20-Repos/"+fmt.Sprint(field(s, "id"))+"/.gitkeep", ""); e != nil {
			return e
		}
	}
	for name, target := range map[string]string{"Flujos.md.tmpl": "30-Flujos/Flujos.md",
		"Operacion.md.tmpl": "60-Operacion/Operacion.md", "Aprendizajes.md.tmpl": "70-Aprendizajes/Aprendizajes.md"} {
		text, e := template(name, "en", nil)
		if e != nil {
			return e
		}
		if e := write(target, text); e != nil {
			return e
		}
	}
	return write(".gitignore", strings.Join(startingIgnores, "\n")+"\n")
}
