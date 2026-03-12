package tome_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/tome"
)

func TestInstallHooksCreatesNew(t *testing.T) {
	repoDir := t.TempDir()
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	require.NoError(t, tome.InstallHooks(repoDir))

	// Verify post-commit hook.
	postCommit, err := os.ReadFile(filepath.Join(hooksDir, "post-commit"))
	require.NoError(t, err)
	assert.Contains(t, string(postCommit), "# managed by tome")
	assert.Contains(t, string(postCommit), "tome checkpoint")

	// Verify pre-push hook.
	prePush, err := os.ReadFile(filepath.Join(hooksDir, "pre-push"))
	require.NoError(t, err)
	assert.Contains(t, string(prePush), "# managed by tome")
	assert.Contains(t, string(prePush), "tome sync --push")

	// Verify executable permissions.
	info, err := os.Stat(filepath.Join(hooksDir, "post-commit"))
	require.NoError(t, err)
	assert.True(t, info.Mode()&0o100 != 0, "post-commit should be executable")

	info, err = os.Stat(filepath.Join(hooksDir, "pre-push"))
	require.NoError(t, err)
	assert.True(t, info.Mode()&0o100 != 0, "pre-push should be executable")
}

func TestInstallHooksIdempotent(t *testing.T) {
	repoDir := t.TempDir()
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	// Install once.
	require.NoError(t, tome.InstallHooks(repoDir))

	// Read the content.
	first, err := os.ReadFile(filepath.Join(hooksDir, "post-commit"))
	require.NoError(t, err)

	// Install again.
	require.NoError(t, tome.InstallHooks(repoDir))

	// Content should be unchanged.
	second, err := os.ReadFile(filepath.Join(hooksDir, "post-commit"))
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second))
}

func TestInstallHooksPreservesExisting(t *testing.T) {
	repoDir := t.TempDir()
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	// Create an existing hook.
	existing := "#!/bin/sh\necho 'existing hook'\n"
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte(existing), 0o755))

	require.NoError(t, tome.InstallHooks(repoDir))

	// Verify existing content is preserved.
	content, err := os.ReadFile(filepath.Join(hooksDir, "post-commit"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "existing hook")
	assert.Contains(t, string(content), "# managed by tome")
	assert.Contains(t, string(content), "tome checkpoint")
}

func TestInstallHooksNotGitRepo(t *testing.T) {
	repoDir := t.TempDir()
	// No .git/hooks directory

	err := tome.InstallHooks(repoDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestAddGitignoreCreatesNew(t *testing.T) {
	repoDir := t.TempDir()

	require.NoError(t, tome.AddGitignore(repoDir))

	content, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(content), ".tome/")
}

func TestAddGitignoreAppendsToExisting(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte("node_modules/\n"), 0o644))

	require.NoError(t, tome.AddGitignore(repoDir))

	content, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "node_modules/")
	assert.Contains(t, string(content), ".tome/")
}

func TestAddGitignoreIdempotent(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte(".tome/\n"), 0o644))

	require.NoError(t, tome.AddGitignore(repoDir))

	content, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, ".tome/\n", string(content))
}

func TestUninstallHooksRemovesTomeOnly(t *testing.T) {
	repoDir := t.TempDir()
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	// Install hooks, then uninstall.
	require.NoError(t, tome.InstallHooks(repoDir))
	require.NoError(t, tome.UninstallHooks(repoDir))

	// Hook files should be removed (they only had tome content).
	_, err := os.Stat(filepath.Join(hooksDir, "post-commit"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(hooksDir, "pre-push"))
	assert.True(t, os.IsNotExist(err))
}

func TestUninstallHooksPreservesOtherContent(t *testing.T) {
	repoDir := t.TempDir()
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	// Create an existing hook, then install tome on top.
	existing := "#!/bin/sh\necho 'existing hook'\n"
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte(existing), 0o755))
	require.NoError(t, tome.InstallHooks(repoDir))

	// Uninstall — existing content should remain.
	require.NoError(t, tome.UninstallHooks(repoDir))

	content, err := os.ReadFile(filepath.Join(hooksDir, "post-commit"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "existing hook")
	assert.NotContains(t, string(content), "managed by tome")
}

func TestUninstallHooksNoHooksDir(t *testing.T) {
	repoDir := t.TempDir()
	// No .git/hooks — should not error.
	require.NoError(t, tome.UninstallHooks(repoDir))
}

func TestRemoveGitignore(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte("node_modules/\n.tome/\n"), 0o644))

	require.NoError(t, tome.RemoveGitignore(repoDir))

	content, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "node_modules/")
	assert.NotContains(t, string(content), ".tome")
}

func TestRemoveGitignoreDeletesFileIfOnlyTome(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte(".tome/\n"), 0o644))

	require.NoError(t, tome.RemoveGitignore(repoDir))

	_, err := os.Stat(filepath.Join(repoDir, ".gitignore"))
	assert.True(t, os.IsNotExist(err))
}

func TestRemoveGitignoreNoFile(t *testing.T) {
	repoDir := t.TempDir()
	// No .gitignore — should not error.
	require.NoError(t, tome.RemoveGitignore(repoDir))
}

func TestInstallSkill(t *testing.T) {
	repoDir := t.TempDir()

	require.NoError(t, tome.InstallSkill(repoDir))

	skillPath := filepath.Join(repoDir, ".claude", "skills", "tome", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "name: tome")
	assert.Contains(t, string(content), "tome search")
	assert.Contains(t, string(content), "tome record")
}

func TestInstallSkillIdempotent(t *testing.T) {
	repoDir := t.TempDir()

	require.NoError(t, tome.InstallSkill(repoDir))
	first, err := os.ReadFile(filepath.Join(repoDir, ".claude", "skills", "tome", "SKILL.md"))
	require.NoError(t, err)

	require.NoError(t, tome.InstallSkill(repoDir))
	second, err := os.ReadFile(filepath.Join(repoDir, ".claude", "skills", "tome", "SKILL.md"))
	require.NoError(t, err)

	assert.Equal(t, string(first), string(second))
}

func TestRemoveSkill(t *testing.T) {
	repoDir := t.TempDir()

	require.NoError(t, tome.InstallSkill(repoDir))
	require.NoError(t, tome.RemoveSkill(repoDir))

	// Skill directory should be gone.
	_, err := os.Stat(filepath.Join(repoDir, ".claude", "skills", "tome"))
	assert.True(t, os.IsNotExist(err))

	// Empty parent dirs should be cleaned up.
	_, err = os.Stat(filepath.Join(repoDir, ".claude", "skills"))
	assert.True(t, os.IsNotExist(err))
}

func TestRemoveSkillNoSkill(t *testing.T) {
	repoDir := t.TempDir()
	// No skill installed — should not error.
	require.NoError(t, tome.RemoveSkill(repoDir))
}

func TestRemoveSkillPreservesOtherSkills(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, ".claude", "skills")

	// Install tome skill + another skill.
	require.NoError(t, tome.InstallSkill(repoDir))
	otherSkillDir := filepath.Join(skillsDir, "other")
	require.NoError(t, os.MkdirAll(otherSkillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(otherSkillDir, "SKILL.md"), []byte("other"), 0o644))

	require.NoError(t, tome.RemoveSkill(repoDir))

	// Tome skill should be gone.
	_, err := os.Stat(filepath.Join(skillsDir, "tome"))
	assert.True(t, os.IsNotExist(err))

	// Other skill and parent dirs should remain.
	_, err = os.Stat(filepath.Join(otherSkillDir, "SKILL.md"))
	assert.NoError(t, err)
}
