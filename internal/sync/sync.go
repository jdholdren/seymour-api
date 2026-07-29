package sync

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sym01/htmlsanitizer"

	"github.com/jdholdren/seymour/internal/seymour"
)

// Represents a response from an RSS feed fetch.
type rssFeedResp struct {
	XMLName xml.Name `xml:"rss"`
	Channel []struct {
		Title       string `xml:"title"`
		Description string `xml:"description"`
		Link        string `xml:"link"`
		Items       []struct {
			Title       string   `xml:"title"`
			Links       []string `xml:"link"`
			GUID        string   `xml:"guid"`
			Description string   `xml:"description"`
			PubDate     string   `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

// Represents a response from an Atom feed fetch.
type atomFeedResp struct {
	XMLName  xml.Name `xml:"feed"`
	Title    string   `xml:"title"`
	Subtitle string   `xml:"subtitle"`
	Links    []struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
	} `xml:"link"`
	Entries []struct {
		Title string `xml:"title"`
		ID    string `xml:"id"`
		Links []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
		} `xml:"link"`
		Summary string `xml:"summary"`
		Content string `xml:"content"`
		Updated string `xml:"updated"`
	} `xml:"entry"`
}

var syncClient = &http.Client{
	Timeout: time.Second * 3,
}

func Feed(ctx context.Context, feedID, feedURL string) (seymour.Feed, []seymour.FeedEntry, error) {
	resp, err := syncClient.Get(feedURL)
	if err != nil {
		return seymour.Feed{}, nil, fmt.Errorf("error getting feed url: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return seymour.Feed{}, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return seymour.Feed{}, nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Detect feed format based on the root XML element
	format := detectFormat(body)
	switch format {
	case "atom":
		return parseAtom(feedID, body)
	default:
		return parseRSS(feedID, body)
	}
}

// detectFormat peeks at the root XML element to determine if the feed is RSS or Atom.
func detectFormat(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := decoder.Token()
		if err != nil {
			return "rss"
		}
		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local == "feed" {
				return "atom"
			}
			return "rss"
		}
	}
}

func parseRSS(feedID string, data []byte) (seymour.Feed, []seymour.FeedEntry, error) {
	var feedResp rssFeedResp
	if err := xml.NewDecoder(bytes.NewReader(data)).Decode(&feedResp); err != nil {
		return seymour.Feed{}, nil, fmt.Errorf("error decoding rss feed: %w", err)
	}

	entries := []seymour.FeedEntry{}
	for _, channel := range feedResp.Channel {
		for _, item := range channel.Items {
			// Parse the links to the post
			var nonEmptyLink string
			for _, link := range item.Links {
				if link == "" {
					continue
				}

				nonEmptyLink = link
			}

			// Parse the publish date
			var publishedAt seymour.DBTime
			if parsedTime, err := time.Parse(time.RFC1123, item.PubDate); err == nil {
				publishedAt.Time = parsedTime
			}
			if parsedTime, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
				publishedAt.Time = parsedTime
			}

			entries = append(entries, seymour.FeedEntry{
				FeedID:      feedID,
				GUID:        item.GUID,
				Title:       sanitize(item.Title),
				Description: sanitize(item.Description),
				Link:        nonEmptyLink,
				PublishTime: publishedAt,
			})
		}
	}

	// Only the fields being updated:
	return seymour.Feed{
		ID:          feedID,
		Title:       &feedResp.Channel[0].Title,
		Description: &feedResp.Channel[0].Description,
	}, entries, nil
}

func parseAtom(feedID string, data []byte) (seymour.Feed, []seymour.FeedEntry, error) {
	var feedResp atomFeedResp
	if err := xml.NewDecoder(bytes.NewReader(data)).Decode(&feedResp); err != nil {
		return seymour.Feed{}, nil, fmt.Errorf("error decoding atom feed: %w", err)
	}

	entries := []seymour.FeedEntry{}
	for _, entry := range feedResp.Entries {
		// Find the best link: prefer "alternate", fall back to first with href
		var link string
		for _, l := range entry.Links {
			if l.Href == "" {
				continue
			}
			if link == "" || l.Rel == "alternate" {
				link = l.Href
			}
		}

		// Use content if summary is empty
		description := entry.Summary
		if description == "" {
			description = entry.Content
		}

		// Parse the publish date (Atom uses RFC3339)
		var publishedAt seymour.DBTime
		if parsedTime, err := time.Parse(time.RFC3339, entry.Updated); err == nil {
			publishedAt.Time = parsedTime
		}

		entries = append(entries, seymour.FeedEntry{
			FeedID:      feedID,
			GUID:        entry.ID,
			Title:       sanitize(entry.Title),
			Description: sanitize(description),
			Link:        link,
			PublishTime: publishedAt,
		})
	}

	subtitle := feedResp.Subtitle
	return seymour.Feed{
		ID:          feedID,
		Title:       &feedResp.Title,
		Description: &subtitle,
	}, entries, nil
}

// Removes all html tags from the string, usually a description.
//
// Also limits the length of the string so there's not a massive chunk of text being output.
func sanitize(s string) string {
	sanitizer := htmlsanitizer.NewHTMLSanitizer()
	sanitizer.AllowList = nil // Remove anything HTML

	// Remove any extra HTML
	s, err := sanitizer.SanitizeString(s)
	if err != nil {
		return ""
	}

	// Unescape any HTML codes into the real thing for display
	s = html.UnescapeString(s)

	// Remove whitespace and newlines
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")

	// Keep the length under 2048: some feeds put the whole post in there.
	if len(s) > 2048 {
		s = s[:2048]
	}

	return s
}
