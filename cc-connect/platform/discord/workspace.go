package discord

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/chenhg5/cc-connect/core"
	"github.com/bwmarrin/discordgo"
)

type workspaceDef struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type workspaceState struct {
	Workspaces      map[string]workspaceDef   `json:"workspaces"`
	ChannelBindings map[string]string         `json:"channel_bindings"`
	GuildCategories map[string]map[string]string `json:"guild_categories"`
}

type workspaceManager struct {
	path  string
	roots []string

	mu    sync.RWMutex
	state workspaceState
}

func newWorkspaceManager(statePath string, roots []string) (*workspaceManager, error) {
	wm := &workspaceManager{
		path:  statePath,
		roots: normalizeRoots(roots),
		state: workspaceState{
			Workspaces:      map[string]workspaceDef{},
			ChannelBindings: map[string]string{},
			GuildCategories: map[string]map[string]string{},
		},
	}
	if err := wm.load(); err != nil {
		return nil, err
	}
	return wm, nil
}

func normalizeRoots(roots []string) []string {
	var out []string
	for _, r := range roots {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		abs, err := filepath.Abs(r)
		if err != nil {
			continue
		}
		out = append(out, strings.ToLower(filepath.Clean(abs)))
	}
	return out
}

func workspaceKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (wm *workspaceManager) load() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	data, err := os.ReadFile(wm.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("discord workspace: read state: %w", err)
	}
	var st workspaceState
	if err := json.Unmarshal(data, &st); err != nil {
		return fmt.Errorf("discord workspace: parse state: %w", err)
	}
	if st.Workspaces == nil {
		st.Workspaces = map[string]workspaceDef{}
	}
	if st.ChannelBindings == nil {
		st.ChannelBindings = map[string]string{}
	}
	if st.GuildCategories == nil {
		st.GuildCategories = map[string]map[string]string{}
	}
	wm.state = st
	return nil
}

func (wm *workspaceManager) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(wm.path), 0o755); err != nil {
		return fmt.Errorf("discord workspace: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(wm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("discord workspace: marshal state: %w", err)
	}
	if err := core.AtomicWriteFile(wm.path, data, 0o644); err != nil {
		return fmt.Errorf("discord workspace: write state: %w", err)
	}
	return nil
}

func (wm *workspaceManager) validateWorkspacePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	abs = filepath.Clean(abs)

	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("workspace path does not exist")
		}
		return "", fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace path is not a directory")
	}
	if !wm.allowedByRoots(abs) {
		return "", fmt.Errorf("path is outside allowed roots")
	}
	return abs, nil
}

