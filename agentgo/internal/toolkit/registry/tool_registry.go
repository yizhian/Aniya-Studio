package registry

import (
	"fmt"
	"strings"
	"sync"

	"agentgo/internal/model"
	"agentgo/internal/toolkit/contracts"
)

// DeferredToolLoader 用于延迟加载工具实现。
type DeferredToolLoader func() (contracts.Tool, error)

type registryItem struct {
	tool       contracts.Tool
	loader     DeferredToolLoader
	isDeferred bool
	activated  bool
}

// ToolRegistry 全局静态工具池（启动期一次性装载定义）。
type ToolRegistry struct {
	mu                 sync.RWMutex
	items              map[string]registryItem
	canonicalImmediate []string
	canonicalDeferred  []string
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{items: make(map[string]registryItem)}
}

func (r *ToolRegistry) Register(tool contracts.Tool) error {
	desc := tool.Descriptor()
	name := strings.TrimSpace(desc.Name)
	if name == "" {
		return fmt.Errorf("tool registry: empty tool name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[name]; ok {
		return fmt.Errorf("tool registry: duplicated name %q", name)
	}
	item := registryItem{tool: tool, isDeferred: false, activated: true}
	r.items[name] = item
	for _, alias := range desc.Aliases {
		a := strings.TrimSpace(alias)
		if a == "" {
			continue
		}
		if _, ok := r.items[a]; ok {
			return fmt.Errorf("tool registry: duplicated alias %q", a)
		}
		r.items[a] = item
	}
	r.canonicalImmediate = append(r.canonicalImmediate, name)
	return nil
}

func (r *ToolRegistry) RegisterDeferred(name string, loader DeferredToolLoader) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return fmt.Errorf("tool registry: empty deferred name")
	}
	if loader == nil {
		return fmt.Errorf("tool registry: nil loader for deferred %q", n)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[n]; ok {
		return fmt.Errorf("tool registry: duplicated deferred name %q", n)
	}
	r.items[n] = registryItem{
		loader:     loader,
		isDeferred: true,
		activated:  false,
	}
	r.canonicalDeferred = append(r.canonicalDeferred, n)
	return nil
}

// ActivateDeferred 将延迟工具标记为已激活并加载实现（供 tool_search 调用）。
func (r *ToolRegistry) ActivateDeferred(name string) error {
	n := strings.TrimSpace(name)
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[n]
	if !ok {
		return fmt.Errorf("tool registry: unknown deferred tool %q", n)
	}
	if !item.isDeferred {
		return fmt.Errorf("tool registry: %q is not a deferred tool", n)
	}
	if item.activated && item.tool != nil {
		return nil
	}
	if item.loader == nil {
		return fmt.Errorf("tool registry: deferred tool %q has no loader", n)
	}
	loaded, err := item.loader()
	if err != nil {
		return err
	}
	item.tool = loaded
	item.activated = true
	item.loader = nil
	r.items[n] = item
	return nil
}

// IsDeferredActivated 返回延迟工具是否已激活（已加载实现）。
func (r *ToolRegistry) IsDeferredActivated(name string) bool {
	n := strings.TrimSpace(name)
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[n]
	if !ok || !item.isDeferred {
		return false
	}
	return item.activated && item.tool != nil
}

// GetDeferredToolNames 返回尚未激活的延迟工具名（供 system prompt 列举）。
func (r *ToolRegistry) GetDeferredToolNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.canonicalDeferred))
	for _, n := range r.canonicalDeferred {
		item := r.items[n]
		if item.isDeferred && !item.activated {
			out = append(out, n)
		}
	}
	return out
}

// GetActiveToolDefinitions 生成发给模型 API 的 tools 列表：已就绪工具为完整 schema；未激活的延迟工具仅名称占位。
func (r *ToolRegistry) GetActiveToolDefinitions() []model.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.ToolDefinition, 0, len(r.canonicalImmediate)+len(r.canonicalDeferred))

	for _, name := range r.canonicalImmediate {
		item := r.items[name]
		if item.tool == nil {
			continue
		}
		out = append(out, toolDefinitionFromContract(item.tool.Descriptor()))
	}

	for _, name := range r.canonicalDeferred {
		item := r.items[name]
		if !item.isDeferred {
			continue
		}
		if item.activated && item.tool != nil {
			out = append(out, toolDefinitionFromContract(item.tool.Descriptor()))
			continue
		}
		out = append(out, deferredPlaceholderDefinition(name))
	}
	return out
}

// GetToolFlags returns the behavior flags for a registered tool by name.
// Returns zero-value flags and an error if the tool is not found.
func (r *ToolRegistry) GetToolFlags(name string) (contracts.ToolBehaviorFlags, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.items[name]
	if !ok {
		return contracts.ToolBehaviorFlags{}, fmt.Errorf("tool %q not found", name)
	}
	if item.tool == nil {
		return contracts.ToolBehaviorFlags{}, fmt.Errorf("tool %q not loaded", name)
	}
	return item.tool.Descriptor().Flags, nil
}

func deferredPlaceholderDefinition(name string) model.ToolDefinition {
	return model.ToolDefinition{
		Type: "function",
		Function: model.FunctionSpec{
			Name:        name,
			Description: "延迟工具：当前仅注册名称。请调用 tool_search 并传入该工具名以加载完整参数定义后再调用。",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

func toolDefinitionFromContract(d contracts.ToolDescriptor) model.ToolDefinition {
	params := d.InputJSONSchema
	if len(params) == 0 {
		params = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}
	return model.ToolDefinition{
		Type: "function",
		Function: model.FunctionSpec{
			Name:        d.Name,
			Description: d.Description,
			Parameters:  params,
		},
	}
}

// GetActiveToolPrompts collects the Prompt field from every active tool descriptor.
// Deferred tools that haven't been activated are skipped.
// The result is formatted for injection into a system prompt's tool section.
func (r *ToolRegistry) GetActiveToolPrompts() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	type entry struct {
		name   string
		prompt string
	}
	var entries []entry

	for _, name := range r.canonicalImmediate {
		item := r.items[name]
		if item.tool == nil {
			continue
		}
		desc := item.tool.Descriptor()
		if desc.Prompt != "" {
			entries = append(entries, entry{name: desc.Name, prompt: desc.Prompt})
		}
	}
	for _, name := range r.canonicalDeferred {
		item := r.items[name]
		if !item.activated || item.tool == nil {
			continue
		}
		desc := item.tool.Descriptor()
		if desc.Prompt != "" {
			entries = append(entries, entry{name: desc.Name, prompt: desc.Prompt})
		}
	}

	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "- %s: %s\n", e.name, e.prompt)
	}
	return b.String()
}

func (r *ToolRegistry) Resolve(name string) (contracts.Tool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[name]
	if !ok {
		return nil, fmt.Errorf("tool registry: unknown tool %q", name)
	}
	if item.isDeferred && (!item.activated || item.tool == nil) {
		return nil, fmt.Errorf("tool registry: deferred tool %q is not activated; use tool_search first", name)
	}
	if item.tool != nil {
		return item.tool, nil
	}
	if item.loader == nil {
		return nil, fmt.Errorf("tool registry: tool %q has no implementation", name)
	}
	loaded, err := item.loader()
	if err != nil {
		return nil, err
	}
	item.tool = loaded
	item.loader = nil
	r.items[name] = item
	return loaded, nil
}
