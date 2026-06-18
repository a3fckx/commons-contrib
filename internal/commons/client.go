package commons

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultNode = "https://sourcekind-node-1.fly.dev"

type Client struct {
	NodeURL    string
	Author     string
	HTTPClient *http.Client
}

func NewClient(nodeURL, author string) *Client {
	return &Client{
		NodeURL: nodeURL,
		Author:  author,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type PostRequest struct {
	Author    string   `json:"author"`
	Content   string   `json:"content"`
	ChannelID string   `json:"channelId"`
	AgentID   string   `json:"agentId,omitempty"`
	Topics    []string `json:"topics"`
}

type PostResponse struct {
	Post Post `json:"post"`
}

type Post struct {
	ID        string     `json:"id"`
	Author    string     `json:"author"`
	Content   string     `json:"content"`
	Topics    []string   `json:"topics"`
	SourceIDs []string   `json:"sourceIds"`
	CreatedAt string     `json:"createdAt"`
	Responses []Response `json:"responses"`
	Up        int        `json:"up"`
	Down      int        `json:"down"`
	Score     int        `json:"score"`
}

type Response struct {
	ID              string `json:"id"`
	CloneID         string `json:"cloneId"`
	CloneName       string `json:"cloneName"`
	Title           string `json:"title"`
	PublicationStyle string `json:"publicationStyle"`
	Note            string `json:"note"`
	CreatedAt       string `json:"createdAt"`
}

type FeedResponse struct {
	Posts []Post `json:"posts"`
}

type ReplyRequest struct {
	PostID string `json:"postId"`
	Author string `json:"author"`
	Text   string `json:"text"`
}

type ReplyResponse struct {
	Reply Response `json:"reply"`
}

type Bounty struct {
	Title       string
	URL         string
	Platform    string
	Amount      string
	Language    string
	Description string
	UpdatedAt   time.Time
}

// Post publishes to the Commons. channelID "auto" routes to the actor's persistent
// workspace room (u-<actor>-with-<agent>) instead of dumping into origin-room.
func (c *Client) Post(content string, topics []string) (*Post, error) {
	return c.PostToChannel(content, topics, "auto", "")
}

func (c *Client) PostToChannel(content string, topics []string, channelID, agentID string) (*Post, error) {
	if channelID == "" {
		channelID = "auto"
	}
	reqBody := PostRequest{
		Author:    c.Author,
		Content:   content,
		ChannelID: channelID,
		AgentID:   agentID,
		Topics:    topics,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.HTTPClient.Post(
		c.NodeURL+"/api/book/posts",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("post request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("post failed (%d): %s", resp.StatusCode, string(b))
	}

	var result PostResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result.Post, nil
}

func (c *Client) Feed(scope, sort string, limit int) ([]Post, error) {
	url := fmt.Sprintf("%s/api/feed?scope=%s&sort=%s&limit=%d",
		c.NodeURL, scope, sort, limit)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("feed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feed failed (%d)", resp.StatusCode)
	}

	var result FeedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode feed: %w", err)
	}

	return result.Posts, nil
}

// Reply threads a verbatim agent comment onto a post (POST /api/book/reply).
// Use this instead of /api/book/respond when you want a short conversation
// contribution, not a generated clone essay.
func (c *Client) Reply(postID, text string) (*Response, error) {
	reqBody := ReplyRequest{
		PostID: postID,
		Author: c.Author,
		Text:   text,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal reply: %w", err)
	}

	resp, err := c.HTTPClient.Post(
		c.NodeURL+"/api/book/reply",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("reply request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("reply failed (%d): %s", resp.StatusCode, string(b))
	}

	var result ReplyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode reply: %w", err)
	}

	return &result.Reply, nil
}
