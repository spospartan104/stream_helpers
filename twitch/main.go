package main


import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"net/url"

	"golang.org/x/oauth2"
)

const twitchAPI = "https://api.twitch.tv/helix"

var (
	clientID             = os.Getenv("TWITCH_CLIENT_ID")
	clientSecret         = os.Getenv("TWITCH_CLIENT_SECRET")
	refreshToken         string
	configRoot           = os.Getenv("TWITCH_API_DIR")
	configDir            string
	configDirName        = ".streaming"
	tokenFile            string
	tokenFileName        = "refresh_token.txt"
	clientIDFile         string
	clientIDFileName     = "client_id.txt"
	clientSecretFile     string
	clientSecretFileName = "client_secret.txt"
)

var oauth2Config = &oauth2.Config{
	Endpoint: oauth2.Endpoint{
		TokenURL: "https://id.twitch.tv/oauth2/token",
	},
	Scopes: []string{"moderator:read:followers", "channel:read:subscriptions"},
}

func main() {
	// Determine if we have enough credentials + refresh token
	hasClientCreds := clientID != "" && clientSecret != ""
	hasRefreshToken := false

	if configRoot == "" {
		userhome, err := os.UserHomeDir()
		if err != nil {
			log.Errorf("Could not get user home dir; will need to prompt for path %s\n", err)
		} else {
			configRoot = userhome
			configDir = filepath.Join(userhome, configDirName)
			fmt.Printf("Config path: %s\n", configDir)
			fmt.Print("Making config path\n")
			err := os.MkdirAll(configDir, 0750)
			if err != nil {
				fmt.Printf("\nwtf? %s", err)
			}

			tokenFile = filepath.Join(configDir, tokenFileName)
			clientIDFile = filepath.Join(configDir, clientIDFileName)
			clientSecretFile = filepath.Join(configDir, clientSecretFileName)
		}
	}

	if t := os.Getenv("TWITCH_REFRESH_TOKEN"); t != "" {
		hasRefreshToken = true
	} else if data, err := os.ReadFile(tokenFile); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		hasRefreshToken = true
	}

	if len(os.Args) < 2 {
		if !hasClientCreds || !hasRefreshToken {
			fmt.Println("⚠️ Missing credentials or refresh token. Running bootstrap...")
			bootstrap()
		}
		run()
		return
	}

	// handle explicit commands
	switch os.Args[1] {
	case "bootstrap":
		bootstrap()
	case "run":
		run()
	default:
		fmt.Println("Usage: twitch-client [bootstrap|run]")
	}
}

