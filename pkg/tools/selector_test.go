package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- ToolGroup tests ---

func TestToolGroup_Prefix(t *testing.T) {
	assert.Equal(t, "git", ToolGroup("git_status"))
	assert.Equal(t, "git", ToolGroup("git_commit"))
	assert.Equal(t, "git", ToolGroup("git_push"))
	assert.Equal(t, "memory", ToolGroup("memory_save"))
	assert.Equal(t, "memory", ToolGroup("memory_search"))
	assert.Equal(t, "web", ToolGroup("web_search"))
	assert.Equal(t, "web", ToolGroup("web_fetch"))
}

func TestToolGroup_Explicit(t *testing.T) {
	assert.Equal(t, "core", ToolGroup("exec"))
	assert.Equal(t, "core", ToolGroup("glob"))
	assert.Equal(t, "core", ToolGroup("grep"))
	assert.Equal(t, "sidecars", ToolGroup("deep_scrape"))
	assert.Equal(t, "sidecars", ToolGroup("youtube_transcript"))
	assert.Equal(t, "sidecars", ToolGroup("transcribe_audio"))
	assert.Equal(t, "comm", ToolGroup("message"))
	assert.Equal(t, "spawn", ToolGroup("spawn"))
	assert.Equal(t, "skills", ToolGroup("find_skills"))
	assert.Equal(t, "skills", ToolGroup("install_skill"))
	assert.Equal(t, "hardware", ToolGroup("i2c"))
	assert.Equal(t, "hardware", ToolGroup("spi"))
}

func TestToolGroup_CoreFilesystem(t *testing.T) {
	assert.Equal(t, "core", ToolGroup("read_file"))
	assert.Equal(t, "core", ToolGroup("write_file"))
	assert.Equal(t, "core", ToolGroup("list_dir"))
	assert.Equal(t, "core", ToolGroup("edit_file"))
	assert.Equal(t, "core", ToolGroup("append_file"))
}

func TestToolGroup_DefaultCore(t *testing.T) {
	assert.Equal(t, "core", ToolGroup("unknown_tool"))
	assert.Equal(t, "core", ToolGroup("something_new"))
}

// --- SelectActiveGroups tests ---

func TestSelectActiveGroups_AlwaysCoreAndMemory(t *testing.T) {
	groups := SelectActiveGroups("hoe laat is het", nil)
	assert.True(t, groups["core"])
	assert.True(t, groups["memory"])
}

func TestSelectActiveGroups_GitKeywords(t *testing.T) {
	groups := SelectActiveGroups("commit deze wijzigingen", nil)
	assert.True(t, groups["git"])

	groups = SelectActiveGroups("push naar origin", nil)
	assert.True(t, groups["git"])

	groups = SelectActiveGroups("bekijk de git log", nil)
	assert.True(t, groups["git"])
}

func TestSelectActiveGroups_WebKeywords(t *testing.T) {
	groups := SelectActiveGroups("fetch https://example.com", nil)
	assert.True(t, groups["web"])

	groups = SelectActiveGroups("zoek op het web naar Go tutorials", nil)
	assert.True(t, groups["web"])
}

func TestSelectActiveGroups_SidecarKeywords(t *testing.T) {
	groups := SelectActiveGroups("scrape die website", nil)
	assert.True(t, groups["sidecars"])

	groups = SelectActiveGroups("youtube video samenvatten", nil)
	assert.True(t, groups["sidecars"])
}

func TestSelectActiveGroups_StickyUsedTools(t *testing.T) {
	groups := SelectActiveGroups("doe verder", []string{"git_status"})
	assert.True(t, groups["git"], "git should be sticky when git_status was used")

	groups = SelectActiveGroups("ok", []string{"web_fetch", "deep_scrape"})
	assert.True(t, groups["web"])
	assert.True(t, groups["sidecars"])
}

func TestSelectActiveGroups_NoMatch(t *testing.T) {
	groups := SelectActiveGroups("hoe laat is het", nil)
	assert.False(t, groups["git"])
	assert.False(t, groups["web"])
	assert.False(t, groups["sidecars"])
	assert.False(t, groups["hardware"])
	assert.False(t, groups["comm"])
	assert.False(t, groups["skills"])
	assert.False(t, groups["spawn"])
	// Only core + memory
	assert.Equal(t, 2, len(groups))
}

func TestSelectActiveGroups_MultipleGroups(t *testing.T) {
	groups := SelectActiveGroups("git push en fetch https://example.com", nil)
	assert.True(t, groups["git"])
	assert.True(t, groups["web"])
	assert.True(t, groups["core"])
	assert.True(t, groups["memory"])
}
