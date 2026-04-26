package archive

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Robert-litts/sleeper-fantasy-data-archive/internal/config"
	"github.com/Robert-litts/sleeper-fantasy-data-archive/internal/db"
	"github.com/Robert-litts/sleeper-fantasy-data-archive/internal/sleeper"
)

type SeasonPlan struct {
	Season  int
	Leagues []sleeper.League
}

type leagueSettings struct {
	PlayoffWeekStart int `json:"playoff_week_start"`
}

const maxMatchupWeekProbeLimit = 30

type Service struct {
	cfg     config.Config
	client  *sleeper.Client
	queries *db.Queries
}

func NewService(cfg config.Config, client *sleeper.Client, queries *db.Queries) *Service {
	return &Service{cfg: cfg, client: client, queries: queries}
}

func (s *Service) BuildBackfillPlan(ctx context.Context) ([]SeasonPlan, error) {
	plans := make([]SeasonPlan, 0, s.cfg.EndSeason-s.cfg.StartSeason+1)
	for season := s.cfg.StartSeason; season <= s.cfg.EndSeason; season++ {
		leagues, err := s.client.GetUserLeagues(ctx, s.cfg.SleeperUserID, s.cfg.SleeperSport, season)
		if err != nil {
			return nil, fmt.Errorf("fetch leagues for season %d: %w", season, err)
		}

		plans = append(plans, SeasonPlan{
			Season:  season,
			Leagues: leagues,
		})
	}

	return plans, nil
}

func (s *Service) LoadPlayers(ctx context.Context) (int, error) {
	players, err := s.client.GetPlayers(ctx, s.cfg.SleeperSport)
	if err != nil {
		return 0, err
	}
	return len(players), nil
}

func (s *Service) LeagueReports(ctx context.Context) ([]db.ListLeagueReportsRow, error) {
	if s.queries == nil {
		return nil, fmt.Errorf("database queries are not configured")
	}
	return s.queries.ListLeagueReports(ctx)
}

func (s *Service) ArchivePlayers(ctx context.Context) (int, error) {
	if s.queries == nil {
		return 0, fmt.Errorf("database queries are not configured")
	}

	players, err := s.client.GetPlayers(ctx, s.cfg.SleeperSport)
	if err != nil {
		return 0, err
	}

	count := 0
	for sleeperID, player := range players {
		if player.PlayerID == "" {
			player.PlayerID = sleeperID
		}
		if player.PlayerID == "" {
			continue
		}

		if _, err := s.queries.UpsertPlayer(ctx, upsertPlayerParams(player)); err != nil {
			return count, fmt.Errorf("upsert player %s: %w", player.PlayerID, err)
		}
		count++
	}

	return count, nil
}

