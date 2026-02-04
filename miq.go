package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/darui3018823/discordgo"
	"github.com/u16-io/FindSenryu4Discord/config"
	"github.com/u16-io/FindSenryu4Discord/db"
	"github.com/u16-io/FindSenryu4Discord/model"
	"github.com/u16-io/FindSenryu4Discord/service"
)

const (
	MiqTempDir = "./Temp/miq"
)

type QuoteRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Text        string `json:"text"`
	Avatar      string `json:"avatar"`
	Color       bool   `json:"color"`
}

type QuoteResponse struct {
	URL string `json:"url"`
}

// senryuType represents the type of senryu message
type senryuType int

const (
	senryuTypeUnknown  senryuType = iota
	senryuTypeDetected            // 川柳を検出しました！
	senryuTypeYomuna              // 詠むな (お前が~ / <@ID>が~)
	senryuTypeYome                // 詠め (ここで一句)
)

// parseSenryuMessage parses a bot message and extracts senryu info
// Returns: senryuType, extracted text (with newlines), mentioned user ID (if any)
func parseSenryuMessage(content string) (senryuType, string, string) {
	// Extract text inside 「」
	re := regexp.MustCompile(`「([^」]+)」`)
	match := re.FindStringSubmatch(content)
	if len(match) < 2 {
		return senryuTypeUnknown, "", ""
	}

	senryuText := match[1]
	// Split by half-width space and join with newline
	parts := strings.Split(senryuText, " ")
	formattedText := strings.Join(parts, "\n")

	// Determine message type
	if strings.HasPrefix(content, "川柳を検出しました！") {
		return senryuTypeDetected, formattedText, ""
	}

	if strings.HasPrefix(content, "ここで一句") {
		return senryuTypeYome, formattedText, ""
	}

	// 詠むな patterns: "お前が「...」って詠んだのが最後やぞ" or "<@ID> が「...」って詠んだのが最後やぞ"
	if strings.Contains(content, "って詠んだのが最後やぞ") {
		// Check for mention
		mentionRe := regexp.MustCompile(`<@!?(\d+)>\s*が`)
		mentionMatch := mentionRe.FindStringSubmatch(content)
		if len(mentionMatch) >= 2 {
			return senryuTypeYomuna, formattedText, mentionMatch[1]
		}
		// "お前が" pattern (no mention)
		if strings.HasPrefix(content, "お前が") {
			return senryuTypeYomuna, formattedText, ""
		}
	}

	return senryuTypeUnknown, formattedText, ""
}

// getMemberAvatarURL prioritizes Guild Avatar -> User Avatar -> Default
func getMemberAvatarURL(member *discordgo.Member, user *discordgo.User, defaultURL string) string {
	if member != nil && member.Avatar != "" {
		return fmt.Sprintf("https://cdn.discordapp.com/guilds/%s/users/%s/avatars/%s.png?size=1024", member.GuildID, user.ID, member.Avatar)
	}
	if user != nil {
		return user.AvatarURL("1024")
	}
	return defaultURL
}

