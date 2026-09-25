package discover

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// snapshot reads one repository at one exact commit through Git objects, never the working tree.
type snapshot struct {
	repo   string
	commit string
	files  []string
	cmd    *exec.Cmd
	in     io.WriteCloser
	out    *bufio.Reader
}

func execGit(repo string, args ...string) *exec.Cmd {
	return exec.Command("git", append([]string{"-c", "core.quotePath=false", "-C", repo}, args...)...)
}

func gitOutput(repo string, args ...string) (string, error) {
	cmd := execGit(repo, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, e := cmd.Output()
	if e != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(b)), nil
}

// resolveCommit accepts a SHA or ref and returns the full commit id.
func resolveCommit(repo, ref string) (string, error) {
	return gitOutput(repo, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
}

// referenceRef applies the vault's reference-branch policy (90-Meta/reference-branches.md): a
// configured branch is exact; otherwise the cell's branch order (default main, then master). The
// remote-tracking ref wins over the local branch, which may lag behind it. A non-empty note explains a
// fallback the caller reports as pending.
func referenceRef(repo, configured string, order []string) (ref, note string, err error) {
	candidates := order
	if len(candidates) == 0 {
		candidates = []string{"main", "master"}
	}
	if configured != "" {
		candidates = []string{configured}
	}
	for _, b := range candidates {
		for _, r := range []string{"refs/remotes/origin/" + b, "refs/heads/" + b} {
			if _, e := resolveCommit(repo, r); e == nil {
				return r, "", nil
			}
		}
	}
	if configured != "" {
		return "", "", fmt.Errorf("reference branch %s is absent; fetch it or correct sources.reference_branches", configured)
	}
	fallback := "HEAD"
	if r, e := gitOutput(repo, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); e == nil && r != "" {
		fallback = r
	}
	return fallback, "none of " + strings.Join(candidates, ", ") + " exists; scanned " + fallback + "; declare the branch in sources.reference_branches or extend sources.reference_branch_order", nil
}

func openSnapshot(repo, ref string) (*snapshot, error) {
	commit, e := resolveCommit(repo, ref)
	if e != nil || commit == "" {
		return nil, fmt.Errorf("commit %s is not available in %s", ref, repo)
	}
	list, e := gitOutput(repo, "ls-tree", "-r", "--name-only", "-z", commit)
	if e != nil {
		return nil, e
	}
	files := []string{}
	for _, f := range strings.Split(list, "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	cmd := exec.Command("git", "-C", repo, "cat-file", "--batch")
	in, e := cmd.StdinPipe()
	if e != nil {
		return nil, e
	}
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		return nil, e
	}
	if e := cmd.Start(); e != nil {
		return nil, e
	}
	return &snapshot{repo: repo, commit: commit, files: files, cmd: cmd, in: in, out: bufio.NewReaderSize(stdout, 1<<16)}, nil
}

// read returns file bytes at the snapshot commit; missing objects return nil.
func (s *snapshot) read(path string) ([]byte, error) {
	if _, e := fmt.Fprintf(s.in, "%s:%s\n", s.commit, path); e != nil {
		return nil, e
	}
	header, e := s.out.ReadString('\n')
	if e != nil {
		return nil, e
	}
	fields := strings.Fields(header)
	if len(fields) < 3 || fields[1] == "missing" {
		return nil, nil
	}
	size, e := strconv.Atoi(fields[2])
	if e != nil {
		return nil, e
	}
	buf := make([]byte, size)
	if _, e := io.ReadFull(s.out, buf); e != nil {
		return nil, e
	}
	_, e = s.out.ReadByte()
	return buf, e
}

func (s *snapshot) close() {
	if s.in != nil {
		_ = s.in.Close()
	}
	if s.cmd != nil {
		_ = s.cmd.Wait()
	}
}
