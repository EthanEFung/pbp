/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var gameStyle = lipgloss.NewStyle().
	Padding(2).
	PaddingTop(1).
	PaddingBottom(1).
	BorderStyle(lipgloss.NormalBorder())

type team struct {
	TeamTricode string
	Score       int
}

type game struct {
	GameStatus     int
	GameStatusText string
	HomeTeam       team
	AwayTeam       team
}

type response struct {
	Scoreboard struct {
		GameDate string
		Games    []game
	}
}

func formatGame(g game) string {
	b := strings.Builder{}
	b.WriteString(g.GameStatusText)
	b.WriteRune('\n')
	b.WriteString(g.HomeTeam.TeamTricode + " " + strconv.Itoa(g.HomeTeam.Score))
	b.WriteRune('\n')
	b.WriteString(g.AwayTeam.TeamTricode + " " + strconv.Itoa(g.AwayTeam.Score))
	return gameStyle.Render(b.String())
}

// todayCmd represents the today command
var todayCmd = &cobra.Command{
	Use:   "today",
	Short: "get the scores of the games that are played today",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		res, err := http.Get("http://cdn.nba.com/static/json/liveData/scoreboard/todaysScoreboard_00.json")
		if err != nil {
			fmt.Printf("failed to request for todays scoreboards: %s\n", err.Error())
		}

		defer res.Body.Close()
		blob, err := io.ReadAll(res.Body)
		if err != nil {
			fmt.Printf("failed to read body of todays scoreboards: %s\n", err.Error())
		}

		var jsonBody response
		json.Unmarshal(blob, &jsonBody)

		formattedGames := []string{}
		for _, game := range jsonBody.Scoreboard.Games {
			formattedGames = append(formattedGames, formatGame(game))
		}

		fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, formattedGames...))
	},
}

func init() {
	rootCmd.AddCommand(todayCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// todayCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// todayCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
