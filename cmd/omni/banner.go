package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/exploreomni/omni-cli/internal/config"
	"github.com/exploreomni/omni-cli/internal/output"
	"github.com/exploreomni/omni-cli/internal/updatecheck"
	"golang.org/x/term"
)

// agentEnvVars are set by coding agents in the shells they run commands in.
// Some of them hand the command a pseudo-terminal, so a TTY alone doesn't
// prove a person is reading.
var agentEnvVars = []string{"AI_AGENT", "AGENT", "CLAUDECODE", "CODEX_SANDBOX", "CURSOR_AGENT", "GEMINI_CLI"}

// maybePrintBanner greets bare `omni` with the welcome banner, ahead of the
// root help cobra prints for it.
func maybePrintBanner(args []string, stdout, stderr *os.File, version string) {
	fd := int(stdout.Fd())
	isTTY := term.IsTerminal(fd) && term.IsTerminal(int(stderr.Fd()))
	if !bannerAllowed(args, isTTY, config.ResolveOutputFormat("", isTTY), os.Getenv) {
		return
	}
	// Leave the last column free: some terminals wrap a line that fills it.
	width, _, err := term.GetSize(fd)
	if err != nil || width-1 < output.BannerMinWidth {
		return
	}
	cfg, _ := config.Load()
	dir, _ := os.Getwd()
	output.Banner(stdout, bannerInfo(version, cfg, os.Getenv, dir), width-1)
	fmt.Fprintln(stdout)
}

// bannerAllowed reports whether this invocation is a person at a terminal
// running `omni` with nothing after it. Everything else goes without: any
// argument at all (so no command's output ever changes), piped or redirected
// output, a JSON output format, CI, a dumb terminal, a coding agent's shell,
// and anyone who set OMNI_NO_BANNER.
func bannerAllowed(args []string, isTTY bool, format string, getenv func(string) string) bool {
	if len(args) != 0 || !isTTY || format != config.FormatHuman {
		return false
	}
	if getenv("OMNI_NO_BANNER") != "" || getenv("CI") != "" || getenv("TERM") == "dumb" {
		return false
	}
	for _, name := range agentEnvVars {
		if getenv(name) != "" {
			return false
		}
	}
	return true
}

// bannerInfo describes the connection a command run right now would use. It
// reads what config.Resolve reads — default profile, then environment — but
// never refreshes a token: the banner must not touch the network.
func bannerInfo(version string, cfg *config.Config, getenv func(string) string, dir string) output.BannerInfo {
	const none, unconfigured = "none", "not configured"
	profile, instance, auth := none, unconfigured, none
	if cfg != nil && cfg.DefaultProfile != "" {
		profile = cfg.DefaultProfile
		if p, ok := cfg.Profiles[cfg.DefaultProfile]; ok {
			if p.APIEndpoint != "" {
				instance = p.APIEndpoint
			}
			switch {
			case p.AuthMethod == "oauth" && p.AccessToken != "":
				auth = "oauth"
			case p.AuthMethod != "oauth" && p.APIKey != "":
				auth = "api-key"
			}
		}
	}
	if v := getenv("OMNI_BASE_URL"); v != "" {
		instance = v
	}
	if getenv("OMNI_API_TOKEN") != "" {
		auth = "OMNI_API_TOKEN"
	}

	hint := "AI agents: start with omni agent-help"
	if instance == unconfigured || auth == none {
		hint = "Run omni config init to get started"
	}
	return output.BannerInfo{
		Version: bannerVersion(version),
		Fields: []output.BannerField{
			{Label: "Profile", Value: profile},
			{Label: "Instance", Value: strings.TrimPrefix(instance, "https://")},
			{Label: "Auth", Value: auth},
		},
		Hint: hint,
		Dir:  dir,
	}
}

// bannerVersion is the build's version as the banner shows it: v-prefixed
// for a release, and as-is ("dev") for a build that doesn't have one.
func bannerVersion(version string) string {
	if updatecheck.IsReleaseVersion(version) {
		return displayVersion(version)
	}
	return version
}