// handleSenryuMiqContext handles the context menu for senryu image generation
func handleSenryuMiqContext(s *discordgo.Session, i *discordgo.InteractionCreate) {
	log.Printf("[Senryu MIQ] Called by %s", i.Member.User.Username)

	// Check if required settings are configured
	conf := config.GetConf()
	if conf.CDN.QuoteAPIURL == "" || conf.CDN.Token == "" || conf.CDN.UploadURL == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "この機能は現在設定されていません。",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	data := i.ApplicationCommandData()
	targetMsgID := data.TargetID
	targetMsg := data.Resolved.Messages[targetMsgID]

	if targetMsg == nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "対象のメッセージが見つかりませんでした。",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Check if message is from bot
	if targetMsg.Author.ID != s.State.User.ID {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "川柳Botのメッセージのみ対応しています。",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Parse the message
	msgType, senryuText, mentionedUserID := parseSenryuMessage(targetMsg.Content)
	if msgType == senryuTypeUnknown || senryuText == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "川柳のメッセージを解析できませんでした。",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Defer response
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	var avatarURL, username, displayName string
	botAvatarURL := s.State.User.AvatarURL("1024")

	switch msgType {
	case senryuTypeDetected:
		// Get avatar from the reply target (original senryu author)
		if targetMsg.MessageReference != nil {
			refMsg, err := s.ChannelMessage(targetMsg.ChannelID, targetMsg.MessageReference.MessageID)
			if err == nil {
				member, _ := s.GuildMember(i.GuildID, refMsg.Author.ID)
				avatarURL = getMemberAvatarURL(member, refMsg.Author, botAvatarURL)
				username = refMsg.Author.Username
				displayName = refMsg.Author.GlobalName
				if displayName == "" {
					displayName = username
				}
				if member != nil && member.Nick != "" {
					displayName = member.Nick
				}
			}
		}
		if avatarURL == "" {
			avatarURL = botAvatarURL
			username = s.State.User.Username
			displayName = s.State.User.Username
		}

	case senryuTypeYome:
		// Look up author IDs from YomeMessage
		yomeMsg, err := service.GetYomeMessage(targetMsg.ID)
		if err == nil && yomeMsg != nil {
			// Randomly select one of the three authors
			authorIDs := []string{yomeMsg.Author1ID, yomeMsg.Author2ID, yomeMsg.Author3ID}
			selectedAuthorID := authorIDs[rand.Intn(len(authorIDs))]

			user, err := s.User(selectedAuthorID)
			if err == nil {
				member, _ := s.GuildMember(i.GuildID, selectedAuthorID)
				avatarURL = getMemberAvatarURL(member, user, botAvatarURL)
				// Use DB cache for avatar
				cachedURL := getAvatarURL(user.ID)
				if cachedURL != "" {
					avatarURL = cachedURL
				} else {
					// Fallback to Discord URL directly and cache it
					// Note: avatarURL from getMemberAvatarURL is already Discord CDN URL
					go saveAvatarURL(user.ID, avatarURL)
				}
				username = user.Username
				displayName = user.GlobalName
				if displayName == "" {
					displayName = username
				}
				if member != nil && member.Nick != "" {
					displayName = member.Nick
				}
			}
		}
		// Fallback to bot avatar if no YomeMessage found or error
		if avatarURL == "" {
			avatarURL = botAvatarURL
			username = s.State.User.Username
			displayName = s.State.User.Username
		}

	case senryuTypeYomuna:
		if mentionedUserID != "" {
			// Other user mentioned
			user, err := s.User(mentionedUserID)
			if err == nil {
				member, _ := s.GuildMember(i.GuildID, mentionedUserID)
				avatarURL = getMemberAvatarURL(member, user, botAvatarURL)
				username = user.Username
				displayName = user.GlobalName
				if displayName == "" {
					displayName = username
				}
				if member != nil && member.Nick != "" {
					displayName = member.Nick
				}
			}
		} else {
			// "お前が~" - use the person who invoked 詠むな (reply target of bot message)
			if targetMsg.MessageReference != nil {
				refMsg, err := s.ChannelMessage(targetMsg.ChannelID, targetMsg.MessageReference.MessageID)
				if err == nil {
					member, _ := s.GuildMember(i.GuildID, refMsg.Author.ID)
					avatarURL = getMemberAvatarURL(member, refMsg.Author, botAvatarURL)
					username = refMsg.Author.Username
					displayName = refMsg.Author.GlobalName
					if displayName == "" {
						displayName = username
					}
					if member != nil && member.Nick != "" {
						displayName = member.Nick
					}
				}
			}
		}
		if avatarURL == "" {
			avatarURL = botAvatarURL
			username = s.State.User.Username
			displayName = s.State.User.Username
		}
	}

	log.Printf("[Senryu MIQ] Type: %d, Text: %s, Avatar: %s", msgType, senryuText, avatarURL)

	cdnURL, err := createQuoteAndUpload(username, displayName, senryuText, avatarURL)
	if err != nil {
		log.Printf("[Senryu MIQ] Failed to create quote: %v", err)
		errMsg := err.Error()
		var responseMsg string
		if strings.Contains(errMsg, "CDN") || strings.Contains(errMsg, "upload") {
			responseMsg = "画像のCDNへのアップロードに失敗しました。しばらくしてからもう一度お試しください。"
		} else if strings.Contains(errMsg, "API") || strings.Contains(errMsg, "download") {
			responseMsg = "画像の生成に失敗しました。引用APIが応答していない可能性があります。"
		} else {
			responseMsg = fmt.Sprintf("エラーが発生しました: %v", err)
		}
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: responseMsg,
		})
		return
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: cdnURL,
	})
}

