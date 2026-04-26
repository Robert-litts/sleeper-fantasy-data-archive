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

	return count, nil
}

func (s *Service) ArchiveBasic(ctx context.Context) (int, int, int, int, int, error) {
	playerCount, err := s.ArchivePlayers(ctx)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	leagueCount, err := s.ArchiveLeagues(ctx)
	if err != nil {
		return playerCount, 0, 0, 0, 0, err
	}

	teamCount, rosterEntryCount, err := s.ArchiveTeamsAndRosters(ctx)
	if err != nil {
		return playerCount, leagueCount, teamCount, rosterEntryCount, 0, err
	}

	matchupCount, err := s.ArchiveMatchups(ctx)
	if err != nil {
		return playerCount, leagueCount, teamCount, rosterEntryCount, matchupCount, err
	}

	return playerCount, leagueCount, teamCount, rosterEntryCount, matchupCount, nil
}

func upsertLeagueParams(league sleeper.League, fallbackSeason int) db.UpsertLeagueParams {
	season := fallbackSeason
	if parsed, err := strconv.Atoi(league.Season); err == nil {
		season = parsed
	}

	return db.UpsertLeagueParams{
		SleeperLeagueID: league.LeagueID,
		Season:          int32(season),
		Name:            league.Name,
		Status:          league.Status,
		Sport:           valueOrDefault(league.Sport, "nfl"),
		TotalRosters:    int32(league.TotalRosters),
		DraftID:         nullString(league.DraftID),
		Avatar:          nullString(league.Avatar),
		RosterPositions: mustJSON(league.RosterPositions, []byte("[]")),
		ScoringSettings: mustRawJSON(league.ScoringSettings, []byte("{}")),
		LeagueSettings:  mustRawJSON(league.Settings, []byte("{}")),
	}
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

func (s *Service) archiveRosterEntries(ctx context.Context, leagueID int64, roster sleeper.Roster) (int, error) {
	starters := make(map[string]bool, len(roster.Starters))
	for _, sleeperPlayerID := range roster.Starters {
		starters[sleeperPlayerID] = true
	}

	count := 0
	for _, sleeperPlayerID := range roster.Players {
		if sleeperPlayerID == "" {
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
