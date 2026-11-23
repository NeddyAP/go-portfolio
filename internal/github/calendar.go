package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const cacheTTL = 6 * time.Hour

// ContributionDay represents a single day cell inside the contribution SVG grid.
type ContributionDay struct {
	Date  string
	Count int
	Level int
	Row   int // 0 (Sunday) .. 6 (Saturday)
	Y     int
}

// WeekColumn represents a vertical column of up to 7 days in the contribution graph.
type WeekColumn struct {
	ColIndex int
	X        int
	Days     []ContributionDay
}

// MonthLabel indicates where to draw a month caption above the SVG grid.
type MonthLabel struct {
	Name string
	X    int
}

// CalendarData contains all precomputed layout values for SSR SVG rendering.
type CalendarData struct {
	Username           string
	TotalContributions int
	Weeks              []WeekColumn
	MonthLabels        []MonthLabel
	SVGWidth           int
	SVGHeight          int
	FetchedAt          time.Time
}

type apiResponse struct {
	Total         map[string]int `json:"total"`
	Contributions []struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
		Level int    `json:"level"`
	} `json:"contributions"`
}

// Service caches GitHub contribution data with non-blocking stale-while-revalidate refreshes.
type Service struct {
	apiBaseURL string
	client     *http.Client
	onUpdate   func()

	mu       sync.RWMutex
	entries  map[string]*CalendarData
	inflight map[string]bool
}

// NewService creates a GitHub calendar Service.
func NewService(apiBaseURL string, onUpdate func()) *Service {
	if apiBaseURL == "" {
		apiBaseURL = "https://github-contributions-api.jogruber.de/v4"
	}
	return &Service{
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
		client:     &http.Client{Timeout: 5 * time.Second},
		onUpdate:   onUpdate,
		entries:    make(map[string]*CalendarData),
		inflight:   make(map[string]bool),
	}
}

// ParseUsername extracts the GitHub username safely from a URL (handling trailing slashes!).
func ParseUsername(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if u.Host == "" && !strings.Contains(rawURL, "/") {
		return rawURL
	}
	trimmed := strings.Trim(u.Path, "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	return strings.TrimSpace(parts[0])
}

// WarmAsync triggers a non-blocking background fetch for the given GitHub username.
func (s *Service) WarmAsync(username string) {
	username = strings.TrimSpace(username)
	if username == "" {
		return
	}
	s.mu.Lock()
	if s.inflight[username] {
		s.mu.Unlock()
		return
	}
	s.inflight[username] = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.inflight, username)
			s.mu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()

		data, err := s.fetch(ctx, username)
		if err != nil || data == nil {
			return
		}
		s.mu.Lock()
		s.entries[username] = data
		s.mu.Unlock()

		if s.onUpdate != nil {
			s.onUpdate()
		}
	}()
}

// GetCached returns cached calendar data immediately without blocking HTTP requests.
// If missing or expired, it schedules a background refresh so page loads stay sub-millisecond.
func (s *Service) GetCached(username string) *CalendarData {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil
	}

	s.mu.RLock()
	entry := s.entries[username]
	s.mu.RUnlock()

	if entry == nil {
		s.WarmAsync(username)
		return nil
	}
	if time.Since(entry.FetchedAt) > cacheTTL {
		s.WarmAsync(username)
	}
	return entry
}

// RefreshBlocking fetches calendar data synchronously (useful in tests or manual refresh).
func (s *Service) RefreshBlocking(ctx context.Context, username string) (*CalendarData, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("empty username")
	}
	data, err := s.fetch(ctx, username)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.entries[username] = data
	s.mu.Unlock()
	return data, nil
}

func (s *Service) fetch(ctx context.Context, username string) (*CalendarData, error) {
	endpoint := fmt.Sprintf("%s/%s?y=last", s.apiBaseURL, url.PathEscape(username))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "neddyap-portfolio-go/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github contributions API returned %d", resp.StatusCode)
	}

	var raw apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if len(raw.Contributions) == 0 {
		return nil, fmt.Errorf("no contributions returned")
	}

	return buildCalendarData(username, raw), nil
}

func buildCalendarData(username string, raw apiResponse) *CalendarData {
	const (
		cellSize = 13
		cellGap  = 4
		colStep  = cellSize + cellGap
		topPad   = 22
	)

	total := raw.Total["lastYear"]
	if total == 0 {
		for _, c := range raw.Contributions {
			total += c.Count
		}
	}

	var weeks []WeekColumn
	var currentWeek *WeekColumn
	var monthLabels []MonthLabel
	lastMonth := time.Month(0)

	for _, item := range raw.Contributions {
		t, err := time.Parse("2006-01-02", item.Date)
		if err != nil {
			continue
		}
		wd := int(t.Weekday()) // 0 = Sunday .. 6 = Saturday
		if currentWeek == nil || wd == 0 {
			colIdx := len(weeks)
			weeks = append(weeks, WeekColumn{
				ColIndex: colIdx,
				X:        colIdx * colStep,
			})
			currentWeek = &weeks[len(weeks)-1]
		}
		if t.Month() != lastMonth && wd <= 3 {
			if len(monthLabels) == 0 || (currentWeek.X-monthLabels[len(monthLabels)-1].X) >= colStep*2 {
				monthLabels = append(monthLabels, MonthLabel{
					Name: t.Format("Jan"),
					X:    currentWeek.X,
				})
				lastMonth = t.Month()
			}
		}

		lvl := item.Level
		if lvl < 0 {
			lvl = 0
		} else if lvl > 4 {
			lvl = 4
		}
		currentWeek.Days = append(currentWeek.Days, ContributionDay{
			Date:  item.Date,
			Count: item.Count,
			Level: lvl,
			Row:   wd,
			Y:     topPad + wd*colStep,
		})
	}

	svgWidth := len(weeks)*colStep - cellGap
	if svgWidth < 200 {
		svgWidth = 200
	}
	svgHeight := topPad + 7*colStep - cellGap

	return &CalendarData{
		Username:           username,
		TotalContributions: total,
		Weeks:              weeks,
		MonthLabels:        monthLabels,
		SVGWidth:           svgWidth,
		SVGHeight:          svgHeight,
		FetchedAt:          time.Now().UTC(),
	}
}
