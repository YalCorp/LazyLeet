package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/YalCorp/LazyLeet/internal/catalog"
	"github.com/YalCorp/LazyLeet/internal/config"
)

var (
	doctorLookPath = exec.LookPath
	doctorOutput   = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).CombinedOutput()
	}
)

type doctorCheck struct {
	Name   string
	Status string
	Detail string
}

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local LazyLeet setup and language runtimes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.OutOrStdout())
		},
	}
}

func runDoctor(out io.Writer) error {
	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "LazyLeet doctor")
	fmt.Fprintln(out)
	writeDoctorSection(out, "Runtimes", []doctorCheck{
		runtimeCheck("Java", "javac", "javac", "-version"),
		runtimeCheck("Java VM", "java", "java", "-version"),
		runtimeCheck("Python", "python3", "python3", "--version"),
		runtimeCheck("Go", "go", "go", "version"),
	})
	fmt.Fprintln(out)
	writeDoctorSection(out, "Local data", localDataChecks(paths))
	return nil
}

func runtimeCheck(label, executable, versionCommand string, versionArgs ...string) doctorCheck {
	path, err := doctorLookPath(executable)
	if err != nil {
		return doctorCheck{Name: label, Status: "missing", Detail: executable + " not found"}
	}
	output, err := doctorOutput(versionCommand, versionArgs...)
	version := firstNonEmptyLine(string(output))
	if err != nil {
		return doctorCheck{Name: label, Status: "error", Detail: strings.TrimSpace(err.Error() + " " + version)}
	}
	if version == "" {
		version = path
	}
	return doctorCheck{Name: label, Status: "ok", Detail: version}
}

func localDataChecks(paths config.Paths) []doctorCheck {
	checks := []doctorCheck{
		pathCheck("App dir", paths.AppDir, true),
		pathCheck("Workspace", paths.WorkspaceDir, true),
		pathCheck("Packs dir", paths.PacksDir, true),
		pathCheck("Database", paths.DBPath, false),
	}
	packs, err := catalog.DiscoverDataPacks(paths.PacksDir, ".local")
	if err != nil {
		checks = append(checks, doctorCheck{Name: "Data packs", Status: "error", Detail: err.Error()})
		return checks
	}
	switch len(packs) {
	case 0:
		checks = append(checks, doctorCheck{Name: "Data packs", Status: "missing", Detail: "no packs discovered"})
	case 1:
		checks = append(checks, doctorCheck{Name: "Data packs", Status: "ok", Detail: "1 pack: " + packs[0].Slug})
	default:
		checks = append(checks, doctorCheck{Name: "Data packs", Status: "ok", Detail: fmt.Sprintf("%d packs", len(packs))})
	}
	return checks
}

func pathCheck(name, path string, wantDir bool) doctorCheck {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		status := "missing"
		if !wantDir {
			status = "pending"
		}
		return doctorCheck{Name: name, Status: status, Detail: path}
	}
	if err != nil {
		return doctorCheck{Name: name, Status: "error", Detail: err.Error()}
	}
	if wantDir && !info.IsDir() {
		return doctorCheck{Name: name, Status: "error", Detail: path + " is not a directory"}
	}
	if !wantDir && info.IsDir() {
		return doctorCheck{Name: name, Status: "error", Detail: path + " is a directory"}
	}
	if wantDir && !isWritableDir(path) {
		return doctorCheck{Name: name, Status: "error", Detail: path + " is not writable"}
	}
	return doctorCheck{Name: name, Status: "ok", Detail: path}
}

func isWritableDir(path string) bool {
	file, err := os.CreateTemp(path, ".lazyleet-doctor-*")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return filepath.Dir(name) == path
}

func writeDoctorSection(out io.Writer, title string, checks []doctorCheck) {
	fmt.Fprintln(out, title)
	for _, check := range checks {
		fmt.Fprintf(out, "  %-10s %-7s %s\n", check.Name, check.Status, check.Detail)
	}
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
