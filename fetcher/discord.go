package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// discordData holds the URL and text for the Discord notification.
type discordData struct {
	URL  string
	Text string
}

// discordActions is an action generator that checks page content and sends a notification
// via the provided discordMetaData channel if the content differs from the expected text.
type discordActions struct {
	postActionData chan discordData
	checkSelector  string
	checkType      string
	expectedText   string
	url            string
}

// Generate creates a chromedp task that inspects page content and pushes a notification
// into the channel if the extracted text doesn't match the expected text.
func (d discordActions) Generate(actions chromedp.Tasks) chromedp.Tasks {
	actions = append(actions,
		chromedp.ActionFunc(func(ctx context.Context) error {
			if len(d.checkSelector) != 0 && len(d.expectedText) != 0 {
				res, err := extractData(ctx, d.checkSelector, d.checkType)
				if err != nil {
					return err
				}
				if !strings.Contains(res, d.expectedText) {
					Log().Infof("For URL [%s] found update: [%s] (expected: [%s]), sending Discord notification.", d.url, res, d.expectedText)
					go func() {
						d.postActionData <- discordData{URL: d.url, Text: res}
					}()
				} else {
					Log().Infof("For URL [%s] the result matches expected text.", d.url)
				}
			} else {
				Log().Infof("Condition met for URL [%s]; sending Discord notification.", d.url)
				go func() {
					d.postActionData <- discordData{URL: d.url}
				}()
			}
			return nil
		}),
	)
	return actions
}

// discordWatchFunc is responsible for sending notifications to Discord via a webhook.
type discordWatchFunc struct {
	webhookURL string
	username   string
}

// sendDiscordNotification sends a POST request to Discord with the payload, using the provided username.
func (d discordWatchFunc) sendDiscordNotification(data discordData) {
	payload := map[string]interface{}{
		"content":  fmt.Sprintf("URL: %s\nText: %s", data.URL, data.Text),
		"username": d.username,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		Log().Errorf("Error marshalling Discord payload: %v", err)
		return
	}

	Log().Infof("Sending payload to Discord: %s", string(jsonData))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(d.webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		Log().Errorf("Error sending Discord notification: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		Log().Errorf("Discord webhook returned unexpected status: %s", resp.Status)
	} else {
		Log().Infof("Discord notification sent successfully for URL: %s", data.URL)
	}
}

// DiscordContent sets up and starts the watch executor that monitors URLs and sends
// Discord notifications via the specified webhook and username.
func DiscordContent(cmd *cobra.Command) {
	viper.BindPFlags(cmd.Flags())

	// Get required configuration values.
	webhook := viper.GetString("webhook")
	if webhook == "" {
		Log().Errorf("Discord webhook URL must be provided via --webhook")
		return
	}
	username := viper.GetString("discord_username")
	if username == "" {
		username = "Go-Scraper Discord Alert"
	}

	// Retrieve common flags.
	urls := viper.GetStringSlice("urls")
	waitSelectors := viper.GetStringSlice("wait_selectors")
	checkSelectors := viper.GetStringSlice("check_selectors")
	checkTypes := viper.GetStringSlice("check_types")
	expectedTexts := viper.GetStringSlice("expected_texts")

	Log().Infof("Watching URLs: %v", urls)
	Log().Infof("Waiting on selectors: %v", waitSelectors)
	Log().Infof("Using check_selectors: %v", checkSelectors)
	Log().Infof("Using check_types: %v", checkTypes)
	Log().Infof("Using expected_texts: %v", expectedTexts)

	// Retrieve detection flags.
	detectAccessDeniedOn := viper.GetBool("detect_access_denied")
	detectCaptchaBoxOn := viper.GetBool("detect_captcha_box")
	captchaWaitSelector := viper.GetString("captcha_wait_selector")
	captchaClickSelector := viper.GetString("captcha_click_selector")
	captchaIframeWaitSelector := viper.GetString("captcha_iframe_wait_selector")
	captchaClickSleep := viper.GetInt("captcha_click_sleep")

	// Optionally detect notify-paths.
	detectNotifyPath := viper.GetBool("detect_notify_path")
	notifyPaths := viper.GetStringSlice("notify_paths")
	var notifyPath string
	if detectNotifyPath && len(notifyPaths) > 0 {
		notifyPath = notifyPaths[0] // Adapt as needed per URL.
	}

	// Error dump settings.
	errorDump := viper.GetBool("error_dump")
	errorLocation := viper.GetBool("error_location")
	redisDumpOn := viper.GetBool("redis_dumps")
	if redisDumpOn {
		setupRedis(cmd)
	}

	// Set up the Discord notification channel and notifier.
	discordMetaData := make(chan discordData)
	discordNotifier := discordWatchFunc{
		webhookURL: webhook,
		username:   username,
	}
	go func() {
		for {
			data := <-discordMetaData
			discordNotifier.sendDiscordNotification(data)
		}
	}()

	// Build action generators for each URL.
	actionGens := make([][]actionGenerator, len(urls))
	for i, u := range urls {
		actionGens[i] = make([]actionGenerator, 0)

		// Navigation action.
		actionGens[i] = append(actionGens[i], navigateActions{url: u})

		// Detection actions.
		actionGens[i] = append(actionGens[i], detectActions{
			url:                       u,
			detectAccessDenied:        detectAccessDeniedOn,
			detectCaptchaBox:          detectCaptchaBoxOn,
			captchaWaitSelector:       captchaWaitSelector,
			captchaClickSelector:      captchaClickSelector,
			captchaIframeWaitSelector: captchaIframeWaitSelector,
			captchaClickSleep:         captchaClickSleep,
			dumpOnError:               errorDump,
			locationOnError:           errorLocation,
			dumpToRedis:               redisDumpOn,
			notifyPath:                notifyPath,
			detectNotifyPath:          detectNotifyPath,
		})

		// Wait action.
		actionGens[i] = append(actionGens[i], waitActions{
			url:             u,
			waitSelector:    waitSelectors[i],
			dumpOnError:     errorDump,
			locationOnError: errorLocation,
			dumpToRedis:     redisDumpOn,
		})

		// Discord notification action.
		da := discordActions{
			postActionData: discordMetaData,
			url:            u,
		}
		if len(checkSelectors) > i && len(checkTypes) > i && len(expectedTexts) > i {
			da.checkSelector = checkSelectors[i]
			da.checkType = checkTypes[i]
			da.expectedText = expectedTexts[i]
		}
		actionGens[i] = append(actionGens[i], da)
	}

	// Initialize and execute the watch executor.
	e := executors["watch"].(*watchExecutor)
	e.Init(actionGens, urls)
	Log().Infof("Starting Discord watch executor for URLs: %v", urls)
	e.Execute() // Blocks indefinitely.
}
