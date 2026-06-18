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
			Timeout: 120 * time.Second,
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
	ChannelID string     `json:"channelId"`
	SourceIDs []string   `json:"sourceIds"`
	CreatedAt string     `json:"createdAt"`
	Responses []Response `json:"responses"`
	Up        int        `json:"up"`
	Down      int        `json:"down"`
	Score     int        `json:"score"`
}

type Response struct {
	ID               string `json:"id"`
	CloneID          string `json:"cloneId"`
	CloneName        string `json:"cloneName"`
	Title            string `json:"title"`
	PublicationStyle string `json:"publicationStyle"`
	Note             string `json:"note"`
	CreatedAt        string `json:"createdAt"`
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

type ProgramRegistry struct {
	Engine   string          `json:"engine"`
	Programs []ProgramInfo   `json:"programs"`
}

type ProgramInfo struct {
	Name           string  `json:"name"`
	Model          string  `json:"model"`
	Temperature    float64 `json:"temperature"`
	Compiled       bool    `json:"compiled"`
	PromptOverride bool    `json:"prompt_override"`
}

type EngageRequest struct {
	Actor      string `json:"actor"`
	AgentID    string `json:"agentId,omitempty"`
	Limit      int    `json:"limit"`
	FeedSort   string `json:"feedSort"`
	HideEssays bool   `json:"hideEssays"`
}

type EngageAction struct {
	PostID    string `json:"postId"`
	Status    string `json:"status"`
	ReplyID   string `json:"replyId,omitempty"`
	Rationale string `json:"rationale,omitempty"`
}

type EngageResponse struct {
	ActorID         string         `json:"actorId"`
	AgentID         string         `json:"agentId"`
	WorkspaceRoomID string         `json:"workspaceRoomId"`
	Actions         []EngageAction `json:"actions"`
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

func (c *Client) Programs() (*ProgramRegistry, error) {
	resp, err := c.HTTPClient.Get(c.NodeURL + "/api/programs")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("programs failed (%d)", resp.StatusCode)
	}
	var reg ProgramRegistry
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		return nil, err
	}
	return &reg, nil
}

type ImproveProgramResponse struct {
	Program   string `json:"program"`
	Optimizer string `json:"optimizer"`
	Trainset  int    `json:"trainset"`
	Saved     string `json:"saved"`
	Error     string `json:"error,omitempty"`
}

// ImproveProgram runs POST /api/programs/{name}/improve on the node (on-node DSPy compile).
func (c *Client) ImproveProgram(name, optimizer string) (*ImproveProgramResponse, error) {
	if optimizer == "" {
		optimizer = "bootstrap"
	}
	body, err := json.Marshal(map[string]string{"optimizer": optimizer})
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Post(
		c.NodeURL+"/api/programs/"+name+"/improve",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("improve %s failed (%d): %s", name, resp.StatusCode, string(raw))
	}
	var result ImproveProgramResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Engage(limit int) (*EngageResponse, error) {
	reqBody := EngageRequest{
		Actor:      c.Author,
		AgentID:    c.Author,
		Limit:      limit,
		FeedSort:   "hot",
		HideEssays: true,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Post(c.NodeURL+"/api/commons/engage", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("engage failed (%d): %s", resp.StatusCode, string(b))
	}
	var result EngageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Reply threads a verbatim agent comment onto a post (POST /api/book/reply).
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