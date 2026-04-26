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
	mode := flag.String("mode", "inspect", "Mode to run: inspect, players, leagues, teams, matchups, backfill-basic, state")
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
	case "matchups":
		count, err := svc.ArchiveMatchups(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("archived %d matchup entries\n", count)
	case "backfill-basic":
		playerCount, leagueCount, teamCount, rosterEntryCount, matchupCount, err := svc.ArchiveBasic(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("archived %d players, %d leagues, %d teams, %d roster entries, and %d matchup entries\n", playerCount, leagueCount, teamCount, rosterEntryCount, matchupCount)
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