// === Bootstrap flow ===
func bootstrap() {
	reader := bufio.NewReader(os.Stdin)

	// Ensure config directory exists
	if configDir == "" {
		if configRoot == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				log.Fatal("Cannot determine home directory:", err)
			}
			configRoot = home
		}
		configDir = filepath.Join(configRoot, configDirName)
		if err := os.MkdirAll(configDir, 0750); err != nil {
			log.Fatalf("Failed to create config dir: %v", err)
		}

		tokenFile = filepath.Join(configDir, tokenFileName)
		clientIDFile = filepath.Join(configDir, clientIDFileName)
		clientSecretFile = filepath.Join(configDir, clientSecretFileName)
		fmt.Printf("Config path: %s\n", configDir)
	}

	// Prompt/load client ID
	for clientID == "" {
		if data, err := os.ReadFile(clientIDFile); err == nil && len(strings.TrimSpace(string(data))) > 0 {
			clientID = strings.TrimSpace(string(data))
			log.Info("Client ID loaded from file")
			break
		}
		fmt.Print("Enter TWITCH_CLIENT_ID: ")
		input, _ := reader.ReadString('\n')
		clientID = strings.TrimSpace(input)
		if clientID != "" {
			os.WriteFile(clientIDFile, []byte(clientID), 0600)
		}
	}

	// Prompt/load client secret
	for clientSecret == "" {
		if data, err := os.ReadFile(clientSecretFile); err == nil && len(strings.TrimSpace(string(data))) > 0 {
			clientSecret = strings.TrimSpace(string(data))
			log.Info("Client secret loaded from file")
			break
		}
		fmt.Print("Enter TWITCH_CLIENT_SECRET: ")
		input, _ := reader.ReadString('\n')
		clientSecret = strings.TrimSpace(input)
		if clientSecret != "" {
			os.WriteFile(clientSecretFile, []byte(clientSecret), 0600)
		}
	}

	refresh_file_tested:=false
	// Prompt for refresh token or redirect URI
	for {
		// fallback: read from file
		if !refresh_file_tested {
			// fallback: read from file
			data, err := os.ReadFile(tokenFile)
			if err != nil || len(strings.TrimSpace(string(data))) == 0 {
				fmt.Println("No token found in file, please try again.")
				continue
			}
			refreshToken = strings.TrimSpace(string(data))
			refresh_file_tested = true
		} else {
			fmt.Printf("Please go to https://id.twitch.tv/oauth2/authorize?client_id=%s&redirect_uri=http://localhost&response_type=code&scope=moderator:read:followers+channel:read:subscriptions\n", clientID)

			fmt.Print("Paste full Twitch redirect URI (or press Enter to read from file): ")
			redirectInput, _ := reader.ReadString('\n')
			redirectInput = strings.TrimSpace(redirectInput)

			if redirectInput != "" {
				rt, at, err := exchangeCodeForRefreshTokenWithAccess(redirectInput, clientID, clientSecret, "http://localhost")
				if err != nil {
					fmt.Printf("❌ Failed to exchange code for refresh token: %v\n", err)
					continue
				}
				refreshToken = rt
				saveRefreshToken(rt)

				// Use the returned access token immediately
				token := &oauth2.Token{
					AccessToken:  at,
					RefreshToken: refreshToken,
					Expiry:       time.Now().Add(time.Hour), // temporary
				}
				ts := oauth2Config.TokenSource(context.Background(), token)
				client := oauth2.NewClient(context.Background(), ts)

				// Test API call
				userID := getUserID(client)
				fmt.Printf("✅ Bootstrap successful. Broadcaster ID: %s\n", userID)
				break
			} else {
				// fallback: read from file
				data, err := os.ReadFile(tokenFile)
				if err != nil || len(strings.TrimSpace(string(data))) == 0 {
					fmt.Println("No token found in file, please try again.")
					continue
				}
				refreshToken = strings.TrimSpace(string(data))
			}
		}
		// Verify token works
		oauth2Config.ClientID = clientID
		oauth2Config.ClientSecret = clientSecret
		ctx := context.Background()
		token := &oauth2.Token{
			RefreshToken: refreshToken,
			Expiry:       time.Now().Add(-time.Hour),
		}
		ts := oauth2Config.TokenSource(ctx, token)
		client := oauth2.NewClient(ctx, ts)

		newToken, err := ts.Token()
		if err != nil {
			fmt.Printf("❌ Failed to refresh token: %v\n", err)
			fmt.Println("Retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}

		// Save rotated token
		if newToken.RefreshToken != "" && newToken.RefreshToken != refreshToken {
			fmt.Println("🔄 Refresh token rotated, saving new token.")
			saveRefreshToken(newToken.RefreshToken)
			refreshToken = newToken.RefreshToken
		}

		// Test API call
		userID := getUserID(client)
		fmt.Printf("✅ Bootstrap successful. Broadcaster ID: %s\n", userID)
		break
	}
}


// === Normal run flow ===
func run() {
	if clientID == "" || clientSecret == "" {
		log.Fatal("Set TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET")
	}

	refreshToken := loadRefreshToken()
	oauth2Config.ClientID = clientID
	oauth2Config.ClientSecret = clientSecret

	ctx := context.Background()
	token := &oauth2.Token{
		RefreshToken: refreshToken,
		Expiry:       time.Now().Add(-time.Hour), // expired so it refreshes
	}
	ts := oauth2Config.TokenSource(ctx, token)
	client := oauth2.NewClient(ctx, ts)

	// refresh immediately
	for{
		newToken, err := ts.Token()
		if err != nil {
			log.Fatalf("failed to refresh token: %v", err)
		}
		if newToken.RefreshToken != "" && newToken.RefreshToken != refreshToken {
			fmt.Println("🔄 Refresh token rotated, saving new token.")
			saveRefreshToken(newToken.RefreshToken)
		}

		// fetch info
		userID := getUserID(client)
		// fmt.Println("Broadcaster ID:", userID)

		follower := getLatestFollower(client, userID)
		fmt.Println("Most recent follower:", follower)
		err = os.WriteFile(filepath.Join(configDir, "latestFollow.txt"), []byte(follower), 0600)
		if err != nil {
			log.Printf("⚠️ Failed to save latestFollow: %v", err)
		}
		subscriber := getLatestSubscriber(client, userID)
		fmt.Println("Most recent subscriber:", subscriber)
		err = os.WriteFile(filepath.Join(configDir, "latestSubscriber.txt"), []byte(subscriber), 0600)
		if err != nil {
			log.Printf("⚠️ Failed to save latestSubscriber: %v", err)
		}
		time.Sleep( 30 * time.Second)
	}
	
}


// === Helpers ===
func loadRefreshToken() string {
	if t := os.Getenv("TWITCH_REFRESH_TOKEN"); t != "" {
		return strings.TrimSpace(t)
	}
	data, err := os.ReadFile(tokenFile)
	if err == nil {
		return strings.TrimSpace(string(data))
	}
	fmt.Println("⚠️ No refresh token found. Run `./twitch-client bootstrap` first.")
	os.Exit(1)
	return ""
}

func saveRefreshToken(t string) {
	if t == "" {
		return
	}
	err := os.WriteFile(tokenFile, []byte(t), 0600)
	if err != nil {
		log.Printf("⚠️ Failed to save refresh token: %v", err)
	}
}

func getUserID(client *http.Client) string {
	req, _ := http.NewRequest("GET", twitchAPI+"/users", nil)
	req.Header.Set("Client-Id", clientID)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		log.Fatal(err)
	}
	if len(body.Data) == 0 {
		log.Fatal("no user data returned")
	}
	return body.Data[0].ID
}