func (wm *workspaceManager) allowedByRoots(abs string) bool {
	if len(wm.roots) == 0 {
		return true
	}
	absLower := strings.ToLower(abs)
	for _, root := range wm.roots {
		if absLower == root || strings.HasPrefix(absLower, root+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func (wm *workspaceManager) Create(name, path string) (workspaceDef, error) {
	path, err := wm.validateWorkspacePath(path)
	if err != nil {
		return workspaceDef{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(path)
	}
	key := workspaceKey(name)
	if key == "" {
		return workspaceDef{}, fmt.Errorf("workspace name is required")
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()
	if _, exists := wm.state.Workspaces[key]; exists {
		return workspaceDef{}, fmt.Errorf("workspace already exists: %s", name)
	}
	ws := workspaceDef{Name: name, Path: path}
	wm.state.Workspaces[key] = ws
	if err := wm.saveLocked(); err != nil {
		return workspaceDef{}, err
	}
	return ws, nil
}

func (wm *workspaceManager) Get(name string) (workspaceDef, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	ws, ok := wm.state.Workspaces[workspaceKey(name)]
	return ws, ok
}

func (wm *workspaceManager) List() []workspaceDef {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	items := make([]workspaceDef, 0, len(wm.state.Workspaces))
	for _, ws := range wm.state.Workspaces {
		items = append(items, ws)
	}
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items
}

func (wm *workspaceManager) BindChannel(channelID, workspaceName string) (workspaceDef, error) {
	ws, ok := wm.Get(workspaceName)
	if !ok {
		return workspaceDef{}, fmt.Errorf("workspace not found: %s", workspaceName)
	}
	key := workspaceKey(ws.Name)

	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.state.ChannelBindings[channelID] = key
	if err := wm.saveLocked(); err != nil {
		return workspaceDef{}, err
	}
	return ws, nil
}

func (wm *workspaceManager) UnbindChannel(channelID string) (bool, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if _, ok := wm.state.ChannelBindings[channelID]; !ok {
		return false, nil
	}
	delete(wm.state.ChannelBindings, channelID)
	if err := wm.saveLocked(); err != nil {
		return false, err
	}
	return true, nil
}

func (wm *workspaceManager) BoundWorkspace(channelID string) (workspaceDef, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	key, ok := wm.state.ChannelBindings[channelID]
	if !ok {
		return workspaceDef{}, false
	}
	ws, ok := wm.state.Workspaces[key]
	return ws, ok
}

func (wm *workspaceManager) SetGuildCategory(guildID, workspaceName, categoryID string) error {
	key := workspaceKey(workspaceName)

	wm.mu.Lock()
	defer wm.mu.Unlock()
	if wm.state.GuildCategories[guildID] == nil {
		wm.state.GuildCategories[guildID] = map[string]string{}
	}
	wm.state.GuildCategories[guildID][key] = categoryID
	return wm.saveLocked()
}

func (wm *workspaceManager) WorkspaceByCategory(guildID, categoryID string) (workspaceDef, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	byWorkspace := wm.state.GuildCategories[guildID]
	for key, catID := range byWorkspace {
		if catID == categoryID {
			ws, ok := wm.state.Workspaces[key]
			return ws, ok
		}
	}
	return workspaceDef{}, false
}

func splitArgs(input string) []string {
	var args []string
	var buf strings.Builder
	var quote rune
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		args = append(args, buf.String())
		buf.Reset()
	}
	for _, r := range strings.TrimSpace(input) {
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			buf.WriteRune(r)
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t', '\n', '\r':
			flush()
		default:
			buf.WriteRune(r)
		}
	}
	flush()
	return args
}

func (p *Platform) workspaceSession(channelID, userID, guildID string) (string, []string) {
	base := p.makeSessionKey(channelID, userID)
	if p.workspace == nil {
		return base, nil
	}
	ws, ok := p.resolveWorkspace(channelID, guildID)
	if !ok {
		return base, nil
	}
	sessionKey := fmt.Sprintf("%s:ws=%s", base, strings.ToLower(strings.ReplaceAll(ws.Name, " ", "_")))
	env := []string{
		"CC_WORKSPACE=" + ws.Name,
		"CC_WORK_DIR=" + ws.Path,
	}
	return sessionKey, env
}

func (p *Platform) resolveWorkspace(channelID, guildID string) (workspaceDef, bool) {
	if p.workspace == nil {
		return workspaceDef{}, false
	}
	if ws, ok := p.workspace.BoundWorkspace(channelID); ok {
		return ws, true
	}
	if guildID == "" || p.session == nil {
		return workspaceDef{}, false
	}

	var ch *discordgo.Channel
	var err error
	if p.session.State != nil {
		ch, err = p.session.State.Channel(channelID)
	}
	if err != nil || ch == nil {
		ch, err = p.session.Channel(channelID)
		if err != nil || ch == nil {
			return workspaceDef{}, false
		}
	}
	if ch.ParentID == "" {
		return workspaceDef{}, false
	}
	return p.workspace.WorkspaceByCategory(guildID, ch.ParentID)
}

func (p *Platform) ensureWorkspaceCategory(guildID string, ws workspaceDef) (string, error) {
	if p.workspace == nil {
		return "", fmt.Errorf("workspace manager is not enabled")
	}
	if guildID == "" {
		return "", fmt.Errorf("this command only works in a server channel")
	}
	channels, err := p.session.GuildChannels(guildID)
	if err != nil {
		return "", fmt.Errorf("list guild channels: %w", err)
	}
	for _, c := range channels {
		if c.Type == discordgo.ChannelTypeGuildCategory && strings.EqualFold(c.Name, ws.Name) {
			_ = p.workspace.SetGuildCategory(guildID, ws.Name, c.ID)
			return c.ID, nil
		}
	}
	created, err := p.session.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name: ws.Name,
		Type: discordgo.ChannelTypeGuildCategory,
	})
	if err != nil {
		return "", fmt.Errorf("create category: %w", err)
	}
	if err := p.workspace.SetGuildCategory(guildID, ws.Name, created.ID); err != nil {
		return "", err
	}
	return created.ID, nil
}

func (p *Platform) runWorkspaceCommand(channelID, guildID, raw string) string {
	if p.workspace == nil {
		return "Workspace management is disabled for this bot."
	}

	args := splitArgs(strings.TrimSpace(raw))
	if len(args) == 0 {
		return "Usage: /workspace <create|list|bind|unbind|current> ..."
	}

	switch strings.ToLower(args[0]) {
	case "create":
		if len(args) < 2 {
			return "Usage: /workspace create <path> OR /workspace create <name> <path>"
		}
		name := ""
		path := ""
		if len(args) == 2 {
			path = args[1]
		} else {
			name = args[1]
			path = strings.Join(args[2:], " ")
		}
		ws, err := p.workspace.Create(name, path)
		if err != nil {
			return "Workspace create failed: " + err.Error()
		}
		if guildID != "" {
			if _, err := p.ensureWorkspaceCategory(guildID, ws); err != nil {
				return fmt.Sprintf("Workspace created: %s (category setup failed: %v)", ws.Name, err)
			}
		}
		return fmt.Sprintf("Workspace created: %s", ws.Name)

	case "list":
		items := p.workspace.List()
		if len(items) == 0 {
			return "No workspaces configured. Use /workspace create ..."
		}
		lines := []string{"Workspaces:"}
		for i, ws := range items {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, ws.Name))
		}
		return strings.Join(lines, "\n")

	case "bind":
		if len(args) < 2 {
			return "Usage: /workspace bind <workspace-name>"
		}
		ws, err := p.workspace.BindChannel(channelID, strings.Join(args[1:], " "))
		if err != nil {
			return "Workspace bind failed: " + err.Error()
		}
		if guildID != "" {
			catID, err := p.ensureWorkspaceCategory(guildID, ws)
			if err != nil {
				return fmt.Sprintf("Bound to workspace %s, but failed to ensure category: %v", ws.Name, err)
			}
			_, err = p.session.ChannelEditComplex(channelID, &discordgo.ChannelEdit{ParentID: catID})
			if err != nil {
				return fmt.Sprintf("Bound to workspace %s, but failed to move channel: %v", ws.Name, err)
			}
		}
		return fmt.Sprintf("This channel is now bound to workspace: %s", ws.Name)

	case "unbind":
		changed, err := p.workspace.UnbindChannel(channelID)
		if err != nil {
			return "Workspace unbind failed: " + err.Error()
		}
		if !changed {
			return "This channel has no explicit workspace binding."
		}
		return "Workspace binding removed for this channel."

	case "current":
		ws, ok := p.resolveWorkspace(channelID, guildID)
		if !ok {
			return "No workspace is currently active for this channel."
		}
		return fmt.Sprintf("Current workspace: %s", ws.Name)
	}

	return "Unknown workspace command. Use: create, list, bind, unbind, current"
}
