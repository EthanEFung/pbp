/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const todaysScoreboardURL = "http://cdn.nba.com/static/json/liveData/scoreboard/todaysScoreboard_00.json"
const playByPlayTemplateURL = "http://cdn.nba.com/static/json/liveData/playbyplay/playbyplay_%s.json"
const boxScoreTemplateURL = "http://cdn.nba.com/static/json/liveData/boxscore/boxscore_%s.json"

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		// b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1).MarginTop(1)
	}()
)

type tick time.Time

type todaysGame struct {
	GameId   string
	GameCode string
}

type todays struct {
	Scoreboard struct {
		Games []todaysGame
	}
}

type gameAction struct {
	TeamTricode string
	Description string
}

type gamePBP struct {
	Game struct {
		GameId  string
		Actions []gameAction
	}
}

type boxscoreTeam struct {
	TeamTricode string
	Score       int
}

type gameBoxscore struct {
	Game struct {
		GameId         string
		GameStatus     int
		GameStatusText string
		HomeTeam       boxscoreTeam
		AwayTeam       boxscoreTeam
	}
}

type model struct {
	ready      bool
	scoreURL   string
	pbpURL     string
	pbp        gamePBP
	header string
	viewport   viewport.Model
	lastUpdate time.Time
}

func newModel(tricode string) (model, error) {
	pbpURL, scoreURL, err := deriveURLs(tricode, todaysScoreboardURL)

	if err != nil {
		return model{}, err
	}
	if scoreURL == todaysScoreboardURL {
		return model{
			scoreURL: scoreURL,
			pbpURL:   scoreURL,
		}, nil
	}

	return model{
		scoreURL: scoreURL,
		pbpURL:   pbpURL,
	}, nil
}

func tickEvery() tea.Cmd {
	return tea.Every(time.Second*10, func(t time.Time) tea.Msg {
		return tick(t)
	})
}

func getPBP(m model) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(m.pbpURL)
		if err != nil {
			return tea.Quit
		}
		if resp.StatusCode != 200 {
			return tea.Quit
		}
		blob, err := io.ReadAll(resp.Body)
		if err != nil {
			return tea.Quit
		}
		var pbp gamePBP
		json.Unmarshal(blob, &pbp)
		return pbp
	}
}

func getBoxscore(m model) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(m.scoreURL)
		if err != nil {
			return tea.Quit
		}
		if resp.StatusCode != 200 {
			return tea.Quit
		}
		blob, err := io.ReadAll(resp.Body)
		if err != nil {
			return tea.Quit
		}
		var boxscore gameBoxscore
		json.Unmarshal(blob, &boxscore)
		return boxscore
	}
}

// yup its ugly, but will work for now
func deriveURLs(tricode, defaultUrl string) (string, string, error) {
	if len(tricode) != 3 {
		return defaultUrl, defaultUrl, nil
	}

	response, err := http.Get(todaysScoreboardURL)
	if err != nil {
		return "", "", fmt.Errorf("Issue found grabbing todays games: %v", err)
	}

	blob, err := io.ReadAll(response.Body)
	if err != nil {
		return "", "", fmt.Errorf("Issue reading response body from games: %v", err)
	}

	var t todays
	json.Unmarshal(blob, &t)

	for _, game := range t.Scoreboard.Games {
		if strings.Contains(game.GameCode, strings.ToUpper(tricode)) {
			teamPlayByPlayURL := fmt.Sprintf(playByPlayTemplateURL, game.GameId)
			teamBoxscoreURL := fmt.Sprintf(boxScoreTemplateURL, game.GameId)
			return teamPlayByPlayURL, teamBoxscoreURL, nil
		}
	}

	return defaultUrl, defaultUrl, nil
}


func buildContent(gamePBP gamePBP) string {
	b := strings.Builder{}
	for _, action := range gamePBP.Game.Actions {
		b.WriteString(action.TeamTricode + ": " + action.Description)
		b.WriteRune('\n')
	}
	return b.String()
}

func buildHeader(boxscore gameBoxscore) string {
	b := strings.Builder{}
	b.WriteString(boxscore.Game.GameStatusText + " | ")
	b.WriteString(boxscore.Game.HomeTeam.TeamTricode + " ")
	b.WriteString(strconv.Itoa(boxscore.Game.HomeTeam.Score) + " <> ")
	b.WriteString(boxscore.Game.AwayTeam.TeamTricode + " ")
	b.WriteString(strconv.Itoa(boxscore.Game.AwayTeam.Score))
	return b.String()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	case tick:
		m.lastUpdate = time.Now()
		return m, tea.Batch(getPBP(m), getBoxscore(m), tickEvery())
	case gamePBP:
		m.lastUpdate = time.Now()
		m.pbp = msg
		content := buildContent(msg)
		m.viewport.SetContent(content)
		m.viewport.ViewDown()
	case gameBoxscore:
		m.lastUpdate = time.Now()
		m.header = buildHeader(msg)
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height-headerHeight
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) Init() tea.Cmd {
	return tea.Batch(getPBP(m), getBoxscore(m), tickEvery())
}

func (m model) View() string {
	if m.pbp.Game.GameId == "" {
		return "requesting from " + m.pbpURL
	}
	return fmt.Sprintf("%s\n%s\n", m.headerView(), m.viewport.View())
}

func (m model) headerView() string {
	return titleStyle.Render(m.header)
}

// watchCmd represents the watch command
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		m, err := newModel(args[0])
		if err != nil {
			fmt.Printf("Error parsing new model: %v", err)
			os.Exit(1)
		}
		p := tea.NewProgram(
			m,
			tea.WithAltScreen(),
			tea.WithMouseAllMotion(),
		)
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error booting up bubbletea: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// watchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// watchCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