func getLatestFollower(client *http.Client, userID string) string {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/channels/followers?broadcaster_id=%s&first=1", twitchAPI, userID), nil)
	req.Header.Set("Client-Id", clientID)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var res struct {
		Data []struct {
			UserName string `json:"user_name"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	if len(res.Data) == 0 {
		return "N/A"
	}
	return res.Data[0].UserName
}

func getLatestSubscriber(client *http.Client, userID string) string {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/subscriptions?broadcaster_id=%s&first=1", twitchAPI, userID), nil)
	req.Header.Set("Client-Id", clientID)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var res struct {
		Data []struct {
			UserName string `json:"user_name"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	if len(res.Data) == 0 {
		return "N/A"
	}
	return res.Data[0].UserName
}


func exchangeCodeForRefreshTokenWithAccess(redirectURI, clientID, clientSecret, redirectURL string) (string, string, error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return "", "", err
	}

	code := u.Query().Get("code")
	if code == "" && strings.Contains(redirectURI, "#") {
		values, err := url.ParseQuery(strings.SplitN(redirectURI, "#", 2)[1])
		if err != nil {
			return "", "", err
		}
		code = values.Get("code")
	}
	if code == "" {
		return "", "", fmt.Errorf("no code found in URI")
	}

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", redirectURL)

	req, err := http.NewRequest("POST", "https://id.twitch.tv/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tr struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", "", err
	}

	return tr.RefreshToken, tr.AccessToken, nil
}

