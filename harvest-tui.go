package main

import (
	"fmt"
	"log"
	"os"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-resty/resty/v2"
)

// Constants and styles
const (
	defaultBaseURL = "https://api.harvestapp.com/v2"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))
)

// Configuration holds the Harvest API credentials
type Configuration struct {
	AccountID   string
	AccessToken string
	BaseURL     string
}

// HarvestClient handles API communication
type HarvestClient struct {
	config Configuration
	client *resty.Client
}

// Project represents a Harvest project
type Project struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Task represents a Harvest task
type Task struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Timer represents a running Harvest timer
type Timer struct {
	ID        int     `json:"id"`
	Notes     string  `json:"notes"`
	Hours     float64 `json:"hours"`
	ProjectID int     `json:"project_id"`
	TaskID    int     `json:"task_id"`
	IsRunning bool    `json:"is_running"`
}

// ListItem for bubbles list
type ListItem struct {
	ID   int
	Name string
}

func (i ListItem) FilterValue() string { return i.Name }
func (i ListItem) Title() string       { return i.Name }
func (i ListItem) Description() string { return fmt.Sprintf("ID: %d", i.ID) }

// Model represents the application state
type Model struct {
	harvestClient   *HarvestClient
	state           string
	projects        []Project
	tasks           []Task
	selectedProject Project
	selectedTask    Task
	ticketInput     textinput.Model
	projectList     list.Model
	taskList        list.Model
	activeTimer     *Timer
	error           string
	success         string
	quitting        bool
	showHelp        bool
}

// Initialize the Harvest client
func NewHarvestClient(config Configuration) *HarvestClient {
	// Default to the API URL if none provided
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}

	// Create resty client with TLS configuration
	client := resty.New()
	client.SetBaseURL(config.BaseURL)
	client.SetHeader("Harvest-Account-ID", config.AccountID)
	client.SetHeader("Authorization", "Bearer "+config.AccessToken)
	client.SetHeader("User-Agent", "Harvest-TUI/1.0 (your-email@example.com)")
	client.SetHeader("Content-Type", "application/json")
	client.SetHeader("Accept", "application/json")

	// Set TLS configuration for secure HTTPS connections
	client.SetTLSClientConfig(nil) // Use default which validates certificates

	return &HarvestClient{
		config: config,
		client: client,
	}
}

// TestConnection verifies your API credentials work
func (h *HarvestClient) TestConnection() error {
	resp, err := h.client.R().
		SetResult(map[string]interface{}{}).
		Get("/users/me")
	if err != nil {
		return err
	}

	if resp.IsError() {
		return fmt.Errorf("API connection test failed: Status=%d, Body=%s",
			resp.StatusCode(), resp.String())
	}

	return nil
}

// Fetch user's recent/active projects through timesheet data
func (h *HarvestClient) GetProjects() ([]Project, error) {
	// Try the time entries endpoint to get recent projects
	resp, err := h.client.R().
		SetResult(struct {
			TimeEntries []struct {
				Project struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"project"`
			} `json:"time_entries"`
		}{}).
		Get("/time_entries?per_page=100")
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		return nil, fmt.Errorf("Failed to fetch projects: %v", err)
	}

	result := resp.Result().(*struct {
		TimeEntries []struct {
			Project struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"project"`
		} `json:"time_entries"`
	})

	// Extract unique projects from time entries
	projectMap := make(map[int]Project)
	for _, entry := range result.TimeEntries {
		projectMap[entry.Project.ID] = Project{
			ID:   entry.Project.ID,
			Name: entry.Project.Name,
		}
	}

	projects := make([]Project, 0, len(projectMap))
	for _, project := range projectMap {
		projects = append(projects, project)
	}

	return projects, nil
}

