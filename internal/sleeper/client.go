package sleeper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	trimmed := strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: trimmed,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) GetUserLeagues(ctx context.Context, userID, sport string, season int) ([]League, error) {
	path := fmt.Sprintf("/user/%s/leagues/%s/%d", url.PathEscape(userID), url.PathEscape(sport), season)
	var leagues []League
	if err := c.getJSON(ctx, path, &leagues); err != nil {
		return nil, err
	}
	return leagues, nil
}

func (c *Client) GetLeague(ctx context.Context, leagueID string) (League, error) {
	var league League
	if err := c.getJSON(ctx, "/league/"+url.PathEscape(leagueID), &league); err != nil {
		return League{}, err
	}
	return league, nil
}

func (c *Client) GetLeagueUsers(ctx context.Context, leagueID string) ([]User, error) {
	var users []User
	if err := c.getJSON(ctx, "/league/"+url.PathEscape(leagueID)+"/users", &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (c *Client) GetLeagueRosters(ctx context.Context, leagueID string) ([]Roster, error) {
	var rosters []Roster
	if err := c.getJSON(ctx, "/league/"+url.PathEscape(leagueID)+"/rosters", &rosters); err != nil {
		return nil, err
	}
	return rosters, nil
}

func (c *Client) GetLeagueDrafts(ctx context.Context, leagueID string) ([]Draft, error) {
	var drafts []Draft
	if err := c.getJSON(ctx, "/league/"+url.PathEscape(leagueID)+"/drafts", &drafts); err != nil {
		return nil, err
	}
	return drafts, nil
}

func (c *Client) GetDraftPicks(ctx context.Context, draftID string) ([]DraftPick, error) {
	var picks []DraftPick
	if err := c.getJSON(ctx, "/draft/"+url.PathEscape(draftID)+"/picks", &picks); err != nil {
		return nil, err
	}
	return picks, nil
}

func (c *Client) GetLeagueMatchups(ctx context.Context, leagueID string, week int) ([]Matchup, error) {
	var matchups []Matchup
	path := fmt.Sprintf("/league/%s/matchups/%d", url.PathEscape(leagueID), week)
	if err := c.getJSON(ctx, path, &matchups); err != nil {
		return nil, err
	}
	return matchups, nil
}

func (c *Client) GetLeagueWinnersBracket(ctx context.Context, leagueID string) ([]BracketMatchup, error) {
	return c.getLeagueBracket(ctx, leagueID, "winners_bracket")
}

func (c *Client) GetLeagueLosersBracket(ctx context.Context, leagueID string) ([]BracketMatchup, error) {
	return c.getLeagueBracket(ctx, leagueID, "losers_bracket")
}

func (c *Client) getLeagueBracket(ctx context.Context, leagueID, bracket string) ([]BracketMatchup, error) {
	var matchups []BracketMatchup
	path := fmt.Sprintf("/league/%s/%s", url.PathEscape(leagueID), bracket)
	if err := c.getJSON(ctx, path, &matchups); err != nil {
		return nil, err
	}
	return matchups, nil
}

func (c *Client) GetPlayers(ctx context.Context, sport string) (map[string]Player, error) {
	players := make(map[string]Player)
	if err := c.getJSON(ctx, "/players/"+url.PathEscape(sport), &players); err != nil {
		return nil, err
	}
	return players, nil
}

func (c *Client) GetState(ctx context.Context, sport string) (map[string]any, error) {
	state := make(map[string]any)
	if err := c.getJSON(ctx, "/state/"+url.PathEscape(sport), &state); err != nil {
		return nil, err
	}
	return state, nil
}

func (c *Client) getJSON(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sleeper api returned %s for %s", resp.Status, path)
	}

	return json.NewDecoder(resp.Body).Decode(dst)
}
