package bootstrap

import (
	"agentgo/internal/toolkit/extended/skill"
	"agentgo/internal/toolkit/core"
	"agentgo/internal/toolkit/registry"
)

// RegisterAllTools registers all core tools at startup.
// loadedSkills is the full skill map loaded from skills/ and project-skills/.
// skill is registered as an immediate tool; no activation required.
func RegisterAllTools(r *registry.ToolRegistry, loadedSkills *skill.SkillIndex) error {
	ranker := skill.NewSkillRanker(loadedSkills)
	if err := r.Register(skill.NewSkillTool(loadedSkills, ranker)); err != nil {
		return err
	}

	if err := r.Register(core.NewTodoWriteTool()); err != nil {
		return err
	}

	if err := r.Register(core.NewReadFileTool()); err != nil {
		return err
	}
	if err := r.Register(core.NewWriteFileTool()); err != nil {
		return err
	}
	if err := r.Register(core.NewEditFileTool()); err != nil {
		return err
	}
	if err := r.Register(core.NewListFilesTool()); err != nil {
		return err
	}
	if err := r.Register(core.NewGrepSearchTool()); err != nil {
		return err
	}
	if err := r.Register(core.NewWebFetchTool()); err != nil {
		return err
	}
	if err := r.Register(core.NewToolSearchTool(r)); err != nil {
		return err
	}
	return nil
}
