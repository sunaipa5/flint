package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ncruces/zenity"
)

type Project struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Editor       string    `json:"editor"`
	OpenedAtLast time.Time `json:"opened_at_last"`
}

func (a *App) loadProjects() error {
	projectsPath := filepath.Join(a.ConfigDir, "projects.json")
	if _, err := os.Stat(projectsPath); os.IsNotExist(err) {
		return a.saveProjects()
	}

	data, err := os.ReadFile(projectsPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &a.Projects)
}

func (a *App) saveProjects() error {
	projectsPath := filepath.Join(a.ConfigDir, "projects.json")
	data, err := json.MarshalIndent(a.Projects, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(projectsPath, data, 0644)
}

func (a *App) findProject(query string) *Project {
	id, err := strconv.Atoi(query)
	isID := err == nil

	for i := range a.Projects {
		if isID && a.Projects[i].ID == id {
			return &a.Projects[i]
		}
		if a.Projects[i].Name == query {
			return &a.Projects[i]
		}
	}
	return nil
}

func (a *App) cmdLast() {
	if len(a.Projects) == 0 {
		fmt.Println("No registered projects found.")
		return
	}

	lastProject := &a.Projects[len(a.Projects)-1]
	fmt.Printf("Opening last accessed project: %s\n", lastProject.Name)
	a.openProject(lastProject)
}

func (a *App) cmdOpen(args []string) {
	if len(args) == 0 {
		fmt.Println("Error: Missing project name or ID.")
		return
	}

	p := a.findProject(args[0])
	if p == nil {
		fmt.Printf("Error: Project '%s' not found.\n", args[0])
		return
	}

	a.openProject(p)
}

func (a *App) cmdProjectDir(args []string) {
	if len(a.Projects) == 0 {
		fmt.Println("No registered projects found.")
		return
	}

	cleanArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "dir" {
			cleanArgs = append(cleanArgs, arg)
		}
	}

	targetPath := ""

	if len(cleanArgs) == 0 {
		lastProject := &a.Projects[len(a.Projects)-1]
		fmt.Printf("Opening directory for last project: %s\n", lastProject.Name)
		targetPath = lastProject.Path
	} else {
		query := cleanArgs[0]
		p := a.findProject(query)

		if p == nil || p.Path == "" || p.Name == "" {
			fmt.Printf("Error: Project '%s' not found.\n", query)
			return
		}
		targetPath = p.Path
	}

	openOSNative(targetPath)
}

func (a *App) cmdAdd(args []string) {
	addCmd := flag.NewFlagSet("add", flag.ContinueOnError)
	nameFlag := addCmd.String("n", "", "Project Name")
	pathFlag := addCmd.String("p", "", "Project Path")
	pathSelectorFlag := addCmd.Bool("ps", false, "Select project path with a native dialog")
	if err := addCmd.Parse(args); err != nil {
		return
	}

	var name, path, editor string
	if *pathSelectorFlag {
		if *pathFlag != "" {
			fmt.Println("Error: -p and -ps cannot be used together.")
			return
		}

		selectedPath, err := zenity.SelectFile(zenity.Directory(), zenity.Title("Select Project Folder"))
		if err != nil {
			if !errors.Is(err, zenity.ErrCanceled) {
				fmt.Fprintf(os.Stderr, "Could not select project folder: %v\n", err)
			}
			return
		}
		path = selectedPath
		name = *nameFlag
		if name == "" {
			name = filepath.Base(filepath.Clean(path))
		}
	} else if *nameFlag != "" && *pathFlag != "" {
		name = *nameFlag
		path = *pathFlag
	} else {
		name := a.getInputWithRetry("project name", "Name cannot be empty.", 2)
		if name == "" {
			fmt.Println("Aborted.")
			return
		}

		path := a.getInputWithRetry("project path", "Path cannot be empty.", 2)
		if path == "" {
			fmt.Println("Aborted.")
			return
		}

		editor, canceled := a.editField("editor (optional, press enter for global default)", "")
		if canceled {
			fmt.Println("Aborted.")
			return
		}
		editor = strings.TrimSpace(editor)
	}

	if err := a.addProject(name, path, editor); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add project: %v\n", err)
		return
	}

	fmt.Printf("Project '%s' added successfully.\n", name)
}

