package tools

import "strings"

// Tool group constants.
const (
	GroupCore     = "core"
	GroupGit      = "git"
	GroupMemory   = "memory"
	GroupWeb      = "web"
	GroupSidecars = "sidecars"
	GroupHardware = "hardware"
	GroupComm     = "comm"
	GroupSkills   = "skills"
	GroupSpawn    = "spawn"
)

// explicitGroups maps tool names that don't follow prefix convention.
var explicitGroups = map[string]string{
	"exec":                GroupCore,
	"glob":                GroupCore,
	"grep":                GroupCore,
	"read_file":           GroupCore,
	"write_file":          GroupCore,
	"list_dir":            GroupCore,
	"edit_file":           GroupCore,
	"append_file":         GroupCore,
	"deep_scrape":         GroupSidecars,
	"youtube_transcript":  GroupSidecars,
	"transcribe_audio":    GroupSidecars,
	"message":             GroupComm,
	"spawn":               GroupSpawn,
	"find_skills":         GroupSkills,
	"install_skill":       GroupSkills,
	"i2c":                 GroupHardware,
	"spi":                 GroupHardware,
}

// prefixGroups maps name prefixes to groups.
var prefixGroups = map[string]string{
	"git_":    GroupGit,
	"memory_": GroupMemory,
	"web_":    GroupWeb,
}

// groupKeywords maps groups to activation keywords (lowercase).
var groupKeywords = map[string][]string{
	GroupGit:      {"git", "commit", "branch", "diff", "push", "pull", "merge", "stash"},
	GroupWeb:      {"http", "url", "web", "search", "fetch", "website", "link"},
	GroupSidecars: {"scrape", "crawl", "youtube", "transcript", "audio", "whisper"},
	GroupHardware: {"i2c", "spi", "sensor", "hardware", "gpio"},
	GroupComm:     {"message", "send", "notify", "telegram"},
	GroupSkills:   {"skill", "plugin"},
	GroupSpawn:    {"spawn", "subagent", "parallel"},
}

// ToolGroup returns the group for a tool by name.
// Checks explicit map first, then prefix, defaults to "core".
func ToolGroup(name string) string {
	if g, ok := explicitGroups[name]; ok {
		return g
	}
	for prefix, group := range prefixGroups {
		if strings.HasPrefix(name, prefix) {
			return group
		}
	}
	return GroupCore
}

// SelectActiveGroups determines which tool groups should be active
// based on the user message and previously used tools.
func SelectActiveGroups(lastUserMsg string, usedTools []string) map[string]bool {
	active := map[string]bool{
		GroupCore:   true,
		GroupMemory: true,
	}

	// Scan message for keywords
	lower := strings.ToLower(lastUserMsg)
	for group, keywords := range groupKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				active[group] = true
				break
			}
		}
	}

	// Sticky: activate groups of previously used tools
	for _, toolName := range usedTools {
		active[ToolGroup(toolName)] = true
	}

	return active
}
