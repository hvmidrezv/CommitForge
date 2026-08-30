// Package tui contains the bubbletea TUI model and views for CommitForge.
package tui

// binding groups the keys for one logical action and the hint text shown in the footer.
type binding struct {
	keys []string
	help string
}

func (b binding) label() string {
	if len(b.keys) == 0 {
		return ""
	}
	out := b.keys[0]
	for i := 1; i < len(b.keys); i++ {
		out += "/" + b.keys[i]
	}
	return out
}

// keyMap defines shared keybindings used across the TUI.
var keyMap = struct {
	Up, Down, Left, Right binding
	Select                binding
	RangeSelect           binding
	SelectAll             binding
	Clear                 binding
	ClearAll              binding
	PrevYear              binding
	NextYear              binding
	Confirm               binding
	Back                  binding
	Help                  binding
	Yes, No               binding
	Quit                  binding
	ForceQuit             binding
}{
	Up:          binding{keys: []string{"up", "k"}, help: "move up"},
	Down:        binding{keys: []string{"down", "j"}, help: "move down"},
	Left:        binding{keys: []string{"left", "h"}, help: "move left"},
	Right:       binding{keys: []string{"right", "l"}, help: "move right"},
	Select:      binding{keys: []string{" "}, help: "toggle selection for the focused day"},
	RangeSelect: binding{keys: []string{"v"}, help: "start or confirm chronological range selection"},
	SelectAll:   binding{keys: []string{"a"}, help: "select all visible days in the current window"},
	Clear:       binding{keys: []string{"u"}, help: "clear current selection"},
	ClearAll:    binding{keys: []string{"x"}, help: "clear all commits (history rewrite + force push)"},
	PrevYear:    binding{keys: []string{"[", "pgup"}, help: "shift displayed year window backward"},
	NextYear:    binding{keys: []string{"]", "pgdown"}, help: "shift displayed year window forward"},
	Confirm:     binding{keys: []string{"enter"}, help: "proceed with selected dates (counts/menu)"},
	Back:        binding{keys: []string{"esc", "backspace"}, help: "back"},
	Help:        binding{keys: []string{"?"}, help: "toggle help overlay"},
	Yes:         binding{keys: []string{"y"}, help: "yes"},
	No:          binding{keys: []string{"n"}, help: "no"},
	Quit:        binding{keys: []string{"q", "ctrl+c"}, help: "quit"},
	ForceQuit:   binding{keys: []string{"ctrl+c"}, help: "force quit"},
}

// activeBindings returns the set of keybindings relevant for the current screen.
func activeBindings(m Model) []binding {
	switch m.screen {
	case screenProjectSelect:
		return []binding{keyMap.Up, keyMap.Down, keyMap.Confirm, keyMap.Quit}
	case screenProjectCreateName:
		return []binding{
			{keys: []string{"a-z/0-9/space/-/_"}, help: "type project name"},
			{keys: []string{"backspace"}, help: "delete"},
			keyMap.Confirm, keyMap.Back, keyMap.ForceQuit,
		}
	case screenProjectRemoteMode:
		return []binding{keyMap.Up, keyMap.Down, keyMap.Confirm, keyMap.Back, keyMap.Quit}
	case screenProjectRemoteInput:
		return []binding{
			{keys: []string{"0-9/a-z/:._-@\\"}, help: "type remote URL"},
			{keys: []string{"backspace"}, help: "delete"},
			keyMap.Confirm, keyMap.Back, keyMap.ForceQuit,
		}
	case screenGrid:
		if m.rangeMode {
			return []binding{
				keyMap.Up, keyMap.Down, keyMap.Left, keyMap.Right,
				keyMap.RangeSelect, keyMap.Select, keyMap.PrevYear, keyMap.NextYear, keyMap.Back, keyMap.Help, keyMap.Quit,
			}
		}
		return []binding{
			keyMap.Up, keyMap.Down, keyMap.Left, keyMap.Right,
			keyMap.Select, keyMap.RangeSelect, keyMap.SelectAll, keyMap.Clear,
			keyMap.PrevYear, keyMap.NextYear, keyMap.Confirm, keyMap.Help, keyMap.Quit,
		}
	case screenCountEntry:
		return []binding{
			keyMap.Confirm, {keys: []string{"0-9/-"}, help: "type count or min-max"},
			{keys: []string{"backspace"}, help: "delete"},
			keyMap.Back, keyMap.Help, keyMap.ForceQuit,
		}
	case screenOptions:
		return []binding{keyMap.Up, keyMap.Down, keyMap.Confirm, keyMap.ClearAll, keyMap.Select, keyMap.Back, keyMap.Help, keyMap.Quit}
	case screenDeselectConfirm:
		return []binding{keyMap.Yes, keyMap.No, keyMap.Back, keyMap.Help, keyMap.Quit}
	case screenClearAllConfirm:
		return []binding{
			{keys: []string{"yes or project name"}, help: "type explicit confirmation"},
			keyMap.Confirm, keyMap.Back, keyMap.Help, keyMap.ForceQuit,
		}
	case screenPreview:
		return []binding{keyMap.Up, keyMap.Down, keyMap.Back, keyMap.Help, keyMap.Quit}
	case screenGenerating:
		return []binding{keyMap.Quit, keyMap.Help}
	case screenGenerateDone:
		return []binding{keyMap.Confirm, keyMap.Back, keyMap.Help, keyMap.Quit}
	case screenPushConfirm:
		return []binding{keyMap.Yes, keyMap.No, keyMap.Back, keyMap.Help, keyMap.Quit}
	case screenPushGuidance:
		return []binding{keyMap.Confirm, keyMap.Back, keyMap.Help, keyMap.Quit}
	case screenPushRepoType:
		return []binding{keyMap.Up, keyMap.Down, keyMap.Confirm, keyMap.Back, keyMap.Help, keyMap.Quit}
	case screenPushRemoteInput:
		return []binding{
			{keys: []string{"0-9/a-z/:._-@\\"}, help: "type remote URL"},
			{keys: []string{"backspace"}, help: "delete"},
			keyMap.Confirm, keyMap.Back, keyMap.Help, keyMap.ForceQuit,
		}
	case screenPushRunning:
		return []binding{keyMap.Up, keyMap.Down, keyMap.Help, keyMap.Quit}
	case screenPushDone:
		return []binding{keyMap.Confirm, keyMap.Back, keyMap.Help, keyMap.Quit}
	default:
		return []binding{keyMap.Help, keyMap.Quit}
	}
}