func createQuoteAndUpload(username, displayName, messageContent, avatarURL string) (string, error) {
	quoteReq := QuoteRequest{
		Username:    username,
		DisplayName: displayName,
		Text:        messageContent,
		Avatar:      avatarURL,
		Color:       true,
	}

	reqBody, err := json.Marshal(quoteReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	conf := config.GetConf()
	resp, err := http.Post(conf.CDN.QuoteAPIURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to call quote API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("quote API returned status %d: %s", resp.StatusCode, string(body))
	}

	var quoteResp QuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&quoteResp); err != nil {
		return "", fmt.Errorf("failed to decode quote response: %w", err)
	}

	if quoteResp.URL == "" {
		return "", fmt.Errorf("quote API did not return image URL")
	}

	log.Printf("[Senryu MIQ] Quote image URL: %s", quoteResp.URL)

	imgResp, err := http.Get(quoteResp.URL)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer imgResp.Body.Close()

	if imgResp.StatusCode != 200 {
		return "", fmt.Errorf("failed to download image, status: %d", imgResp.StatusCode)
	}

	imgData, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %w", err)
	}

	if err := os.MkdirAll(MiqTempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	parsedURL, _ := url.Parse(quoteResp.URL)
	filename := filepath.Base(parsedURL.Path)
	tempPath := filepath.Join(MiqTempDir, filename)

	if err := os.WriteFile(tempPath, imgData, 0644); err != nil {
		return "", fmt.Errorf("failed to save temp file: %w", err)
	}

	log.Printf("[Senryu MIQ] Saved temp file: %s", tempPath)

	cdnURL, err := uploadToCDN(imgData, "senryu", filename, "image/png")
	if err != nil {
		return "", fmt.Errorf("failed to upload to CDN: %w", err)
	}

	cdnURL = strings.ReplaceAll(cdnURL, "//", "/")
	if strings.HasPrefix(cdnURL, "http:/") && !strings.HasPrefix(cdnURL, "http://") {
		cdnURL = strings.Replace(cdnURL, "http:/", "http://", 1)
	}
	if strings.HasPrefix(cdnURL, "https:/") && !strings.HasPrefix(cdnURL, "https://") {
		cdnURL = strings.Replace(cdnURL, "https:/", "https://", 1)
	}

	log.Printf("[Senryu MIQ] CDN upload successful: %s", cdnURL)
	return cdnURL, nil
}

func uploadToCDN(content []byte, subpath, filename, contentType string) (string, error) {
	conf := config.GetConf()
	cdnToken := conf.CDN.Token
	cdnBaseURL := conf.CDN.UploadURL

	if cdnToken == "" {
		return "", fmt.Errorf("CDN_TOKEN is not set")
	}
	if cdnBaseURL == "" {
		return "", fmt.Errorf("CDN_UPLOAD_URL is not set")
	}

	uploadURL := fmt.Sprintf("%s/%s", cdnBaseURL, subpath)
	log.Printf("[Senryu MIQ CDN] Uploading to: %s", uploadURL)

	// Retry logic: try up to 3 times with exponential backoff
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("[Senryu MIQ CDN] Upload attempt %d/%d", attempt, maxRetries)

		body := &bytes.Buffer{}
		boundary := "----WebKitFormBoundary" + randomString(16)

		// Properly construct multipart form data
		fmt.Fprintf(body, "--%s\r\n", boundary)
		fmt.Fprintf(body, "Content-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\n", filename)
		fmt.Fprintf(body, "Content-Type: %s\r\n\r\n", contentType)
		body.Write(content)
		fmt.Fprintf(body, "\r\n--%s--\r\n", boundary)

		req, err := http.NewRequest("POST", uploadURL, body)
		if err != nil {
			lastErr = fmt.Errorf("CDN request creation error: %w", err)
			log.Printf("[Senryu MIQ CDN] %v", lastErr)
			continue
		}

		req.Header.Set("Authorization", "Bearer "+cdnToken)
		req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("CDN upload request error: %w", err)
			log.Printf("[Senryu MIQ CDN] %v", lastErr)
			// Wait before retry with exponential backoff
			if attempt < maxRetries {
				waitTime := time.Duration(attempt*2) * time.Second
				log.Printf("[Senryu MIQ CDN] Retrying in %v...", waitTime)
				time.Sleep(waitTime)
			}
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodyText := string(bodyBytes)

		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("CDN upload failed with status %d: %s", resp.StatusCode, bodyText)
			log.Printf("[Senryu MIQ CDN] %v", lastErr)
			// Wait before retry
			if attempt < maxRetries {
				waitTime := time.Duration(attempt*2) * time.Second
				log.Printf("[Senryu MIQ CDN] Retrying in %v...", waitTime)
				time.Sleep(waitTime)
			}
			continue
		}

		// Success!
		cdnURL := strings.TrimSpace(bodyText)
		log.Printf("[Senryu MIQ CDN] Upload successful on attempt %d: %s", attempt, cdnURL)
		return cdnURL, nil
	}

	return "", fmt.Errorf("CDN upload failed after %d attempts: %w", maxRetries, lastErr)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// getAvatarURL retrieves the cached avatar URL from DB
func getAvatarURL(userID string) string {
	var avatar model.AvatarCache
	if err := db.DB.First(&avatar, "user_id = ?", userID).Error; err != nil {
		return ""
	}
	return avatar.AvatarURL
}

// saveAvatarURL saves the avatar URL to DB
func saveAvatarURL(userID, url string) {
	if url == "" {
		return
	}
	avatar := model.AvatarCache{
		UserID:    userID,
		AvatarURL: url,
	}
	db.DB.Save(&avatar)
}
