package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type wsStyles struct {
	title        lipgloss.Style
	item         lipgloss.Style
	selectedItem lipgloss.Style
	pagination   lipgloss.Style
	help         lipgloss.Style
	quitText     lipgloss.Style
}

func newWSStyles() wsStyles {
	var s wsStyles
	s.title = lipgloss.NewStyle().MarginLeft(0).Bold(true).Foreground(lipgloss.Color("#FF5F87"))
	s.item = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#FAFAFA"))
	s.selectedItem = lipgloss.NewStyle().PaddingLeft(0).Bold(true).Foreground(lipgloss.Color("#00FFD7"))
	s.pagination = list.DefaultStyles().PaginationStyle.PaddingLeft(2)
	s.help = list.DefaultStyles().HelpStyle.PaddingLeft(2).PaddingBottom(1)
	s.quitText = lipgloss.NewStyle().Margin(1, 0, 2, 4)
	return s
}

type wsItem string

func (i wsItem) FilterValue() string { return "" }

type wsItemDelegate struct {
	styles *wsStyles
}

func (d *wsItemDelegate) Height() int                             { return 1 }
func (d *wsItemDelegate) Spacing() int                            { return 0 }
func (d *wsItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d *wsItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(wsItem)
	if !ok {
		return
	}

	str := string(i)

	fn := d.styles.item.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return d.styles.selectedItem.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

type wsModel struct {
	list     list.Model
	choice   string
	styles   wsStyles
	quitting bool
}

func initialWSModel(workspaces []string) wsModel {
	items := make([]list.Item, len(workspaces))
	for i, ws := range workspaces {
		items[i] = wsItem(ws)
	}

	const defaultWidth = 30

	l := list.New(items, &wsItemDelegate{}, defaultWidth, 0)
	l.Title = "Select a workspace to run:"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	m := wsModel{list: l}
	m.updateStyles()
	m.updateHelpKeys()
	return m
}

func (m *wsModel) updateStyles() {
	m.styles = newWSStyles()
	m.list.Styles.Title = m.styles.title
	m.list.Styles.PaginationStyle = m.styles.pagination
	m.list.Styles.HelpStyle = m.styles.help
	m.list.SetDelegate(&wsItemDelegate{styles: &m.styles})
}

func (m *wsModel) updateHelpKeys() {
	if m.list.Paginator.TotalPages > 1 {
		m.list.AdditionalShortHelpKeys = func() []key.Binding {
			return []key.Binding{
				key.NewBinding(
					key.WithKeys("left", "right", "h", "l"),
					key.WithHelp("←/→", "prev/next page"),
				),
			}
		}
	} else {
		m.list.AdditionalShortHelpKeys = nil
	}
}

func (m wsModel) Init() tea.Cmd {
	return nil
}

func (m wsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if i, ok := m.list.SelectedItem().(wsItem); ok {
				m.choice = string(i)
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.updateHelpKeys()
	return m, cmd
}

func (m wsModel) View() string {
	if m.choice != "" || m.quitting {
		return ""
	}
	return "\n" + m.list.View()
}

func (a *App) cmdWorkspace(args []string) {
	if len(args) == 0 {
		a.cmdWorkspaceInteractive()
		return
	}

	switch args[0] {
	case "list":
		a.cmdWorkspaceList()
	case "dir":
		a.cmdWorkspaceDir()
	case "run":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: Missing workspace script name.")
			fmt.Fprintln(os.Stderr, "Usage: flint ws run <name>")
			return
		}
		a.runWorkspace(args[1])
	default:
		a.runWorkspace(args[0])
	}
}

func (a *App) cmdWorkspaceInteractive() {
	workspaces, err := a.workspaces()
	if err != nil || len(workspaces) == 0 {
		fmt.Println("No workspace scripts found.")
		return
	}

	m := initialWSModel(workspaces)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running workspace selector: %v\n", err)
		return
	}

	if model, ok := finalModel.(wsModel); ok && model.choice != "" {
		a.runWorkspace(model.choice)
	}
}

func (a *App) cmdWorkspaceList() {
	workspaces, err := a.workspaces()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading workspaces: %v\n", err)
		return
	}

	fmt.Println("Available Workspaces:")
	for _, workspace := range workspaces {
		fmt.Printf("  - %s\n", workspace)
	}

	if len(workspaces) == 0 {
		fmt.Println("  (No .fws workspace scripts found)")
	}
}

func (a *App) cmdWorkspaceDir() {
	openOSNative(a.Config.WorkspacesDir)
}

func (a *App) workspaces() ([]string, error) {
	files, err := os.ReadDir(a.Config.WorkspacesDir)
	if err != nil {
		return nil, err
	}

	workspaces := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".fws") {
			workspaces = append(workspaces, strings.TrimSuffix(file.Name(), ".fws"))
		}
	}
	return workspaces, nil
}

func (a *App) runWorkspace(wsName string) {
	if !strings.HasSuffix(wsName, ".fws") {
		wsName += ".fws"
	}

	wsPath := filepath.Join(a.Config.WorkspacesDir, wsName)
	file, err := os.Open(wsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening workspace file '%s': %v\n", wsName, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		executeBackgroundCmd(line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading workspace file '%s': %v\n", wsName, err)
	}
}
