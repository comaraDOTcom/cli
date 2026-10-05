package main

import (
	"testing"

	"github.com/exploreomni/omni-cli/internal/config"
	"github.com/exploreomni/omni-cli/internal/output"
)

func bannerFields(info output.BannerInfo) map[string]string {
	fields := map[string]string{}
	for _, f := range info.Fields {
		fields[f.Label] = f.Value
	}
	return fields
}

func TestBannerInfo(t *testing.T) {
	acme := &config.Config{
		DefaultProfile: "acme",
		Profiles: map[string]config.Profile{
			"acme":  {APIEndpoint: "https://acme.omniapp.co", AuthMethod: "api-key", APIKey: "secret-key"},
			"oauth": {APIEndpoint: "https://oauth.omniapp.co", AuthMethod: "oauth", AccessToken: "secret-token"},
			"empty": {APIEndpoint: "https://empty.omniapp.co", AuthMethod: "oauth"},
		},
	}
	withDefault := func(name string) *config.Config {
		cfg := *acme
		cfg.DefaultProfile = name
		return &cfg
	}
	const agentHint, initHint = "AI agents: start with omni agent-help", "Run omni config init to get started"

	for _, tc := range []struct {
		name                          string
		cfg                           *config.Config
		env                           map[string]string
		profile, instance, auth, hint string
	}{
		{"no config", nil, nil, "none", "not configured", "none", initHint},
		{"api key profile", acme, nil, "acme", "acme.omniapp.co", "api-key", agentHint},
		{"oauth profile", withDefault("oauth"), nil, "oauth", "oauth.omniapp.co", "oauth", agentHint},
		{"profile not logged in", withDefault("empty"), nil, "empty", "empty.omniapp.co", "none", initHint},
		{"default profile missing", withDefault("gone"), nil, "gone", "not configured", "none", initHint},
		{"environment only", nil, map[string]string{"OMNI_BASE_URL": "https://env.omniapp.co", "OMNI_API_TOKEN": "t"},
			"none", "env.omniapp.co", "OMNI_API_TOKEN", agentHint},
		{"environment over profile", acme, map[string]string{"OMNI_BASE_URL": "https://env.omniapp.co", "OMNI_API_TOKEN": "t"},
			"acme", "env.omniapp.co", "OMNI_API_TOKEN", agentHint},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := bannerInfo("1.4.0", tc.cfg, getenv(tc.env), "/work")
			fields := bannerFields(info)
			if fields["Profile"] != tc.profile || fields["Instance"] != tc.instance || fields["Auth"] != tc.auth {
				t.Errorf("fields = %v, want profile %q, instance %q, auth %q", fields, tc.profile, tc.instance, tc.auth)
			}
			if info.Hint != tc.hint {
				t.Errorf("hint = %q, want %q", info.Hint, tc.hint)
			}
			if info.Version != "v1.4.0" || info.Dir != "/work" {
				t.Errorf("version %q and dir %q should pass through", info.Version, info.Dir)
			}
		})
	}
}

// The banner never shows a secret, whichever way the profile authenticates.
func TestBannerInfo_KeepsSecretsOut(t *testing.T) {
	cfg := &config.Config{
		DefaultProfile: "acme",
		Profiles: map[string]config.Profile{
			"acme": {APIEndpoint: "https://acme.omniapp.co", AuthMethod: "api-key", APIKey: "secret-key"},
		},
	}
	info := bannerInfo("1.4.0", cfg, getenv(map[string]string{"OMNI_API_TOKEN": "secret-env-token"}), "/work")
	for _, f := range info.Fields {
		if f.Value == "secret-key" || f.Value == "secret-env-token" {
			t.Errorf("%s shows a secret", f.Label)
		}
	}
}

func TestBannerVersion(t *testing.T) {
	for in, want := range map[string]string{
		"1.4.0":        "v1.4.0",
		"v1.4.0":       "v1.4.0",
		"1.5.0-beta.1": "v1.5.0-beta.1",
		"dev":          "dev",
		"(devel)":      "(devel)",
	} {
		if got := bannerVersion(in); got != want {
			t.Errorf("bannerVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

// The banner is for a person who typed `omni` and nothing else. Agents and
// scripts must never find it in front of the output they asked for.
func TestBannerAllowed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		isTTY  bool
		format string
		env    map[string]string
		want   bool
	}{
		{"bare omni at a terminal", nil, true, config.FormatHuman, nil, true},
		{"piped or redirected", nil, false, config.FormatHuman, nil, false},
		{"json output format", nil, true, config.FormatJSON, nil, false},
		{"a subcommand", []string{"agent-help"}, true, config.FormatHuman, nil, false},
		{"a flag", []string{"--help"}, true, config.FormatHuman, nil, false},
		{"opted out", nil, true, config.FormatHuman, map[string]string{"OMNI_NO_BANNER": "1"}, false},
		{"ci", nil, true, config.FormatHuman, map[string]string{"CI": "true"}, false},
		{"dumb terminal", nil, true, config.FormatHuman, map[string]string{"TERM": "dumb"}, false},
		{"claude code", nil, true, config.FormatHuman, map[string]string{"CLAUDECODE": "1"}, false},
		{"cursor agent", nil, true, config.FormatHuman, map[string]string{"CURSOR_AGENT": "1"}, false},
		{"codex", nil, true, config.FormatHuman, map[string]string{"CODEX_SANDBOX": "seatbelt"}, false},
		{"gemini cli", nil, true, config.FormatHuman, map[string]string{"GEMINI_CLI": "1"}, false},
		{"generic agent marker", nil, true, config.FormatHuman, map[string]string{"AI_AGENT": "some-agent"}, false},
		{"an ordinary terminal", nil, true, config.FormatHuman, map[string]string{"TERM": "xterm-256color"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := bannerAllowed(tc.args, tc.isTTY, tc.format, getenv(tc.env)); got != tc.want {
				t.Errorf("bannerAllowed = %v, want %v", got, tc.want)
			}
		})
	}
}