func (a *App) cmdInit(args []string) {
	initCmd := flag.NewFlagSet("init", flag.ContinueOnError)
	nameFlag := initCmd.String("n", "", "Project Name")
	if err := initCmd.Parse(args); err != nil {
		return
	}

	path, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to determine current directory: %v\n", err)
		return
	}
	name := *nameFlag
	if name == "" {
		name = filepath.Base(path)
	}

	if err := a.addProject(name, path, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize project: %v\n", err)
		return
	}
	fmt.Printf("Project '%s' added successfully.\n", name)
}

func (a *App) addProject(name, path, editor string) error {
	name = strings.TrimSpace(name)
	path = strings.TrimSpace(path)
	if name == "" || path == "" {
		return errors.New("name and path cannot be empty")
	}

	for _, project := range a.Projects {
		if project.Name == name {
			return fmt.Errorf("a project named %q already exists", name)
		}
		if sameProjectPath(project.Path, path) {
			return fmt.Errorf("a project already uses path %q", path)
		}
	}

	newID := 1
	for _, project := range a.Projects {
		newID = max(newID, project.ID+1)
	}
	a.Projects = append(a.Projects, Project{ID: newID, Name: name, Path: path, Editor: editor})
	if err := a.saveProjects(); err != nil {
		a.Projects = a.Projects[:len(a.Projects)-1]
		return err
	}
	return nil
}

func sameProjectPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return filepath.Clean(left) == filepath.Clean(right)
	}
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

type editFieldModel struct {
	label     string
	value     string
	original  string
	cursorPos int
	done      bool
	canceled  bool
}

func (m editFieldModel) Init() tea.Cmd {
	return nil
}

func (m editFieldModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.canceled = true
			m.done = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		case "backspace":
			if m.cursorPos > 0 {
				m.value = m.value[:m.cursorPos-1] + m.value[m.cursorPos:]
				m.cursorPos--
			}
		case "left":
			if m.cursorPos > 0 {
				m.cursorPos--
			}
		case "right":
			if m.cursorPos < len(m.value) {
				m.cursorPos++
			}
		case "home":
			m.cursorPos = 0
		case "end":
			m.cursorPos = len(m.value)
		default:
			if len(msg.String()) == 1 {
				m.value = m.value[:m.cursorPos] + msg.String() + m.value[m.cursorPos:]
				m.cursorPos++
			}
		}
	}
	return m, nil
}

func (m editFieldModel) View() string {
	cursor := "█"
	before := m.value[:m.cursorPos]
	after := m.value[m.cursorPos:]
	if m.done {
		return fmt.Sprintf("%s: %s", m.label, m.value)
	}
	return fmt.Sprintf("%s: %s%s%s", m.label, before, cursor, after)
}

func (a *App) editField(label, currentValue string) (string, bool) {
	m := editFieldModel{
		label:     label,
		value:     currentValue,
		original:  currentValue,
		cursorPos: len(currentValue),
	}
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Input error: %v\n", err)
		return currentValue, false
	}
	result := finalModel.(editFieldModel)
	return result.value, result.canceled
}

func (a *App) getInputWithRetry(label, errorMsg string, maxRetries int) string {
	for range maxRetries {
		value, canceled := a.editField(label, "")
		if canceled {
			return ""
		}
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
		fmt.Printf("Error: %s\n", errorMsg)
	}
	return ""
}

func (a *App) cmdEdit(args []string) {
	if len(args) == 0 {
		fmt.Println("Error: Missing project name or ID.")
		return
	}

	p := a.findProject(args[0])
	if p == nil {
		fmt.Printf("Error: Project '%s' not found.\n", args[0])
		return
	}

	name, canceled := a.editField("project name", p.Name)
	if canceled {
		fmt.Println("Aborted.")
		return
	}
	p.Name = strings.TrimSpace(name)

	path, canceled := a.editField("project path", p.Path)
	if canceled {
		fmt.Println("Aborted.")
		return
	}
	p.Path = strings.TrimSpace(path)

	editor, canceled := a.editField("editor (leave empty for global default)", p.Editor)
	if canceled {
		fmt.Println("Aborted.")
		return
	}
	p.Editor = strings.TrimSpace(editor)

	if err := a.saveProjects(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save project edits: %v\n", err)
		return
	}

	fmt.Printf("Project '%s' updated successfully.\n", p.Name)
}

