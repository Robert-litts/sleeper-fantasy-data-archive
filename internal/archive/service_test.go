package archive

import (
	"testing"

	"github.com/Robert-litts/sleeper-fantasy-data-archive/internal/sleeper"
)

func TestUpsertLeagueParamsIncludesLineageFields(t *testing.T) {
	params := upsertLeagueParams(sleeper.League{
		LeagueID:         "league-123",
		Season:           "2024",
		PreviousLeagueID: "league-122",
		Name:             "Main League",
		Status:           "complete",
		Sport:            "nfl",
		TotalRosters:     12,
	}, 2023)

	if params.SleeperLeagueID != "league-123" {
		t.Fatalf("SleeperLeagueID = %q, want %q", params.SleeperLeagueID, "league-123")
	}
	if !params.PreviousLeagueID.Valid || params.PreviousLeagueID.String != "league-122" {
		t.Fatalf("PreviousLeagueID = %#v, want league-122", params.PreviousLeagueID)
	}
	if params.CanonicalLeagueID.Valid {
		t.Fatalf("CanonicalLeagueID = %#v, want invalid before lineage sync", params.CanonicalLeagueID)
	}
	if params.Season != 2024 {
		t.Fatalf("Season = %d, want %d", params.Season, 2024)
	}
}