// Fetch tasks for a specific project
func (h *HarvestClient) GetTasks(projectID int) ([]Task, error) {
	// Try to get tasks from recent time entries for this project
	resp, err := h.client.R().
		SetResult(struct {
			TimeEntries []struct {
				Task struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"task"`
			} `json:"time_entries"`
		}{}).
		Get(fmt.Sprintf("/time_entries?project_id=%d&per_page=100", projectID))
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		return nil, fmt.Errorf("API error: Status=%d, Body=%s", resp.StatusCode(), resp.String())
	}

	result := resp.Result().(*struct {
		TimeEntries []struct {
			Task struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"task"`
		} `json:"time_entries"`
	})

	// Extract unique tasks from time entries
	taskMap := make(map[int]Task)
	for _, entry := range result.TimeEntries {
		taskMap[entry.Task.ID] = Task{
			ID:   entry.Task.ID,
			Name: entry.Task.Name,
		}
	}

	tasks := make([]Task, 0, len(taskMap))
	for _, task := range taskMap {
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// Start a timer for a project/task with notes
func (h *HarvestClient) StartTimer(projectID, taskID int, notes string) (*Timer, error) {
	payload := map[string]interface{}{
		"project_id": projectID,
		"task_id":    taskID,
		"notes":      notes,
	}

	var timerResp struct {
		ID        int     `json:"id"`
		Notes     string  `json:"notes"`
		Hours     float64 `json:"hours"`
		ProjectID int     `json:"project_id"`
		TaskID    int     `json:"task_id"`
		IsRunning bool    `json:"is_running"`
	}

	resp, err := h.client.R().
		SetBody(payload).
		SetResult(&timerResp).
		Post("/time_entries")
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		return nil, fmt.Errorf("API error: Status=%d, Body=%s", resp.StatusCode(), resp.String())
	}

	timer := &Timer{
		ID:        timerResp.ID,
		Notes:     timerResp.Notes,
		Hours:     timerResp.Hours,
		ProjectID: timerResp.ProjectID,
		TaskID:    timerResp.TaskID,
		IsRunning: timerResp.IsRunning,
	}

	return timer, nil
}

// Stop a running timer
func (h *HarvestClient) StopTimer(timerID int) error {
	resp, err := h.client.R().
		Patch(fmt.Sprintf("/time_entries/%d/stop", timerID))
	if err != nil {
		return err
	}

	if resp.IsError() {
		return fmt.Errorf("API error: Status=%d, Body=%s", resp.StatusCode(), resp.String())
	}

	return nil
}

// Initialize the application model
func initialModel(config Configuration) Model {
	harvestClient := NewHarvestClient(config)

	// Initialize text input for ticket/title
	ticketInput := textinput.New()
	ticketInput.Placeholder = "Ticket-123 - Description of work"
	ticketInput.Focus()
	ticketInput.Width = 50

	// Initialize list models
	projectList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	projectList.Title = "Select Project"
	projectList.SetShowStatusBar(false)
	projectList.SetFilteringEnabled(true)
	projectList.Styles.Title = lipgloss.NewStyle().Bold(true)

	taskList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	taskList.Title = "Select Task"
	taskList.SetShowStatusBar(false)
	taskList.SetFilteringEnabled(true)
	taskList.Styles.Title = lipgloss.NewStyle().Bold(true)

	return Model{
		harvestClient: harvestClient,
		state:         "loading_projects",
		ticketInput:   ticketInput,
		projectList:   projectList,
		taskList:      taskList,
	}
}

// Define TUI messages
type (
	fetchProjectsMsg struct{ projects []Project }
	fetchTasksMsg    struct{ tasks []Task }
	startTimerMsg    struct{ timer *Timer }
	stopTimerMsg     struct{ success bool }
	errorMsg         struct{ error string }
)

// Init initializes the model with the first command
func (m Model) Init() tea.Cmd {
	return fetchProjects(m.harvestClient)
}

// Update function for the Bubble Tea framework
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "esc":
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}

			switch m.state {
			case "select_task":
				m.state = "select_project"
				return m, nil
			case "enter_details":
				m.state = "select_task"
				return m, nil
			}
		case "enter":
			m.error = ""
			m.success = ""

			switch m.state {
			case "select_project":
				if len(m.projects) > 0 {
					idx := m.projectList.Index()
					if idx >= 0 && idx < len(m.projects) {
						m.selectedProject = m.projects[idx]
						m.state = "loading_tasks"
						return m, fetchTasks(m.harvestClient, m.selectedProject.ID)
					}
				}
			case "select_task":
				if len(m.tasks) > 0 {
					idx := m.taskList.Index()
					if idx >= 0 && idx < len(m.tasks) {
						m.selectedTask = m.tasks[idx]
						m.state = "enter_details"
						m.ticketInput.Focus()
						return m, nil
					}
				}
			case "enter_details":
				if m.ticketInput.Value() == "" {
					m.error = "Please enter ticket number and description"
					return m, nil
				}

				// If we have an active timer, stop it
				if m.activeTimer != nil {
					return m, stopTimer(m.harvestClient, m.activeTimer.ID)
				}

				// Otherwise start a new timer
				return m, startTimer(
					m.harvestClient,
					m.selectedProject.ID,
					m.selectedTask.ID,
					m.ticketInput.Value(),
				)
			}
		}

	case fetchProjectsMsg:
		m.projects = msg.projects
		m.state = "select_project"

		// Convert projects to list items
		items := make([]list.Item, len(m.projects))
		for i, project := range m.projects {
			items[i] = ListItem{ID: project.ID, Name: project.Name}
		}
		m.projectList.SetItems(items)

	case fetchTasksMsg:
		m.tasks = msg.tasks
		m.state = "select_task"

		// Convert tasks to list items
		items := make([]list.Item, len(m.tasks))
		for i, task := range m.tasks {
			items[i] = ListItem{ID: task.ID, Name: task.Name}
		}
		m.taskList.SetItems(items)

	case startTimerMsg:
		m.activeTimer = msg.timer
		m.success = fmt.Sprintf("Timer started for: %s", m.ticketInput.Value())

	case stopTimerMsg:
		if msg.success {
			m.success = "Timer stopped"
			m.activeTimer = nil
		} else {
			m.error = "Failed to stop timer"
		}

	case errorMsg:
		m.error = msg.error
		if m.state == "loading_projects" || m.state == "loading_tasks" {
			m.state = "error"
		}

	case tea.WindowSizeMsg:
		// Handle window size changes
		h, v := docStyle.GetFrameSize()
		m.projectList.SetSize(msg.Width-h, msg.Height-v)
		m.taskList.SetSize(msg.Width-h, msg.Height-v)
	}

	// Handle input updates
	var cmd tea.Cmd
	if m.state == "enter_details" {
		if m.ticketInput.Focused() {
			m.ticketInput, cmd = m.ticketInput.Update(msg)
			return m, cmd
		}
	}

	// Handle list updates
	if m.state == "select_project" {
		var cmd tea.Cmd
		m.projectList, cmd = m.projectList.Update(msg)
		return m, cmd
	} else if m.state == "select_task" {
		var cmd tea.Cmd
		m.taskList, cmd = m.taskList.Update(msg)
		return m, cmd
	}

	return m, nil
}