func (a *App) cmdRemove(args []string) {
	if len(args) == 0 {
		fmt.Println("Error: Missing project name or ID.")
		return
	}

	p := a.findProject(args[0])
	if p == nil {
		fmt.Printf("Error: Project '%s' not found.\n", args[0])
		return
	}

	confirm, canceled := a.editField(fmt.Sprintf("Are you sure you want to delete '%s'? [y/N]", p.Name), "")
	if canceled {
		fmt.Println("Aborted.")
		return
	}
	confirm = strings.ToLower(strings.TrimSpace(confirm))

	if confirm != "y" && confirm != "yes" {
		fmt.Println("Aborted.")
		return
	}

	var updated []Project
	for _, item := range a.Projects {
		if item.ID != p.ID {
			updated = append(updated, item)
		}
	}

	a.Projects = updated
	if err := a.saveProjects(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update projects: %v\n", err)
		return
	}

	fmt.Println("Project deleted.")
}

func (a *App) openProject(p *Project) {
	p.OpenedAtLast = time.Now()
	if err := a.saveProjects(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update project timestamp: %v\n", err)
	}

	editor := p.Editor
	if editor == "" {
		editor = a.Config.DefaultEditor
	}

	if editor != "" {
		if err := executeCommand(editor, p.Path); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to launch editor '%s': %v\nFalling back to system default...\n", editor, err)
			openOSNative(p.Path)
		}
	} else {
		openOSNative(p.Path)
	}
}

func FormatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "Never opened"
	}
	now := time.Now()
	diff := now.Sub(t)

	if diff < 0 {
		return t.Format("02/01/2006 15:04")
	}

	if diff < time.Minute {
		return "Just now"
	}

	if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}

	y, m, d := now.Date()
	todayStart := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	yesterdayStart := todayStart.AddDate(0, 0, -1)

	if t.After(todayStart) {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}

	if t.After(yesterdayStart) {
		return fmt.Sprintf("Yesterday %s", t.Format("15:04"))
	}

	if diff < 7*24*time.Hour {
		return t.Format("Monday 15:04")
	}

	return t.Format("02/01/2006 15:04")
}

func (a *App) cmdProjectsJSON() {
	data, err := json.MarshalIndent(a.Projects, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func (a *App) cmdLastProjectJSON() {
	if len(a.Projects) == 0 {
		fmt.Fprintf(os.Stderr, "No projects registered\n")
		return
	}
	last := a.Projects[len(a.Projects)-1]
	data, err := json.MarshalIndent(last, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func (a *App) cmdProjectJSON(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: Missing project name or ID\n")
		return
	}
	p := a.findProject(args[0])
	if p == nil {
		fmt.Fprintf(os.Stderr, "Error: Project '%s' not found\n", args[0])
		return
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func (a *App) cmdWorkspacesJSON() {
	workspaces, err := a.workspaces()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	data, err := json.MarshalIndent(workspaces, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func (a *App) cmdLastProjectName() {
	if len(a.Projects) == 0 {
		fmt.Fprintf(os.Stderr, "No projects registered\n")
		return
	}
	last := a.Projects[len(a.Projects)-1]
	fmt.Println(last.Name)
}

func (a *App) cmdLastProjectPath() {
	if len(a.Projects) == 0 {
		fmt.Fprintf(os.Stderr, "No projects registered\n")
		return
	}
	last := a.Projects[len(a.Projects)-1]
	fmt.Println(last.Path)
}

func (a *App) cmdProjectPath(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: Missing project name or ID\n")
		return
	}
	p := a.findProject(args[0])
	if p == nil {
		fmt.Fprintf(os.Stderr, "Error: Project '%s' not found\n", args[0])
		return
	}
	fmt.Println(p.Path)
}
