package main

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D7D8C"))
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E4E4E7")).
			Background(lipgloss.Color("#52525B"))
	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#7D7D8C")).
				Padding(0, 1)
	inactivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#3F3F46")).
				Padding(0, 1)
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA"))

	activeDotStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFD7")) // Parlak Turkuaz
	inactiveDotStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4A4A5A")) // Mat Gri
)

type selectionKind int

const (
	noSelection selectionKind = iota
	projectSelection
	workspaceSelection
	templateSelection
)

type selection struct {
	kind  selectionKind
	index int
}

type selectorModel struct {
	projects       []Project
	workspaces     []string
	templates      []string
	activePanel    selectionKind
	projectIndex   int
	workspaceIndex int
	templateIndex  int

	projectPage   int
	workspacePage int
	templatePage  int

	selection selection
	width     int
	height    int
}

func newSelectorModel(projects []Project, workspaces []string, templates []string) selectorModel {
	activePanel := projectSelection
	if len(projects) == 0 {
		if len(workspaces) > 0 {
			activePanel = workspaceSelection
		} else {
			activePanel = templateSelection
		}
	}

	sortedProjects := make([]Project, len(projects))
	copy(sortedProjects, projects)
	sort.Slice(sortedProjects, func(i, j int) bool {
		if sortedProjects[i].OpenedAtLast.IsZero() && sortedProjects[j].OpenedAtLast.IsZero() {
			return sortedProjects[i].ID > sortedProjects[j].ID
		}
		if sortedProjects[i].OpenedAtLast.IsZero() {
			return false
		}
		if sortedProjects[j].OpenedAtLast.IsZero() {
			return true
		}
		return sortedProjects[i].OpenedAtLast.After(sortedProjects[j].OpenedAtLast)
	})

	return selectorModel{
		projects:    sortedProjects,
		workspaces:  workspaces,
		templates:   templates,
		activePanel: activePanel,
	}
}

func (m selectorModel) Init() tea.Cmd { return nil }

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if window, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = window.Width
		m.height = window.Height
		return m, nil
	}

	pageSizeProjects, pageSizeOther := m.getPageSizes()

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit

		case "tab":
			m.switchPanel(1)

		case "right", "l":
			m.changePage(1, pageSizeProjects, pageSizeOther)

		case "left", "h":
			m.changePage(-1, pageSizeProjects, pageSizeOther)

		case "up", "k":
			m.moveSelection(-1, pageSizeProjects, pageSizeOther)

		case "down", "j":
			m.moveSelection(1, pageSizeProjects, pageSizeOther)

		case "enter":
			if m.activePanel == projectSelection && len(m.projects) > 0 {
				m.selection = selection{kind: projectSelection, index: m.projectIndex}
				return m, tea.Quit
			}
			if m.activePanel == workspaceSelection && len(m.workspaces) > 0 {
				m.selection = selection{kind: workspaceSelection, index: m.workspaceIndex}
				return m, tea.Quit
			}
			if m.activePanel == templateSelection && len(m.templates) > 0 {
				m.selection = selection{kind: templateSelection, index: m.templateIndex}
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m *selectorModel) switchPanel(delta int) {
	panels := []selectionKind{projectSelection, workspaceSelection, templateSelection}
	currentIndex := 0
	for i, p := range panels {
		if p == m.activePanel {
			currentIndex = i
			break
		}
	}

	nextIdx := (currentIndex + delta + len(panels)) % len(panels)
	m.activePanel = panels[nextIdx]
}

func (m selectorModel) getPageSizes() (int, int) {
	panelHeight := 8
	if m.height > 0 {
		panelHeight = max(4, m.height-3)
	}

	netContentHeight := max(1, panelHeight-2)

	projPageSize := max(1, netContentHeight/2)
	otherPageSize := netContentHeight

	return projPageSize, otherPageSize
}

func (m *selectorModel) changePage(delta int, projPageSize, otherPageSize int) {
	switch m.activePanel {
	case projectSelection:
		totalPages := max(1, (len(m.projects)+projPageSize-1)/projPageSize)
		m.projectPage = (m.projectPage + delta + totalPages) % totalPages
		m.projectIndex = m.projectPage * projPageSize
	case workspaceSelection:
		totalPages := max(1, (len(m.workspaces)+otherPageSize-1)/otherPageSize)
		m.workspacePage = (m.workspacePage + delta + totalPages) % totalPages
		m.workspaceIndex = m.workspacePage * otherPageSize
	case templateSelection:
		totalPages := max(1, (len(m.templates)+otherPageSize-1)/otherPageSize)
		m.templatePage = (m.templatePage + delta + totalPages) % totalPages
		m.templateIndex = m.templatePage * otherPageSize
	}
}

func (m *selectorModel) moveSelection(delta int, projPageSize, otherPageSize int) {
	switch m.activePanel {
	case projectSelection:
		if len(m.projects) == 0 {
			return
		}
		m.projectIndex = (m.projectIndex + delta + len(m.projects)) % len(m.projects)
		m.projectPage = m.projectIndex / projPageSize

	case workspaceSelection:
		if len(m.workspaces) == 0 {
			return
		}
		m.workspaceIndex = (m.workspaceIndex + delta + len(m.workspaces)) % len(m.workspaces)
		m.workspacePage = m.workspaceIndex / otherPageSize

	case templateSelection:
		if len(m.templates) == 0 {
			return
		}
		m.templateIndex = (m.templateIndex + delta + len(m.templates)) % len(m.templates)
		m.templatePage = m.templateIndex / otherPageSize
	}
}