var docStyle = lipgloss.NewStyle().Margin(1, 2)

// View function for the Bubble Tea framework
func (m Model) View() string {
	if m.quitting {
		return "Bye!\n"
	}

	var s string
	title := titleStyle.Render("✓ Harvest Timer TUI")

	if m.showHelp {
		return docStyle.Render(title + "\n\n" + helpContent)
	}

	switch m.state {
	case "loading_projects":
		s = "Loading projects...\n"
	case "loading_tasks":
		s = "Loading tasks...\n"
	case "select_project":
		s = m.projectList.View()
	case "select_task":
		s = fmt.Sprintf(
			"Project: %s\n\n%s",
			m.selectedProject.Name,
			m.taskList.View(),
		)
	case "enter_details":
		status := ""
		actionKey := "Enter"
		actionText := "Start Timer"

		if m.activeTimer != nil {
			status = infoStyle.Render(fmt.Sprintf("\nTimer running: %s (%.2f hours)",
				m.activeTimer.Notes, m.activeTimer.Hours))
			actionText = "Stop Timer"
		}

		s = fmt.Sprintf(
			"Project: %s\nTask: %s\n\n%s%s\n\nPress %s to %s",
			m.selectedProject.Name,
			m.selectedTask.Name,
			m.ticketInput.View(),
			status,
			actionKey,
			actionText,
		)
	case "error":
		s = fmt.Sprintf("Error: %s\nPress q to quit.", m.error)
	}

	if m.error != "" && m.state != "error" {
		errorText := errorStyle.Render("Error: " + m.error)
		s += "\n\n" + errorText
	}

	if m.success != "" {
		successText := successStyle.Render("✓ " + m.success)
		s += "\n\n" + successText
	}

	header := title + "\n\n"
	var footer string

	switch m.state {
	case "select_project", "select_task":
		footer = "\n\nPress ↑/↓ to navigate, / to filter, Enter to select, Esc to go back, ? for help, q to quit"
	case "enter_details":
		footer = "\n\nPress Enter to start/stop timer, Esc to go back, ? for help, q to quit"
	default:
		footer = "\n\nPress ? for help, q to quit"
	}

	return docStyle.Render(header + s + footer)
}

