// Package runtimeenv checks declared tool dependencies without installing or
// executing commands supplied by a template.
package runtimeenv

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"csgclaw/internal/apitypes"
)

var tools = map[string][]string{"node": {"node", "--version"}, "python": {"python3", "--version"}, "npm": {"npm", "--version"}, "git": {"git", "--version"}, "bash": {"bash", "--version"}, "uv": {"uv", "--version"}}
var versionPattern = regexp.MustCompile(`\b(?:v)?([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
var minimumPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+(?:\.[0-9]+)?$`)

type Check struct {
	Name       string `json:"name"`
	Required   bool   `json:"required"`
	State      string `json:"state"`
	Version    string `json:"version,omitempty"`
	MinVersion string `json:"min_version,omitempty"`
}
type Report struct {
	Ready     bool      `json:"ready"`
	Scope     string    `json:"scope"`
	CheckedAt time.Time `json:"checked_at"`
	Checks    []Check   `json:"checks"`
}
type Probe func(context.Context, string) (string, error)

func Validate(deps []apitypes.RuntimeDependency) error {
	seen := map[string]bool{}
	for _, dep := range deps {
		if _, ok := tools[dep.Name]; !ok {
			return fmt.Errorf("unsupported dependency %q", dep.Name)
		}
		if seen[dep.Name] {
			return fmt.Errorf("duplicate dependency %q", dep.Name)
		}
		seen[dep.Name] = true
		if dep.MinVersion != "" && !minimumPattern.MatchString(dep.MinVersion) {
			return fmt.Errorf("dependency %q has invalid min_version", dep.Name)
		}
	}
	return nil
}
func ProbeTool(ctx context.Context, name string) (string, error) {
	args, ok := tools[name]
	if !ok {
		return "", fmt.Errorf("unsupported dependency")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, args[0], args[1:]...).Output()
	if err != nil {
		return "", fmt.Errorf("dependency is unavailable")
	}
	match := versionPattern.FindStringSubmatch(string(output))
	if len(match) < 2 {
		return "", fmt.Errorf("dependency version is unavailable")
	}
	return match[1], nil
}
func AtLeast(actual, minimum string) bool {
	a, b := strings.Split(strings.TrimPrefix(actual, "v"), "."), strings.Split(minimum, ".")
	for i := 0; i < 3; i++ {
		var x, y int
		var err error
		if i < len(a) {
			x, err = strconv.Atoi(a[i])
			if err != nil {
				return false
			}
		}
		if i < len(b) {
			y, err = strconv.Atoi(b[i])
			if err != nil {
				return false
			}
		}
		if x != y {
			return x > y
		}
	}
	return true
}
func Inspect(ctx context.Context, deps []apitypes.RuntimeDependency, probe Probe) Report {
	if probe == nil {
		probe = ProbeTool
	}
	report := Report{Ready: true, Scope: "service_host", CheckedAt: time.Now().UTC(), Checks: []Check{}}
	for _, dep := range deps {
		check := Check{Name: dep.Name, Required: !dep.Optional, MinVersion: dep.MinVersion, State: "ready"}
		if err := Validate([]apitypes.RuntimeDependency{dep}); err != nil {
			check.State = "unsupported"
		} else {
			version, err := probe(ctx, dep.Name)
			check.Version = version
			if err != nil {
				check.State = "missing"
			} else if dep.MinVersion != "" && !AtLeast(version, dep.MinVersion) {
				check.State = "version_mismatch"
			}
		}
		if check.Required && check.State != "ready" {
			report.Ready = false
		}
		report.Checks = append(report.Checks, check)
	}
	return report
}