func (m selectorModel) View() string {
	panelWidth := 26
	panelHeight := 8

	if m.width > 0 {
		panelWidth = max(18, (m.width/3)-2)
	}

	if m.height > 0 {
		panelHeight = max(4, m.height-3)
	}

	projPageSize, otherPageSize := m.getPageSizes()

	projectPanel := m.renderPanel("Projects", m.getProjectPageItems(projPageSize), projectSelection, m.projectIndex, m.projectPage, len(m.projects), projPageSize, panelWidth, panelHeight)
	workspacePanel := m.renderPanel("Workspaces", m.getWorkspacePageItems(otherPageSize), workspaceSelection, m.workspaceIndex, m.workspacePage, len(m.workspaces), otherPageSize, panelWidth, panelHeight)
	templatePanel := m.renderPanel("Templates", m.getTemplatePageItems(otherPageSize), templateSelection, m.templateIndex, m.templatePage, len(m.templates), otherPageSize, panelWidth, panelHeight)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, projectPanel, workspacePanel, templatePanel)

	hasMultiplePages := false
	switch m.activePanel {
	case projectSelection:
		hasMultiplePages = len(m.projects) > projPageSize
	case workspaceSelection:
		hasMultiplePages = len(m.workspaces) > otherPageSize
	case templateSelection:
		hasMultiplePages = len(m.templates) > otherPageSize
	}

	footerText := "Tab: switch panel • ↑/↓: select • Enter: open/run/create • q: quit"
	if hasMultiplePages {
		footerText = "Tab: switch panel • ←/→: page • ↑/↓: select • Enter: open/run/create • q: quit"
	}

	footer := footerStyle.
		Width(m.width).
		Align(lipgloss.Center).
		Render(footerText)

	content := lipgloss.JoinVertical(lipgloss.Left, panels, footer)

	return lipgloss.NewStyle().
		MaxHeight(m.height).
		MaxWidth(m.width).
		Render(content)
}

func (m selectorModel) getProjectPageItems(pageSize int) []string {
	all := m.projectItems()
	start := m.projectPage * pageSize
	if start >= len(all) {
		return nil
	}
	end := min(len(all), start+pageSize)
	return all[start:end]
}

func (m selectorModel) getWorkspacePageItems(pageSize int) []string {
	all := m.workspaceItems()
	start := m.workspacePage * pageSize
	if start >= len(all) {
		return nil
	}
	end := min(len(all), start+pageSize)
	return all[start:end]
}

func (m selectorModel) getTemplatePageItems(pageSize int) []string {
	all := m.templateItems()
	start := m.templatePage * pageSize
	if start >= len(all) {
		return nil
	}
	end := min(len(all), start+pageSize)
	return all[start:end]
}

func (m selectorModel) renderPanel(title string, items []string, panel selectionKind, selectedIdx, currentPage, totalItems, pageSize, width, height int) string {
	totalPages := max(1, (totalItems+pageSize-1)/pageSize)

	lines := []string{titleStyle.Render(title)}

	pageOffset := currentPage * pageSize
	for i, item := range items {
		realIndex := pageOffset + i
		line := "  " + item

		if panel == m.activePanel && realIndex == selectedIdx {
			line = selectedStyle.Width(width - 2).Render("› " + item)
		} else if totalItems == 0 {
			line = mutedStyle.Render(line)
		}

		for l := range strings.SplitSeq(line, "\n") {
			lines = append(lines, l)
		}
	}

	maxContentLines := height - 2
	if totalPages > 1 {
		maxContentLines--
	}

	for len(lines) < maxContentLines {
		lines = append(lines, "")
	}

	if totalPages > 1 {
		var dots []string
		for i := range totalPages {
			if i == currentPage {
				dots = append(dots, activeDotStyle.Render("●"))
			} else {
				dots = append(dots, inactiveDotStyle.Render("○"))
			}
		}

		dotsStr := strings.Join(dots, " ")
		centeredDots := lipgloss.NewStyle().
			Width(width - 2).
			Align(lipgloss.Center).
			Render(dotsStr)

		lines = append(lines, centeredDots)
	}

	style := inactivePanelStyle
	if panel == m.activePanel {
		style = activePanelStyle
	}

	return style.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m selectorModel) projectItems() []string {
	if len(m.projects) == 0 {
		return []string{"No projects"}
	}
	items := make([]string, len(m.projects))
	for i, project := range m.projects {
		timestamp := FormatTimestamp(project.OpenedAtLast)
		items[i] = fmt.Sprintf("%s\n  %s", project.Name, mutedStyle.Render(timestamp))
	}
	return items
}

func (m selectorModel) workspaceItems() []string {
	if len(m.workspaces) == 0 {
		return []string{"No workspaces"}
	}
	return m.workspaces
}

func (m selectorModel) templateItems() []string {
	if len(m.templates) == 0 {
		return []string{"No templates"}
	}
	return m.templates
}