// Command to fetch projects
func fetchProjects(client *HarvestClient) tea.Cmd {
	return func() tea.Msg {
		projects, err := client.GetProjects()
		if err != nil {
			return errorMsg{error: err.Error()}
		}
		return fetchProjectsMsg{projects: projects}
	}
}

// Command to fetch tasks
func fetchTasks(client *HarvestClient, projectID int) tea.Cmd {
	return func() tea.Msg {
		tasks, err := client.GetTasks(projectID)
		if err != nil {
			return errorMsg{error: err.Error()}
		}
		return fetchTasksMsg{tasks: tasks}
	}
}

// Command to start a timer
func startTimer(client *HarvestClient, projectID, taskID int, notes string) tea.Cmd {
	return func() tea.Msg {
		timer, err := client.StartTimer(projectID, taskID, notes)
		if err != nil {
			return errorMsg{error: err.Error()}
		}
		return startTimerMsg{timer: timer}
	}
}

// Command to stop a timer
func stopTimer(client *HarvestClient, timerID int) tea.Cmd {
	return func() tea.Msg {
		err := client.StopTimer(timerID)
		if err != nil {
			return errorMsg{error: err.Error()}
		}
		return stopTimerMsg{success: true}
	}
}

// Help content
const helpContent = `
KEYBOARD SHORTCUTS
  ↑/↓          Navigate through options
  /            Filter the list (start typing to search)
  Enter        Select project/task or start/stop timer
  Esc          Go back to previous screen
  ?            Show/hide this help
  q or Ctrl+C  Quit the application

WORKFLOW
  1. Select a project
  2. Select a task
  3. Enter ticket number and description (e.g., "TICKET-123 - Add new feature")
  4. Press Enter to start the timer
  5. Press Enter again to stop the timer

Your Harvest timer will sync automatically with the web interface.
`

func main() {
	// Load configuration from environment variables
	config := Configuration{
		AccountID:   os.Getenv("HARVEST_ACCOUNT_ID"),
		AccessToken: os.Getenv("HARVEST_ACCESS_TOKEN"),
	}

	// Validate configuration
	if config.AccountID == "" || config.AccessToken == "" {
		log.Fatal("HARVEST_ACCOUNT_ID and HARVEST_ACCESS_TOKEN environment variables must be set")
	}

	// Create Harvest client to test connection
	client := NewHarvestClient(config)
	if err := client.TestConnection(); err != nil {
		fmt.Printf("\n⛔ ERROR: %v\n\n", err)
		fmt.Println("Please check your Harvest API credentials and access.")
		os.Exit(1)
	}

	// Initialize the model
	model := initialModel(config)

	// Start the program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run the program
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