func (s *Service) ArchiveLeagues(ctx context.Context) (int, error) {
	if s.queries == nil {
		return 0, fmt.Errorf("database queries are not configured")
	}

	plans, err := s.BuildBackfillPlan(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, plan := range plans {
		for _, league := range plan.Leagues {
			if league.LeagueID == "" {
				continue
			}

			if _, err := s.queries.UpsertLeague(ctx, upsertLeagueParams(league, plan.Season)); err != nil {
				return count, fmt.Errorf("upsert league %s: %w", league.LeagueID, err)
			}
			count++
		}
	}

	if err := s.syncLeagueLineage(ctx); err != nil {
		return count, err
	}

	return count, nil
}

func (s *Service) ArchiveTeamsAndRosters(ctx context.Context) (int, int, error) {
	if s.queries == nil {
		return 0, 0, fmt.Errorf("database queries are not configured")
	}

	plans, err := s.BuildBackfillPlan(ctx)
	if err != nil {
		return 0, 0, err
	}

	teamCount := 0
	rosterEntryCount := 0
	for _, plan := range plans {
		for _, league := range plan.Leagues {
			if league.LeagueID == "" {
				continue
			}

			dbLeague, err := s.queries.UpsertLeague(ctx, upsertLeagueParams(league, plan.Season))
			if err != nil {
				return teamCount, rosterEntryCount, fmt.Errorf("upsert league %s: %w", league.LeagueID, err)
			}

			users, err := s.client.GetLeagueUsers(ctx, league.LeagueID)
			if err != nil {
				return teamCount, rosterEntryCount, fmt.Errorf("fetch users for league %s: %w", league.LeagueID, err)
			}

			rosters, err := s.client.GetLeagueRosters(ctx, league.LeagueID)
			if err != nil {
				return teamCount, rosterEntryCount, fmt.Errorf("fetch rosters for league %s: %w", league.LeagueID, err)
			}

			usersByID := mapUsersByID(users)
			standings := rankRosters(rosters)
			for _, roster := range rosters {
				user := usersByID[roster.OwnerID]
				if _, err := s.queries.UpsertTeam(ctx, upsertTeamParams(dbLeague.ID, roster, user, standings[roster.RosterID])); err != nil {
					return teamCount, rosterEntryCount, fmt.Errorf("upsert team league %s roster %d: %w", league.LeagueID, roster.RosterID, err)
				}
				teamCount++

				inserted, err := s.archiveRosterEntries(ctx, dbLeague.ID, roster)
				if err != nil {
					return teamCount, rosterEntryCount, fmt.Errorf("archive roster entries league %s roster %d: %w", league.LeagueID, roster.RosterID, err)
				}
				rosterEntryCount += inserted
			}
		}
	}

	if err := s.syncLeagueLineage(ctx); err != nil {
		return teamCount, rosterEntryCount, err
	}

	return teamCount, rosterEntryCount, nil
}

func (s *Service) ArchiveMatchups(ctx context.Context) (int, error) {
	if s.queries == nil {
		return 0, fmt.Errorf("database queries are not configured")
	}

	plans, err := s.BuildBackfillPlan(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, plan := range plans {
		for _, league := range plan.Leagues {
			if league.LeagueID == "" {
				continue
			}

			dbLeague, err := s.queries.UpsertLeague(ctx, upsertLeagueParams(league, plan.Season))
			if err != nil {
				return count, fmt.Errorf("upsert league %s: %w", league.LeagueID, err)
			}

			settings := parseLeagueSettings(league.Settings)
			for week := 1; ; week++ {
				if week > maxMatchupWeekProbeLimit {
					return count, fmt.Errorf("stopped matchup probing for league %s after %d weeks without finding an empty response", league.LeagueID, maxMatchupWeekProbeLimit)
				}

				matchups, err := s.client.GetLeagueMatchups(ctx, league.LeagueID, week)
				if err != nil {
					return count, fmt.Errorf("fetch matchups league %s week %d: %w", league.LeagueID, week, err)
				}
				if len(matchups) == 0 {
					break
				}

				opponents := matchupOpponents(matchups)
				for _, matchup := range matchups {
					if matchup.RosterID == 0 {
						continue
					}

					isPlayoff := settings.PlayoffWeekStart > 0 && week >= settings.PlayoffWeekStart
					matchupType := "REGULAR"
					if isPlayoff {
						matchupType = "WINNERS_BRACKET"
					}

					if _, err := s.queries.UpsertMatchup(ctx, db.UpsertMatchupParams{
						LeagueID:         dbLeague.ID,
						Week:             int32(week),
						MatchupID:        int32(normalizedMatchupID(matchup)),
						RosterID:         int32(matchup.RosterID),
						OpponentRosterID: nullIntValue(opponents[matchup.RosterID]),
						Points:           formatPoints(matchup.Points),
						CustomPoints:     nullFloat(matchup.CustomPoints),
						IsPlayoff:        isPlayoff,
						MatchupType:      matchupType,
						Starters:         mustJSON(matchup.Starters, []byte("[]")),
						Players:          mustJSON(matchup.Players, []byte("[]")),
					}); err != nil {
						return count, fmt.Errorf("upsert matchup league %s week %d roster %d: %w", league.LeagueID, week, matchup.RosterID, err)
					}
					count++
				}
			}
		}
	}

	if err := s.syncLeagueLineage(ctx); err != nil {
		return count, err
	}

	return count, nil
}

func (s *Service) ArchivePlayoffBrackets(ctx context.Context) (int, error) {
	if s.queries == nil {
		return 0, fmt.Errorf("database queries are not configured")
	}

	plans, err := s.BuildBackfillPlan(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, plan := range plans {
		for _, league := range plan.Leagues {
			if league.LeagueID == "" {
				continue
			}

			dbLeague, err := s.queries.UpsertLeague(ctx, upsertLeagueParams(league, plan.Season))
			if err != nil {
				return count, fmt.Errorf("upsert league %s: %w", league.LeagueID, err)
			}

			if err := s.queries.ResetLeagueFinalStandings(ctx, dbLeague.ID); err != nil {
				return count, fmt.Errorf("reset final standings league %s: %w", league.LeagueID, err)
			}

			inserted, err := s.archiveBracket(ctx, dbLeague.ID, league.LeagueID, "WINNERS_BRACKET", s.client.GetLeagueWinnersBracket)
			if err != nil {
				return count, err
			}
			count += inserted

			inserted, err = s.archiveBracket(ctx, dbLeague.ID, league.LeagueID, "LOSERS_BRACKET", s.client.GetLeagueLosersBracket)
			if err != nil {
				return count, err
			}
			count += inserted
		}
	}

	if err := s.syncLeagueLineage(ctx); err != nil {
		return count, err
	}

	return count, nil
}

func (s *Service) ArchiveDrafts(ctx context.Context) (int, error) {
	if s.queries == nil {
		return 0, fmt.Errorf("database queries are not configured")
	}

	plans, err := s.BuildBackfillPlan(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, plan := range plans {
		for _, league := range plan.Leagues {
			if league.LeagueID == "" {
				continue
			}

			dbLeague, err := s.queries.UpsertLeague(ctx, upsertLeagueParams(league, plan.Season))
			if err != nil {
				return count, fmt.Errorf("upsert league %s: %w", league.LeagueID, err)
			}

			if _, _, err := s.archiveTeamsForLeague(ctx, dbLeague.ID, league.LeagueID); err != nil {
				return count, fmt.Errorf("archive teams league %s: %w", league.LeagueID, err)
			}

			drafts, err := s.client.GetLeagueDrafts(ctx, league.LeagueID)
			if err != nil {
				return count, fmt.Errorf("fetch drafts league %s: %w", league.LeagueID, err)
			}

			for _, draft := range drafts {
				if draft.DraftID == "" {
					continue
				}

				picks, err := s.client.GetDraftPicks(ctx, draft.DraftID)
				if err != nil {
					return count, fmt.Errorf("fetch draft picks draft %s: %w", draft.DraftID, err)
				}

				for _, pick := range picks {
					if pick.PickNo == 0 || pick.PlayerID == "" {
						continue
					}

					params, err := s.upsertDraftPickParams(ctx, dbLeague.ID, league.TotalRosters, draft.DraftID, pick)
					if err != nil {
						return count, fmt.Errorf("map draft pick draft %s pick %d: %w", draft.DraftID, pick.PickNo, err)
					}

					if _, err := s.queries.UpsertDraftPick(ctx, params); err != nil {
						return count, fmt.Errorf("upsert draft pick draft %s pick %d: %w", draft.DraftID, pick.PickNo, err)
					}
					count++
				}
			}
		}
	}

	if err := s.syncLeagueLineage(ctx); err != nil {
		return count, err
	}

	return count, nil
}

func (s *Service) ArchiveWeeklyRosters(ctx context.Context) (int, error) {
	if s.queries == nil {
		return 0, fmt.Errorf("database queries are not configured")
	}

	plans, err := s.BuildBackfillPlan(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, plan := range plans {
		for _, league := range plan.Leagues {
			if league.LeagueID == "" {
				continue
			}

			dbLeague, err := s.queries.UpsertLeague(ctx, upsertLeagueParams(league, plan.Season))
			if err != nil {
				return count, fmt.Errorf("upsert league %s: %w", league.LeagueID, err)
			}

			for week := 1; ; week++ {
				if week > maxMatchupWeekProbeLimit {
					return count, fmt.Errorf("stopped weekly roster probing for league %s after %d weeks without finding an empty response", league.LeagueID, maxMatchupWeekProbeLimit)
				}

				matchups, err := s.client.GetLeagueMatchups(ctx, league.LeagueID, week)
				if err != nil {
					return count, fmt.Errorf("fetch matchups league %s week %d: %w", league.LeagueID, week, err)
				}
				if len(matchups) == 0 {
					break
				}

				for _, matchup := range matchups {
					if matchup.RosterID == 0 {
						continue
					}

					inserted, err := s.archiveWeeklyRosterEntries(ctx, dbLeague.ID, week, matchup)
					if err != nil {
						return count, fmt.Errorf("archive weekly roster league %s week %d roster %d: %w", league.LeagueID, week, matchup.RosterID, err)
					}
					count += inserted
				}
			}
		}
	}

	if err := s.syncLeagueLineage(ctx); err != nil {
		return count, err
	}

	return count, nil
}

func (s *Service) ArchiveBasic(ctx context.Context) (int, int, int, int, int, int, int, int, error) {
	playerCount, err := s.ArchivePlayers(ctx)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, err
	}

	leagueCount, err := s.ArchiveLeagues(ctx)
	if err != nil {
		return playerCount, 0, 0, 0, 0, 0, 0, 0, err
	}

	teamCount, rosterEntryCount, err := s.ArchiveTeamsAndRosters(ctx)
	if err != nil {
		return playerCount, leagueCount, teamCount, rosterEntryCount, 0, 0, 0, 0, err
	}

	draftCount, err := s.ArchiveDrafts(ctx)
	if err != nil {
		return playerCount, leagueCount, teamCount, rosterEntryCount, draftCount, 0, 0, 0, err
	}

	matchupCount, err := s.ArchiveMatchups(ctx)
	if err != nil {
		return playerCount, leagueCount, teamCount, rosterEntryCount, draftCount, matchupCount, 0, 0, err
	}

	weeklyRosterCount, err := s.ArchiveWeeklyRosters(ctx)
	if err != nil {
		return playerCount, leagueCount, teamCount, rosterEntryCount, draftCount, matchupCount, weeklyRosterCount, 0, err
	}

	bracketCount, err := s.ArchivePlayoffBrackets(ctx)
	if err != nil {
		return playerCount, leagueCount, teamCount, rosterEntryCount, draftCount, matchupCount, weeklyRosterCount, bracketCount, err
	}

	return playerCount, leagueCount, teamCount, rosterEntryCount, draftCount, matchupCount, weeklyRosterCount, bracketCount, nil
}

type bracketFetcher func(context.Context, string) ([]sleeper.BracketMatchup, error)

func upsertLeagueParams(league sleeper.League, fallbackSeason int) db.UpsertLeagueParams {
	season := fallbackSeason
	if parsed, err := strconv.Atoi(league.Season); err == nil {
		season = parsed
	}

	return db.UpsertLeagueParams{
		SleeperLeagueID:   league.LeagueID,
		PreviousLeagueID:  nullString(league.PreviousLeagueID),
		CanonicalLeagueID: sql.NullString{},
		Season:            int32(season),
		Name:              league.Name,
		Status:            league.Status,
		Sport:             valueOrDefault(league.Sport, "nfl"),
		TotalRosters:      int32(league.TotalRosters),
		DraftID:           nullString(league.DraftID),
		Avatar:            nullString(league.Avatar),
		RosterPositions:   mustJSON(league.RosterPositions, []byte("[]")),
		ScoringSettings:   mustRawJSON(league.ScoringSettings, []byte("{}")),
		LeagueSettings:    mustRawJSON(league.Settings, []byte("{}")),
	}
}

func (s *Service) syncLeagueLineage(ctx context.Context) error {
	seedLeagueID := strings.TrimSpace(s.cfg.SleeperMainLeagueID)
	if seedLeagueID == "" || s.queries == nil {
		return nil
	}

	seen := make(map[string]struct{})
	currentLeagueID := seedLeagueID
	for currentLeagueID != "" {
		if _, ok := seen[currentLeagueID]; ok {
			return fmt.Errorf("detected league lineage cycle at %s", currentLeagueID)
		}
		seen[currentLeagueID] = struct{}{}

		league, err := s.queries.GetLeagueBySleeperID(ctx, currentLeagueID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return fmt.Errorf("load league %s: %w", currentLeagueID, err)
		}

		if _, err := s.queries.UpdateLeagueCanonicalLeagueID(ctx, db.UpdateLeagueCanonicalLeagueIDParams{
			SleeperLeagueID:   currentLeagueID,
			CanonicalLeagueID: sql.NullString{String: seedLeagueID, Valid: true},
		}); err != nil {
			return fmt.Errorf("update canonical league id for %s: %w", currentLeagueID, err)
		}

		if !league.PreviousLeagueID.Valid {
			return nil
		}

		currentLeagueID = strings.TrimSpace(league.PreviousLeagueID.String)
	}

	return nil
}

func upsertPlayerParams(player sleeper.Player) db.UpsertPlayerParams {
	name := strings.TrimSpace(player.SearchFullName)
	if name == "" {
		name = strings.TrimSpace(player.FirstName + " " + player.LastName)
	}
	if name == "" {
		name = player.PlayerID
	}

	return db.UpsertPlayerParams{
		SleeperID:        player.PlayerID,
		EspnID:           nullFlexibleString(player.ESPNID),
		Name:             name,
		FirstName:        nullString(player.FirstName),
		LastName:         nullString(player.LastName),
		Position:         nullString(player.Position),
		Team:             nullString(player.Team),
		Status:           nullString(player.Status),
		FantasyPositions: mustJSON(player.FantasyPositions, []byte("[]")),
		Age:              nullFlexibleInt(player.Age),
	}
}

func upsertTeamParams(leagueID int64, roster sleeper.Roster, user sleeper.User, standing int) db.UpsertTeamParams {
	teamName := strings.TrimSpace(user.Metadata["team_name"])
	if teamName == "" {
		teamName = strings.TrimSpace(user.DisplayName)
	}
	if teamName == "" {
		teamName = strings.TrimSpace(user.Username)
	}
	if teamName == "" {
		teamName = fmt.Sprintf("Roster %d", roster.RosterID)
	}

	return db.UpsertTeamParams{
		LeagueID:         leagueID,
		RosterID:         int32(roster.RosterID),
		OwnerID:          valueOrDefault(roster.OwnerID, fmt.Sprintf("roster-%d", roster.RosterID)),
		UserID:           nullString(user.UserID),
		DisplayName:      nullString(user.DisplayName),
		Username:         nullString(user.Username),
		TeamName:         teamName,
		Avatar:           nullString(user.Avatar),
		Wins:             int32(roster.Settings.Wins),
		Losses:           int32(roster.Settings.Losses),
		Ties:             int32(roster.Settings.Ties),
		PointsFor:        formatPoints(totalSleeperPoints(roster.Settings.Fpts, roster.Settings.FptsDecimal)),
		PointsAgainst:    formatPoints(totalSleeperPoints(roster.Settings.FptsAgainst, roster.Settings.FptsAgainstDecimal)),
		WaiverPosition:   int32(roster.Settings.WaiverPosition),
		WaiverBudgetUsed: int32(roster.Settings.WaiverBudgetUsed),
		TotalMoves:       int32(roster.Settings.TotalMoves),
		StreakType:       "",
		StreakLength:     0,
		Standing:         int32(standing),
		FinalStanding:    int32(standing),
	}
}

func (s *Service) archiveTeamsForLeague(ctx context.Context, dbLeagueID int64, sleeperLeagueID string) (int, []sleeper.Roster, error) {
	users, err := s.client.GetLeagueUsers(ctx, sleeperLeagueID)
	if err != nil {
		return 0, nil, fmt.Errorf("fetch users: %w", err)
	}

	rosters, err := s.client.GetLeagueRosters(ctx, sleeperLeagueID)
	if err != nil {
		return 0, nil, fmt.Errorf("fetch rosters: %w", err)
	}

	usersByID := mapUsersByID(users)
	standings := rankRosters(rosters)
	count := 0
	for _, roster := range rosters {
		user := usersByID[roster.OwnerID]
		if _, err := s.queries.UpsertTeam(ctx, upsertTeamParams(dbLeagueID, roster, user, standings[roster.RosterID])); err != nil {
			return count, rosters, fmt.Errorf("upsert team roster %d: %w", roster.RosterID, err)
		}
		count++
	}

	return count, rosters, nil
}

func (s *Service) archiveRosterEntries(ctx context.Context, leagueID int64, roster sleeper.Roster) (int, error) {
	starters := make(map[string]bool, len(roster.Starters))
	for _, sleeperPlayerID := range roster.Starters {
		if validSleeperPlayerID(sleeperPlayerID) {
			starters[sleeperPlayerID] = true
		}
	}

	count := 0
	for _, sleeperPlayerID := range roster.Players {
		if !validSleeperPlayerID(sleeperPlayerID) {
			continue
		}

		player, err := s.queries.GetPlayerBySleeperID(ctx, sleeperPlayerID)
		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return count, err
		}

		slot := "bench"
		if starters[sleeperPlayerID] {
			slot = "starter"
		}

		if _, err := s.queries.UpsertRosterEntry(ctx, db.UpsertRosterEntryParams{
			LeagueID:   leagueID,
			Week:       0,
			RosterID:   int32(roster.RosterID),
			PlayerID:   player.ID,
			RosterSlot: slot,
			IsStarter:  starters[sleeperPlayerID],
		}); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

func (s *Service) archiveWeeklyRosterEntries(ctx context.Context, leagueID int64, week int, matchup sleeper.Matchup) (int, error) {
	starters := make(map[string]bool, len(matchup.Starters))
	for _, sleeperPlayerID := range matchup.Starters {
		if validSleeperPlayerID(sleeperPlayerID) {
			starters[sleeperPlayerID] = true
		}
	}

	playerIDs := append([]string(nil), matchup.Players...)
	for _, sleeperPlayerID := range matchup.Starters {
		if !containsString(playerIDs, sleeperPlayerID) {
			playerIDs = append(playerIDs, sleeperPlayerID)
		}
	}

	count := 0
	for _, sleeperPlayerID := range playerIDs {
		if !validSleeperPlayerID(sleeperPlayerID) {
			continue
		}

		player, err := s.queries.GetPlayerBySleeperID(ctx, sleeperPlayerID)
		if err != nil {
			if err == sql.ErrNoRows {
				return count, fmt.Errorf("player %s is not archived; run players first", sleeperPlayerID)
			}
			return count, err
		}

		slot := "bench"
		if starters[sleeperPlayerID] {
			slot = "starter"
		}

		if _, err := s.queries.UpsertRosterEntry(ctx, db.UpsertRosterEntryParams{
			LeagueID:   leagueID,
			Week:       int32(week),
			RosterID:   int32(matchup.RosterID),
			PlayerID:   player.ID,
			RosterSlot: slot,
			IsStarter:  starters[sleeperPlayerID],
		}); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

func (s *Service) upsertDraftPickParams(ctx context.Context, leagueID int64, totalRosters int, draftID string, pick sleeper.DraftPick) (db.UpsertDraftPickParams, error) {
	rosterID, err := strconv.Atoi(pick.RosterID.Value)
	if err != nil || rosterID == 0 {
		return db.UpsertDraftPickParams{}, fmt.Errorf("invalid roster id %q", pick.RosterID.Value)
	}

	team, err := s.queries.GetTeamByLeagueAndRoster(ctx, db.GetTeamByLeagueAndRosterParams{
		LeagueID: leagueID,
		RosterID: int32(rosterID),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return db.UpsertDraftPickParams{}, fmt.Errorf("team roster %d is not archived; run teams first", rosterID)
		}
		return db.UpsertDraftPickParams{}, fmt.Errorf("find team roster %d: %w", rosterID, err)
	}

	player, err := s.queries.GetPlayerBySleeperID(ctx, pick.PlayerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return db.UpsertDraftPickParams{}, fmt.Errorf("player %s is not archived; run players first", pick.PlayerID)
		}
		return db.UpsertDraftPickParams{}, fmt.Errorf("find player %s: %w", pick.PlayerID, err)
	}

	roundPick := pick.PickNo
	if totalRosters > 0 && pick.Round > 0 {
		roundPick = pick.PickNo - ((pick.Round - 1) * totalRosters)
	}

	return db.UpsertDraftPickParams{
		LeagueID:     leagueID,
		DraftID:      draftID,
		TeamID:       team.ID,
		PlayerID:     player.ID,
		OverallPick:  int32(pick.PickNo),
		RoundNum:     int32(pick.Round),
		RoundPick:    int32(roundPick),
		DraftSlot:    nullIntValue(pick.DraftSlot),
		PickedBy:     nullFlexibleString(pick.PickedBy),
		KeeperStatus: boolValue(pick.IsKeeper),
	}, nil
}

func (s *Service) archiveBracket(ctx context.Context, dbLeagueID int64, sleeperLeagueID, bracketType string, fetch bracketFetcher) (int, error) {
	bracketMatchups, err := fetch(ctx, sleeperLeagueID)
	if err != nil {
		return 0, fmt.Errorf("fetch %s league %s: %w", strings.ToLower(bracketType), sleeperLeagueID, err)
	}

	count := 0
	for _, matchup := range bracketMatchups {
		if matchup.MatchupID == 0 {
			continue
		}

		if _, err := s.queries.UpsertPlayoffBracketMatchup(ctx, upsertPlayoffBracketMatchupParams(dbLeagueID, bracketType, matchup)); err != nil {
			return count, fmt.Errorf("upsert %s league %s matchup %d: %w", strings.ToLower(bracketType), sleeperLeagueID, matchup.MatchupID, err)
		}

		if bracketType == "WINNERS_BRACKET" && isCompletedPlacementMatchup(matchup) {
			if err := s.updateFinalStandingsFromBracketMatchup(ctx, dbLeagueID, matchup); err != nil {
				return count, fmt.Errorf("update final standings league %s matchup %d: %w", sleeperLeagueID, matchup.MatchupID, err)
			}
		}

		count++
	}

	return count, nil
}

func isCompletedPlacementMatchup(matchup sleeper.BracketMatchup) bool {
	return matchup.Placement != nil && matchup.WinnerRosterID != nil && matchup.LoserRosterID != nil
}

func upsertPlayoffBracketMatchupParams(leagueID int64, bracketType string, matchup sleeper.BracketMatchup) db.UpsertPlayoffBracketMatchupParams {
	t1Source := mergeBracketSource(matchup.Team1.Source, matchup.Team1From)
	t2Source := mergeBracketSource(matchup.Team2.Source, matchup.Team2From)

	return db.UpsertPlayoffBracketMatchupParams{
		LeagueID:             leagueID,
		BracketType:          bracketType,
		RoundNum:             int32(matchup.Round),
		MatchupID:            int32(matchup.MatchupID),
		Placement:            nullInt(matchup.Placement),
		Slot1RosterID:        nullInt(matchup.Team1.RosterID),
		Slot2RosterID:        nullInt(matchup.Team2.RosterID),
		Slot1SourceMatchupID: nullInt(bracketSourceMatchupID(t1Source)),
		Slot1SourceResult:    nullString(bracketSourceResult(t1Source)),
		Slot2SourceMatchupID: nullInt(bracketSourceMatchupID(t2Source)),
		Slot2SourceResult:    nullString(bracketSourceResult(t2Source)),
		WinnerRosterID:       nullInt(matchup.WinnerRosterID),
		LoserRosterID:        nullInt(matchup.LoserRosterID),
		RawPayload:           mustJSON(matchup, []byte("{}")),
	}
}

func (s *Service) updateFinalStandingsFromBracketMatchup(ctx context.Context, leagueID int64, matchup sleeper.BracketMatchup) error {
	if !isCompletedPlacementMatchup(matchup) {
		return nil
	}

	if err := s.queries.UpdateTeamFinalStanding(ctx, db.UpdateTeamFinalStandingParams{
		LeagueID:      leagueID,
		RosterID:      int32(*matchup.WinnerRosterID),
		FinalStanding: int32(*matchup.Placement),
	}); err != nil {
		return err
	}

	return s.queries.UpdateTeamFinalStanding(ctx, db.UpdateTeamFinalStandingParams{
		LeagueID:      leagueID,
		RosterID:      int32(*matchup.LoserRosterID),
		FinalStanding: int32(*matchup.Placement + 1),
	})
}
func mapUsersByID(users []sleeper.User) map[string]sleeper.User {
	usersByID := make(map[string]sleeper.User, len(users))
	for _, user := range users {
		usersByID[user.UserID] = user
	}
	return usersByID
}

func rankRosters(rosters []sleeper.Roster) map[int]int {
	sorted := append([]sleeper.Roster(nil), rosters...)
	sort.Slice(sorted, func(i, j int) bool {
		left := sorted[i]
		right := sorted[j]
		if left.Settings.Wins != right.Settings.Wins {
			return left.Settings.Wins > right.Settings.Wins
		}
		leftPoints := totalSleeperPoints(left.Settings.Fpts, left.Settings.FptsDecimal)
		rightPoints := totalSleeperPoints(right.Settings.Fpts, right.Settings.FptsDecimal)
		if leftPoints != rightPoints {
			return leftPoints > rightPoints
		}
		if left.Settings.Losses != right.Settings.Losses {
			return left.Settings.Losses < right.Settings.Losses
		}
		return left.RosterID < right.RosterID
	})

	standings := make(map[int]int, len(sorted))
	for i, roster := range sorted {
		standings[roster.RosterID] = i + 1
	}
	return standings
}

func totalSleeperPoints(points, decimal float64) float64 {
	return points + decimal/100
}

func formatPoints(points float64) string {
	return strconv.FormatFloat(points, 'f', 2, 64)
}

func parseLeagueSettings(raw json.RawMessage) leagueSettings {
	var settings leagueSettings
	if len(raw) == 0 {
		return settings
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return leagueSettings{}
	}
	return settings
}

func matchupOpponents(matchups []sleeper.Matchup) map[int]int {
	byMatchupID := make(map[int][]int)
	for _, matchup := range matchups {
		if matchup.RosterID == 0 {
			continue
		}
		matchupID := normalizedMatchupID(matchup)
		byMatchupID[matchupID] = append(byMatchupID[matchupID], matchup.RosterID)
	}

	opponents := make(map[int]int)
	for _, rosterIDs := range byMatchupID {
		if len(rosterIDs) != 2 {
			continue
		}
		opponents[rosterIDs[0]] = rosterIDs[1]
		opponents[rosterIDs[1]] = rosterIDs[0]
	}
	return opponents
}

func normalizedMatchupID(matchup sleeper.Matchup) int {
	if matchup.MatchupID.Valid && matchup.MatchupID.Value != 0 {
		return matchup.MatchupID.Value
	}
	return -matchup.RosterID
}

func mergeBracketSource(primary, fallback sleeper.BracketSource) sleeper.BracketSource {
	if primary.WinnerOf != nil || primary.LoserOf != nil {
		return primary
	}
	return fallback
}

func bracketSourceMatchupID(source sleeper.BracketSource) *int {
	if source.WinnerOf != nil {
		return source.WinnerOf
	}
	return source.LoserOf
}

func bracketSourceResult(source sleeper.BracketSource) string {
	if source.WinnerOf != nil {
		return "WINNER"
	}
	if source.LoserOf != nil {
		return "LOSER"
	}
	return ""
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validSleeperPlayerID(value string) bool {
	return value != "" && value != "0"
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullFlexibleString(value sleeper.FlexibleString) sql.NullString {
	return sql.NullString{String: value.Value, Valid: value.Valid && value.Value != ""}
}

func nullInt(value *int) sql.NullInt32 {
	if value == nil {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(*value), Valid: true}
}

func nullFlexibleInt(value sleeper.FlexibleInt) sql.NullInt32 {
	return sql.NullInt32{Int32: int32(value.Value), Valid: value.Valid}
}

func nullIntValue(value int) sql.NullInt32 {
	if value == 0 {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(value), Valid: true}
}

func nullFloat(value *float64) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: formatPoints(*value), Valid: true}
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func mustJSON(value any, fallback json.RawMessage) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		return fallback
	}
	return data
}

func mustRawJSON(value json.RawMessage, fallback json.RawMessage) json.RawMessage {
	if len(value) == 0 || string(value) == "null" {
		return fallback
	}
	return value
}
