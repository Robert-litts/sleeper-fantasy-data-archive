package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Robert-litts/sleeper-fantasy-data-archive/internal/archive"
	"github.com/Robert-litts/sleeper-fantasy-data-archive/internal/config"
	"github.com/Robert-litts/sleeper-fantasy-data-archive/internal/db"
	"github.com/Robert-litts/sleeper-fantasy-data-archive/internal/postgres"
	"github.com/Robert-litts/sleeper-fantasy-data-archive/internal/sleeper"
)

func main() {
	mode := flag.String("mode", "inspect", "Mode to run: inspect, report, players, leagues, teams, drafts, matchups, weekly-rosters, brackets, backfill-basic, state")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	client := sleeper.NewClient(cfg.SleeperBaseURL, cfg.HTTPTimeout)
	ctx := context.Background()

	dbConn, err := postgres.Open(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer dbConn.Close()

	svc := archive.NewService(cfg, client, db.New(dbConn))

	switch *mode {
	case "inspect":
		plans, err := svc.BuildBackfillPlan(ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, plan := range plans {
			fmt.Printf("season %d: %d leagues\n", plan.Season, len(plan.Leagues))
			for _, league := range plan.Leagues {
				fmt.Printf("  - %s (%s)\n", league.Name, league.LeagueID)
			}
		}
	case "report":
		reports, err := svc.LeagueReports(ctx)
		if err != nil {
			log.Fatal(err)
		}
		printLeagueReports(reports)
	case "players":
		count, err := svc.ArchivePlayers(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("archived %d players\n", count)
	case "leagues":
		count, err := svc.ArchiveLeagues(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("archived %d leagues\n", count)
	case "teams":
		teamCount, rosterEntryCount, err := svc.ArchiveTeamsAndRosters(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("archived %d teams and %d roster entries\n", teamCount, rosterEntryCount)
	case "drafts":
		count, err := svc.ArchiveDrafts(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("archived %d draft picks\n", count)
	case "matchups":
		count, err := svc.ArchiveMatchups(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("archived %d matchup entries\n", count)
	case "weekly-rosters":
		count, err := svc.ArchiveWeeklyRosters(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("archived %d weekly roster entries\n", count)
	case "brackets":
		count, err := svc.ArchivePlayoffBrackets(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("archived %d playoff bracket matchups\n", count)
	case "backfill-basic":
		playerCount, leagueCount, teamCount, rosterEntryCount, draftCount, matchupCount, weeklyRosterCount, bracketCount, err := svc.ArchiveBasic(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("archived %d players, %d leagues, %d teams, %d roster entries, %d draft picks, %d matchup entries, %d weekly roster entries, and %d playoff bracket matchups\n", playerCount, leagueCount, teamCount, rosterEntryCount, draftCount, matchupCount, weeklyRosterCount, bracketCount)
	case "state":
		state, err := client.GetState(ctx, cfg.SleeperSport)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("state keys: %d\n", len(state))
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *mode)
		os.Exit(1)
	}
}

func printLeagueReports(reports []db.ListLeagueReportsRow) {
	if len(reports) == 0 {
		fmt.Println("No archived leagues found.")
		return
	}

	for _, report := range reports {
		fmt.Printf("%d %s\n", report.Season, report.Name)
		fmt.Printf("  teams: %d/%d\n", report.TeamCount, report.TotalRosters)
		fmt.Printf("  draft picks: %d\n", report.DraftPickCount)
		fmt.Printf("  matchup weeks: %s\n", weekRange(report.MatchupMinWeek, report.MatchupMaxWeek))
		fmt.Printf("  matchup entries: %d\n", report.MatchupEntryCount)
		fmt.Printf("  current roster entries: %d\n", report.CurrentRosterEntryCount)
		fmt.Printf("  weekly roster entries: %d\n", report.WeeklyRosterEntryCount)
		fmt.Printf("  playoff bracket matchups: %d\n", report.PlayoffBracketMatchupCount)
		fmt.Printf("  champion: %s\n", valueOrPlaceholder(report.ChampionTeamName))
		fmt.Printf("  runner-up: %s\n", valueOrPlaceholder(report.RunnerUpTeamName))

		warnings := reportWarnings(report)
		if len(warnings) > 0 {
			fmt.Println("  warnings:")
			for _, warning := range warnings {
				fmt.Printf("    - %s\n", warning)
			}
		}
		fmt.Println()
	}
}

func weekRange(minWeek, maxWeek int32) string {
	if minWeek == 0 || maxWeek == 0 {
		return "none"
	}
	if minWeek == maxWeek {
		return fmt.Sprintf("%d", minWeek)
	}
	return fmt.Sprintf("%d-%d", minWeek, maxWeek)
}

func valueOrPlaceholder(value string) string {
	if value == "" {
		return "(missing)"
	}
	return value
}

func reportWarnings(report db.ListLeagueReportsRow) []string {
	var warnings []string
	if report.TeamCount != int64(report.TotalRosters) {
		warnings = append(warnings, fmt.Sprintf("team count does not match Sleeper total rosters: %d/%d", report.TeamCount, report.TotalRosters))
	}
	if report.MatchupEntryCount == 0 {
		warnings = append(warnings, "no matchup entries archived")
	}
	if report.DraftPickCount == 0 {
		warnings = append(warnings, "no draft picks archived")
	}
	if report.WeeklyRosterEntryCount == 0 {
		warnings = append(warnings, "no weekly roster entries archived")
	}
	if report.PlayoffBracketMatchupCount == 0 {
		warnings = append(warnings, "no playoff bracket matchups archived")
	}
	if report.ChampionTeamName == "" {
		warnings = append(warnings, "champion is missing")
	}
	if report.RunnerUpTeamName == "" {
		warnings = append(warnings, "runner-up is missing")
	}
	if report.MissingFinalStandingCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d teams have missing final standings", report.MissingFinalStandingCount))
	}
	if report.DuplicateFinalStandingCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d duplicated final standing values found", report.DuplicateFinalStandingCount))
	}
	return warnings
}
