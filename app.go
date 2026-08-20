package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"
)

type App struct {
	ConfigDir string
	Config    Config
	Projects  []Project
}

func newApp() (*App, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	workspacesDir := getWorkspacesDir(configDir)
	if err := os.MkdirAll(workspacesDir, 0755); err != nil {
		return nil, err
	}

	app := &App{
		ConfigDir: configDir,
		Config: Config{
			DefaultEditor: "code",
			WorkspacesDir: workspacesDir,
		},
		Projects: []Project{},
	}

	if err := app.loadConfig(); err != nil {
		return nil, err
	}

	if err := app.loadProjects(); err != nil {
		return nil, err
	}

	return app, nil
}

func (a *App) cmdHelp() {

	const logo = ` *  .    .  (   *  .  (   *
   )   *     )   .     )   *
  _____ _     ___ _   _ _____
 |  ___| |   |_ _| \ | |_   _|
 | |_  | |    | ||  \| | | |
 |  _| | |___ | || |\  | | |
 |_|   |_____|___|_| \_| |_|
`
	helpText :=
		logo + version + `

Usage:
  flint                           Launch interactive project/workspace selector
  flint help                      Show CLI help instructions
  flint version                   Show Flint version
  flint editor [cmd]              Set or view default editor command

Projects:
  flint list                      List all registered projects
  flint select                    Launch interactive project/workspace selector
  flint open <name|id>            Open project in its editor
  flint dir                       Open last accessed project in file manager
  flint dir <name|id>             Open specific project in file manager
  flint last                      Open the most recently accessed project
  flint add                       Add a new project interactively
  flint add -n <name> -p <path>   Add a project via flags
  flint add -ps [-n <name>]       Select a project folder with a native dialog
  flint init [-n <name>]          Add the current directory as a project
  flint edit <name|id>            Edit an existing project
  flint rm <name|id>              Remove a project

Workspaces:
  flint ws                        Launch interactive workspace selector
  flint ws list                   List available workspace scripts (.fws)
  flint ws dir                    Open workspace scripts directory
  flint ws run <name>             Execute workspace script

Templates (aliases: tpl, template, tmpl):
  flint tpl                       Launch interactive template selector
  flint tpl list                  List available templates
  flint tpl dir                   Open templates directory
  flint tpl init                  Create template.yml in current directory
  flint tpl new <tpl> <name>      Create project from template
    Flags:
      -p, --path <path>           Custom target directory
      -v, --var KEY=VALUE         Override template variable

Scripting & Automation:
  flint projects-json             Output all projects as JSON
  flint project-json <name|id>    Output specific project as JSON
  flint last-project-json         Output last accessed project as JSON
  flint workspaces-json           Output all workspaces as JSON
  flint templates-json            Output all templates as JSON
  flint config-json               Output config as JSON
  flint last-project-name         Output last project name
  flint last-project-path         Output last project path
  flint project-path <name|id>    Output project path
  flint config-path               Output config directory path
  flint workspaces-path           Output workspaces directory path
  flint templates-path            Output templates directory path
  flint projects-path             Output projects.json file path
`
	fmt.Print(helpText)
}

func (a *App) cmdList() {
	if len(a.Projects) == 0 {
		fmt.Println("No projects registered yet.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tPROJECT NAME\tPATH\tEDITOR")
	fmt.Fprintln(w, "--\t------------\t----\t------")

	for _, p := range a.Projects {
		editor := p.Editor
		if editor == "" {
			editor = "(default)"
		}
		path := p.Path
		if len(path) > 40 {
			path = "..." + path[len(path)-37:]
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", p.ID, p.Name, path, editor)
	}
	w.Flush()
}

func (a *App) cmdSelect() {
	workspaces, err := a.workspaces()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading workspaces: %v\n", err)
		return
	}

	templates, err := a.templates()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading templates: %v\n", err)
		return
	}

	if len(a.Projects) == 0 && len(workspaces) == 0 && len(templates) == 0 {
		fmt.Println("No projects, workspaces, or templates available.")
		return
	}

	model := newSelectorModel(a.Projects, workspaces, templates)
	result, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Selector error: %v\n", err)
		return
	}

	selected := result.(selectorModel)
	switch selected.selection.kind {
	case projectSelection:
		selectedProject := selected.projects[selected.selection.index]
		fmt.Printf("Opening project: %s\n", selectedProject.Name)
		p := a.findProject(fmt.Sprintf("%d", selectedProject.ID))
		if p != nil {
			a.openProject(p)
		}
	case workspaceSelection:
		workspace := workspaces[selected.selection.index]
		fmt.Printf("Running workspace: %s\n", workspace)
		a.runWorkspace(workspace)
	case templateSelection:
		selectedTemplate := templates[selected.selection.index]

		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Enter project name for template '%s': ", selectedTemplate)
		projectName, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading project name: %v\n", err)
			return
		}

		projectName = strings.TrimSpace(projectName)
		if projectName == "" {
			fmt.Println("Project name cannot be empty.")
			return
		}

		a.cmdTemplateNew([]string{selectedTemplate, projectName})
	}
}

func (a *App) cmdEditor(args []string) {
	if len(args) > 0 {
		a.Config.DefaultEditor = args[0]
		if err := a.saveConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to save editor config: %v\n", err)
			return
		}
		fmt.Printf("Default editor set to: %s\n", a.Config.DefaultEditor)
		return
	}

	result, canceled := a.editField("Default Editor Command", a.Config.DefaultEditor)
	if canceled {
		return
	}

	if result == "" {
		fmt.Println("Editor command cannot be empty.")
		return
	}

	a.Config.DefaultEditor = result
	if err := a.saveConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to save editor config: %v\n", err)
		return
	}

	fmt.Printf("Default editor updated to: %s\n", a.Config.DefaultEditor)
}

func (a *App) cmdConfigJSON() {
	data, err := json.MarshalIndent(a.Config, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func (a *App) cmdConfigPath() {
	fmt.Println(a.ConfigDir)
}

func (a *App) cmdWorkspacesPath() {
	fmt.Println(a.Config.WorkspacesDir)
}

func (a *App) cmdProjectsPath() {
	projectsPath := filepath.Join(a.ConfigDir, "projects.json")
	fmt.Println(projectsPath)
}
