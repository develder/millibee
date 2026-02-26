#!/bin/sh
# Entrypoint for millibee Docker container.
# Syncs built-in skills to the workspace on first run,
# so bind mounts don't hide embedded skills.

WORKSPACE="$HOME/.millibee/workspace"
SKILLS_SRC="$HOME/.millibee/_builtin_skills"
SKILLS_DST="$WORKSPACE/skills"

# If built-in skills source exists (copied during image build),
# sync any missing skills into the mounted workspace volume.
if [ -d "$SKILLS_SRC" ]; then
    mkdir -p "$SKILLS_DST"
    for skill_dir in "$SKILLS_SRC"/*/; do
        skill_name="$(basename "$skill_dir")"
        if [ ! -d "$SKILLS_DST/$skill_name" ]; then
            cp -r "$skill_dir" "$SKILLS_DST/$skill_name"
            echo "Installed built-in skill: $skill_name"
        else
            # Always update SKILL.md from built-in to pick up metadata fixes
            if [ -f "$skill_dir/SKILL.md" ]; then
                cp "$skill_dir/SKILL.md" "$SKILLS_DST/$skill_name/SKILL.md"
            fi
        fi
    done
fi

# Sync bootstrap files (SOUL.md, USER.md, etc.) into workspace.
# Only copy if the file does not already exist (user customizations win).
BOOTSTRAP_SRC="$HOME/.millibee/_builtin_bootstrap"
if [ -d "$BOOTSTRAP_SRC" ]; then
    for f in "$BOOTSTRAP_SRC"/*.md; do
        [ -f "$f" ] || continue
        fname="$(basename "$f")"
        if [ ! -f "$WORKSPACE/$fname" ]; then
            cp "$f" "$WORKSPACE/$fname"
            echo "Installed bootstrap file: $fname"
        fi
    done
fi

# Ensure memory directory exists
mkdir -p "$WORKSPACE/memory"

exec millibee "$@"
