package sleeper

import (
	"bytes"
	"encoding/json"
	"strconv"
)

type FlexibleString struct {
	Value string
	Valid bool
}

type FlexibleInt struct {
	Value int
	Valid bool
}

func (s *FlexibleString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		s.Value = ""
		s.Valid = false
		return nil
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		s.Value = asString
		s.Valid = asString != ""
		return nil
	}

	var asNumber float64
	if err := json.Unmarshal(data, &asNumber); err == nil {
		s.Value = strconv.FormatInt(int64(asNumber), 10)
		s.Valid = s.Value != ""
		return nil
	}

	s.Value = ""
	s.Valid = false
	return nil
}

func (i *FlexibleInt) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		i.Value = 0
		i.Valid = false
		return nil
	}

	var asInt int
	if err := json.Unmarshal(data, &asInt); err == nil {
		i.Value = asInt
		i.Valid = true
		return nil
	}

	var asFloat float64
	if err := json.Unmarshal(data, &asFloat); err == nil {
		i.Value = int(asFloat)
		i.Valid = true
		return nil
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		if asString == "" {
			i.Value = 0
			i.Valid = false
			return nil
		}
		parsed, err := strconv.Atoi(asString)
		if err != nil {
			i.Value = 0
			i.Valid = false
			return nil
		}
		i.Value = parsed
		i.Valid = true
		return nil
	}

	i.Value = 0
	i.Valid = false
	return nil
}

type League struct {
	TotalRosters     int             `json:"total_rosters"`
	Status           string          `json:"status"`
	Sport            string          `json:"sport"`
	Settings         json.RawMessage `json:"settings"`
	SeasonType       string          `json:"season_type"`
	Season           string          `json:"season"`
	ScoringSettings  json.RawMessage `json:"scoring_settings"`
	RosterPositions  []string        `json:"roster_positions"`
	PreviousLeagueID string          `json:"previous_league_id"`
	Name             string          `json:"name"`
	LeagueID         string          `json:"league_id"`
	DraftID          string          `json:"draft_id"`
	Avatar           string          `json:"avatar"`
}

type User struct {
	UserID      string            `json:"user_id"`
	Username    string            `json:"username"`
	DisplayName string            `json:"display_name"`
	Avatar      string            `json:"avatar"`
	Metadata    map[string]string `json:"metadata"`
	IsOwner     bool              `json:"is_owner"`
}

type RosterSettings struct {
	Wins               int     `json:"wins"`
	WaiverPosition     int     `json:"waiver_position"`
	WaiverBudgetUsed   int     `json:"waiver_budget_used"`
	TotalMoves         int     `json:"total_moves"`
	Ties               int     `json:"ties"`
	Losses             int     `json:"losses"`
	Fpts               float64 `json:"fpts"`
	FptsDecimal        float64 `json:"fpts_decimal"`
	FptsAgainst        float64 `json:"fpts_against"`
	FptsAgainstDecimal float64 `json:"fpts_against_decimal"`
}

type Roster struct {
	Starters     []string       `json:"starters"`
	Settings     RosterSettings `json:"settings"`
	RosterID     int            `json:"roster_id"`
	Reserve      []string       `json:"reserve"`
	Players      []string       `json:"players"`
	OwnerID      string         `json:"owner_id"`
	LeagueID     string         `json:"league_id"`
	MatchupID    int            `json:"matchup_id"`
	Points       float64        `json:"points"`
	CustomPoints *float64       `json:"custom_points"`
}

type Draft struct {
	Type            string          `json:"type"`
	Status          string          `json:"status"`
	StartTime       int64           `json:"start_time"`
	Sport           string          `json:"sport"`
	Settings        json.RawMessage `json:"settings"`
	SeasonType      string          `json:"season_type"`
	Season          string          `json:"season"`
	Metadata        json.RawMessage `json:"metadata"`
	LeagueID        string          `json:"league_id"`
	LastPicked      int64           `json:"last_picked"`
	LastMessageTime int64           `json:"last_message_time"`
	LastMessageID   string          `json:"last_message_id"`
	DraftOrder      json.RawMessage `json:"draft_order"`
	DraftID         string          `json:"draft_id"`
	Creators        json.RawMessage `json:"creators"`
	Created         int64           `json:"created"`
	SlotToRosterID  json.RawMessage `json:"slot_to_roster_id"`
}

type DraftPick struct {
	PlayerID  string            `json:"player_id"`
	PickedBy  string            `json:"picked_by"`
	RosterID  string            `json:"roster_id"`
	Round     int               `json:"round"`
	DraftSlot int               `json:"draft_slot"`
	PickNo    int               `json:"pick_no"`
	Metadata  map[string]string `json:"metadata"`
	IsKeeper  *bool             `json:"is_keeper"`
	DraftID   string            `json:"draft_id"`
}

type Matchup struct {
	Starters     []string    `json:"starters"`
	RosterID     int         `json:"roster_id"`
	Players      []string    `json:"players"`
	MatchupID    FlexibleInt `json:"matchup_id"`
	Points       float64     `json:"points"`
	CustomPoints *float64    `json:"custom_points"`
}

type Player struct {
	Hashtag            string         `json:"hashtag"`
	DepthChartPosition FlexibleInt    `json:"depth_chart_position"`
	Status             string         `json:"status"`
	Sport              string         `json:"sport"`
	FantasyPositions   []string       `json:"fantasy_positions"`
	Number             FlexibleInt    `json:"number"`
	SearchLastName     string         `json:"search_last_name"`
	Weight             string         `json:"weight"`
	Position           string         `json:"position"`
	Team               string         `json:"team"`
	LastName           string         `json:"last_name"`
	College            string         `json:"college"`
	FantasyDataID      FlexibleInt    `json:"fantasy_data_id"`
	InjuryStatus       *string        `json:"injury_status"`
	PlayerID           string         `json:"player_id"`
	Height             string         `json:"height"`
	SearchFullName     string         `json:"search_full_name"`
	Age                FlexibleInt    `json:"age"`
	FirstName          string         `json:"first_name"`
	DepthChartOrder    FlexibleInt    `json:"depth_chart_order"`
	YearsExp           FlexibleInt    `json:"years_exp"`
	RotowireID         FlexibleInt    `json:"rotowire_id"`
	RotoworldID        FlexibleInt    `json:"rotoworld_id"`
	SearchFirstName    string         `json:"search_first_name"`
	YahooID            FlexibleString `json:"yahoo_id"`
	ESPNID             FlexibleString `json:"espn_id"`
}
