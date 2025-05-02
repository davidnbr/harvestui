# harvester

# Harvest Timer TUI

A secure, simple text-based user interface for Harvest time tracking built with Go and the Bubbletea framework. This lightweight tool allows you to start and stop Harvest timers with ticket numbers and descriptions directly from your terminal.

## Features

- Select projects and tasks from your recent time entries
- Start/stop timers with ticket numbers and descriptions
- Filter projects and tasks with a simple search
- Keyboard-driven interface for quick time tracking
- Secure HTTPS/TLS connections to Harvest API

## Requirements

- Go 1.16 or higher
- Harvest API token with timer permissions
- Terminal with TUI support

## Installation

1. Clone this repository:

```sh
git clone https://github.com/yourusername/harvest-tui
cd harvest-tui
```

2. Install dependencies:

```sh
go mod init harvest-tui
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/bubbles/list
go get github.com/charmbracelet/bubbles/textinput
go get github.com/charmbracelet/lipgloss
go get github.com/go-resty/resty/v2
go mod tidy
```

3. Build the application:

```sh
go build -o harvest-tui .
```

## Setting up Harvest API Token

1. Go to [Harvest Developer Tools](https://id.getharvest.com/developers)
2. Create a new personal access token
3. Note your Account ID and Token

## Usage

1. Set your Harvest API credentials as environment variables:

```sh
export HARVEST_ACCOUNT_ID=your-account-id
export HARVEST_ACCESS_TOKEN=your-token
```

2. Run the application:

```sh
./harvest-tui
```

3. Simple workflow:
   - Select a project (use ↑/↓ to navigate, / to filter)
   - Select a task
   - Enter ticket number and description (e.g., "TICKET-123 - Add new feature")
   - Press Enter to start/stop timer

## Keyboard Shortcuts

- `↑/↓`: Navigate through options
- `/`: Filter the list (start typing to search)
- `Enter`: Select project/task or start/stop timer
- `Esc`: Go back to previous screen
- `?`: Show/hide help
- `q` or `Ctrl+C`: Quit the application

## Security Features

- HTTPS/TLS for all API communications
- Certificate validation to prevent man-in-the-middle attacks
- Credentials stored in environment variables, not in code

## References

- [Harvest API Documentation](https://help.getharvest.com/api-v2/)
- [Bubbletea TUI Framework](https://github.com/charmbracelet/bubbletea)
- [Go Resty HTTP Client](https://github.com/go-resty/resty)
- [Other Harvest CLI Tools](https://github.com/jamesburns-rts/harvest-go-cli)

## License

MIT
