package fetcher

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/apsdehal/go-logger"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// DefaultUserDataDir default user data dir for non-headless mode
	DefaultUserDataDir = "/tmp/chrome_dev_1"

	// DefaultInterval default time to wait (in seconds) when watching a selector
	DefaultInterval = 30

	// DefaultRedisWriteTimeout default timeout (seconds) for writing to redis
	DefaultRedisWriteTimeout = 10

	// DefaultSubject to send email with
	DefaultSubject = "Go-Scraper Watcher"

	// DefaultCaptchaWaitSelector default to wait for captcha box on block
	DefaultCaptchaWaitSelector = `div.re-captcha`

	// DefaultCaptchaClickSelector default to click once captcha box appears
	DefaultCaptchaClickSelector = `div.g-recaptcha`

	// DefaultCaptchaIframeWaitSelector default wait selector for captcha iframe
	DefaultCaptchaIframeWaitSelector = `body > div:nth-child(6) > div:nth-child(4) > iframe`

	// DefaultCaptchaClickSleep default time (seconds) we sleep after a captcha click, to allow the captcha challenge to get loaded into the iframe
	DefaultCaptchaClickSleep = 5

	accessDeniedMessage = "Access Denied"
)

var (
	// DefaultUserAgents The default user agents to send requests as
	DefaultUserAgents = []string{
		`Mozilla/5.0 (Macintosh; Intel Mac OS X 11_1_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.88 Safari/537.36`,
	}

	executors = map[string]actionExecutor{
		"fetch": &fetchExecutor{},
		"watch": &watchExecutor{},
	}

	gLog    *logger.Logger
	gAgents = []string{}

	gSelectedAgents = map[string]string{}
	gWorkingAgents  = map[string]string{}

	gWaitErrorDumps   = make(chan dumpData)
	gDetectErrorDumps = make(chan dumpData)
	gCaptchaDumps     = make(chan dumpData)
)

type actionGenerator interface {
	Generate(actions chromedp.Tasks) chromedp.Tasks
}

type actionExecutor interface {
	Init(actionGens [][]actionGenerator, urls []string)
	Execute()
}

type dumpData struct {
	URL         string
	ExtractText string
}

type emailData struct {
	URL  string
	Text string
}

type navigateActions struct {
	url string
}

type detectActions struct {
	url                string
	detectAccessDenied bool

	detectCaptchaBox          bool
	captchaWaitSelector       string // used for captcha checkbox
	captchaClickSelector      string // used to click captcha checkbox
	captchaIframeWaitSelector string // only if we get captcha challenge, to load it
	captchaClickSleep         int

	locationOnError bool
	dumpOnError     bool

	dumpToRedis bool

	detectNotifyPath bool
	notifyPath       string         // a url path/domain sequence that indicates a more unique circumstance that we might want to be notified about
	postActionEmail  chan emailData // only if we want to email on certain detection cases
}

type waitActions struct {
	url          string
	waitSelector string

	locationOnError bool
	dumpOnError     bool

	dumpToRedis bool
}

type dumpActions struct {
	postActionData chan dumpData

	textSelector string
	hrefSelector string
	idSelector   string

	url string
}

type emailActions struct {
	postActionData chan emailData

	checkSelector string
	checkType     string
	expectedText  string

	url string
}

type fetchExecutor struct {
	urls []string

	actions []chromedp.Tasks
	errs    chan error
}

type watchExecutor struct {
	interval int
	urls     []string
	actions  []chromedp.Tasks

	dumpOnError bool
}

type emailWatchFunc struct {
	senderPassword string
	fromEmail      string
	toEmail        string
	toSubject      string
}

type pageSnaps struct {
	targetURL        string
	checkLocation    bool
	dumpPageContents bool

	dumpCaptcha               bool
	captchaIframeWaitSelector string
	captchaClickSleep         int

	sendDumps bool
	dumps     chan dumpData

	currentURL string
	pageDump   string

	dumpOnError bool
}

func (n navigateActions) Generate(actions chromedp.Tasks) chromedp.Tasks {
	actions = append(actions,
		chromedp.ActionFunc(func(ctx context.Context) error {
			Log().Infof("Navigating to URL [%s]", n.url)
			err := chromedp.Navigate(n.url).Do(ctx)
			if err != nil {
				Log().Errorf("%v", err)
				// suspecting it's a user agent issue, so we unset any working agent that may have failed at this point
				if len(gWorkingAgents[n.url]) != 0 {
					Log().Errorf("User-agent [%s] for URL [%s] no longer working, will unset it and try a different one on the next request", gWorkingAgents[n.url], n.url)
					gWorkingAgents[n.url] = ""
				}
			} else {
				// if a navigate succeeded, we pick the selected agent from that request as the working one
				if len(gWorkingAgents[n.url]) == 0 {
					Log().Infof("User-agent [%s] for URL [%s] succeeded, so it will be set as the current working agent", gSelectedAgents[n.url], n.url)
					gWorkingAgents[n.url] = gSelectedAgents[n.url]
				}
			}

			return err
		}))
	return actions
}

func (d detectActions) Generate(actions chromedp.Tasks) chromedp.Tasks {
	if d.detectNotifyPath && len(d.notifyPath) != 0 {
		actions = append(actions,
			chromedp.ActionFunc(func(ctx context.Context) error {
				Log().Infof("For URL [%s] check for path [%s] in URL that we should notify on", d.url, d.notifyPath)

				var currentURL string
				err := chromedp.Location(&currentURL).Do(ctx)
				if err != nil {
					Log().Errorf("%v", err)
					return err
				}

				if strings.Contains(currentURL, d.notifyPath) {
					err = fmt.Errorf("Found [%s] path in URL [%s], for target URL [%s] so we are performing notify action(s) that are set", d.notifyPath, currentURL, d.url)

					go func() {
						if d.postActionEmail != nil {
							d.postActionEmail <- emailData{URL: d.url, Text: currentURL}
						}
					}()
					return err
				}

				Log().Infof("For target URL [%s] the current URL [%s] didn't find path [%s] in URL that we should notify on so proceeding to next action", d.url, currentURL, d.notifyPath)

				return nil
			}))
	}
	if d.detectAccessDenied {
		actions = append(actions,
			chromedp.ActionFunc(func(ctx context.Context) error {
				s := pageSnaps{targetURL: d.url, checkLocation: d.locationOnError, dumpPageContents: true, sendDumps: d.dumpToRedis, dumps: gDetectErrorDumps, dumpOnError: d.dumpOnError}

				err := s.before(ctx)
				if err != nil {
					Log().Errorf("%v", err)
					return err
				}

				var title string
				err = chromedp.Title(&title).Do(ctx)
				err = s.after(ctx, err)
				if err != nil {
					Log().Errorf("%v", err)
					return err
				}

				if !strings.Contains(title, accessDeniedMessage) {
					Log().Infof("Didn't find the [%s] message on the page for URL [%s], nothing to do here, proceeding to next step", accessDeniedMessage, d.url)
					return nil
				}

				Log().Infof("Encountered [%s] message, will unset the current user-agent for this URL [%s], which is currently [%s] so we try a different one during the next request", accessDeniedMessage, d.url, gWorkingAgents[d.url])
				gWorkingAgents[d.url] = ""

				return err
			}))
	}
	if d.detectCaptchaBox {
		actions = append(actions,
			chromedp.ActionFunc(func(ctx context.Context) error {
				s := pageSnaps{targetURL: d.url, checkLocation: d.locationOnError, dumpPageContents: true, sendDumps: d.dumpToRedis, dumps: gDetectErrorDumps, dumpOnError: d.dumpOnError}

				err := s.before(ctx)
				if err != nil {
					Log().Errorf("%v", err)
					return err
				}

				if d.url == s.currentURL {
					Log().Infof("No location change detected for target URL [%s] so we will not try to detect a captcha box", d.url)
					return nil
				}
				Log().Infof("Detected location change for target URL [%s] to current URL [%s] so we will proceed to check for a captcha box", d.url, s.currentURL)

				Log().Infof("Waiting for captcha box for URL [%s] using selector [%s]", d.url, d.captchaWaitSelector)
				err = chromedp.WaitVisible(d.captchaWaitSelector).Do(ctx)
				err = s.after(ctx, err)
				if err != nil {
					Log().Errorf("%v", err)
					return err
				}

				Log().Infof("Wait complete, captcha box loaded, clicking captcha box for URL [%s] using selector [%s] using mouse click method", d.url, d.captchaClickSelector)
				err = chromedp.Click(d.captchaClickSelector).Do(ctx)
				if err != nil {
					Log().Errorf("%v", err)

					Log().Info("Failed to click captcha via mouse, trying ENTER key method")
					err = chromedp.SendKeys(d.captchaClickSelector, kb.Enter).Do(ctx)
					err = s.after(ctx, err)
					if err != nil {
						Log().Errorf("%v", err)
						return err
					}
				}

				Log().Infof("Successfully clicked captcha box for URL [%s] using selector [%s]", d.url, d.captchaClickSelector)

				Log().Infof("Check if the block URL [%s] for target URL [%s] has been updated back to target and if not, we will dump captcha contents", s.currentURL, d.url)
				err = s.before(ctx)
				if err != nil {
					Log().Errorf("%v", err)
					return err
				}

				if d.url != s.currentURL {
					Log().Infof("Current URL is [%s], which is not target URL [%s], so we're still blocked - waiting on captcha challenge using selector [%s]", s.currentURL, d.url, d.captchaIframeWaitSelector)
					err = chromedp.WaitVisible(d.captchaIframeWaitSelector).Do(ctx)
					err = s.after(ctx, err)
					if err != nil {
						Log().Errorf("%v", err)
						return err
					}
					Log().Infof("Captcha for URL [%s] loaded", d.url)

					c := pageSnaps{targetURL: d.url, checkLocation: false, dumpCaptcha: true, sendDumps: d.dumpToRedis, dumps: gCaptchaDumps, captchaIframeWaitSelector: d.captchaIframeWaitSelector, captchaClickSleep: d.captchaClickSleep, dumpOnError: d.dumpOnError}
					err = c.before(ctx)
					if err != nil {
						err = s.after(ctx, err)
						if err != nil {
							Log().Errorf("%v", err)
							return err
						}

						Log().Errorf("%v", err)
						return err
					}
					err = fmt.Errorf("Successfully loaded the captcha challenge, but we are still blocked by it, so we are just going to error out")
					err = c.after(ctx, err)
					if err != nil {
						Log().Errorf("%v", err)
					}
				}

				return err
			}))
	}

	return actions
}

func (w waitActions) Generate(actions chromedp.Tasks) chromedp.Tasks {
	if len(w.waitSelector) != 0 {
		actions = append(actions,
			chromedp.ActionFunc(func(ctx context.Context) error {
				s := pageSnaps{targetURL: w.url, checkLocation: w.locationOnError, dumpPageContents: true, sendDumps: w.dumpToRedis, dumps: gWaitErrorDumps, dumpOnError: w.dumpOnError}

				err := s.before(ctx)
				if err != nil {
					Log().Errorf("%v", err)
				}

				Log().Infof("Waiting on selector [%s] for URL [%s]", w.waitSelector, w.url)
				err = chromedp.WaitVisible(w.waitSelector).Do(ctx)
				err = s.after(ctx, err)
				if err != nil {
					Log().Errorf("%v", err)
				}

				return err
			}))
	}

	return actions
}

func (d dumpActions) Generate(actions chromedp.Tasks) chromedp.Tasks {
	actions = append(actions,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var currentURL string
			var res string
			var err error

			err = chromedp.Location(&currentURL).Do(ctx)
			if err != nil {
				Log().Errorf("%v", err)
				return err
			}

			if len(d.textSelector) != 0 {
				res, err = extractData(ctx, d.textSelector, "text")
				if err != nil {
					return err
				}
			} else if len(d.hrefSelector) != 0 {
				res, err = extractData(ctx, d.hrefSelector, "href")
				if err != nil {
					return err
				}
			} else if len(d.idSelector) != 0 {
				res, err = extractData(ctx, d.idSelector, "id")
				if err != nil {
					return err
				}
			} else {
				// by default, this will grab pretty much everything
				res, err = extractData(ctx, "", "dump")
				if err != nil {
					return err
				}
			}

			go func() {
				d.postActionData <- dumpData{URL: currentURL, ExtractText: res}
			}()

			return err
		}))

	return actions
}

func (e emailActions) Generate(actions chromedp.Tasks) chromedp.Tasks {
	actions = append(actions,
		chromedp.ActionFunc(func(ctx context.Context) error {
			if len(e.checkSelector) != 0 && len(e.expectedText) != 0 {
				res, err := extractData(ctx, e.checkSelector, e.checkType)
				if err != nil {
					return err
				}

				if !strings.Contains(res, e.expectedText) {
					Log().Infof("Result found for URL [%s] was [%s], which has been updated from the original value of expected text [%s] so we will perform the desired action!", e.url, res, e.expectedText)
					go func() {
						e.postActionData <- emailData{URL: e.url, Text: res}
					}()
				} else {
					Log().Infof("Result found for URL [%s] was still [%s], which matches the expected text [%s], so we take no action", e.url, res, e.expectedText)
				}
			} else {
				Log().Infof("We were simply told to wait for a page load so we could take action for URL [%s] - this condition has been met, so we are now performing the desired action", e.url)
				go func() {
					e.postActionData <- emailData{URL: e.url}
				}()
			}

			return nil
		}))

	return actions
}

func (f *fetchExecutor) Init(actionGens [][]actionGenerator, urls []string) {
	f.urls = urls
	f.errs = make(chan error)

	for _, gens := range actionGens {
		a := make(chromedp.Tasks, 0)
		for _, g := range gens {
			a = g.Generate(a)
		}
		f.actions = append(f.actions, a)
	}
}

func (f *fetchExecutor) Execute() {
	for i, a := range f.actions {
		err := run(a, f.urls[i])
		if err != nil {
			Log().Errorf("For URL [%s], received error [%v]", f.urls[i], err)
		}
		go func() {
			f.errs <- err
		}()
	}
}

func (w *watchExecutor) Init(actionGens [][]actionGenerator, urls []string) {
	w.urls = urls
	w.interval = viper.GetInt("interval")

	Log().Infof("Will check for updates every %d seconds\n", w.interval)

	for _, gens := range actionGens {
		a := make(chromedp.Tasks, 0)
		for _, g := range gens {
			a = g.Generate(a)
		}
		w.actions = append(w.actions, a)
	}
}

func (w *watchExecutor) Execute() {
	for i, a := range w.actions {
		err := run(a, w.urls[i])
		if err != nil {
			Log().Errorf("Data for %s was not available during this check - received error %s\n", w.urls[i], err.Error())
		}
	}
	ticker := time.NewTicker(time.Duration(w.interval) * time.Second)
	for {
		select {
		case _ = <-ticker.C:
			for i, a := range w.actions {
				err := run(a, w.urls[i])
				if err != nil {
					Log().Errorf("Data for %s was not available during this check - received error %s\n", w.urls[i], err.Error())
				}
			}
		}
	}
}

func (e emailWatchFunc) sendEmail(data emailData) {
	smtpHost := "smtp.gmail.com"
	smtpPort := "465"

	auth := smtp.PlainAuth("", e.fromEmail, e.senderPassword, smtpHost)
	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         smtpHost,
	}

	conn, err := tls.Dial("tcp", smtpHost+":"+smtpPort, tlsconfig)
	if err != nil {
		Log().Errorf("%v", err)
		return
	}

	c, err := smtp.NewClient(conn, smtpHost)
	defer c.Quit()

	if err != nil {
		Log().Errorf("%v", err)
		return
	}
	if err = c.Auth(auth); err != nil {
		Log().Errorf("%v", err)
		return
	}
	if err = c.Mail(e.fromEmail); err != nil {
		Log().Errorf("%v", err)
		return
	}
	if err = c.Rcpt(e.toEmail); err != nil {
		Log().Errorf("%v", err)
		return
	}
	w, err := c.Data()
	if err != nil {
		Log().Errorf("%v", err)
		return
	}

	message := "To: " + e.toEmail + "\r\n" +
		"Subject: " + e.toSubject + "\r\n" +
		"\r\n" +
		"URL: " + data.URL + "\r\n"
	if len(data.Text) != 0 {
		message += "Text: " + data.Text + "\r\n"
	}

	_, err = w.Write([]byte(message))
	if err != nil {
		Log().Errorf("%v", err)
		return
	}
	err = w.Close()
	if err != nil {
		Log().Errorf("%v", err)
		return
	}

	Log().Infof("Emailed %s successfully\n", e.toEmail)
}

func extractData(ctx context.Context, selector string, selectorType string) (string, error) {
	var res string
	switch selectorType {
	case "text":
		err := chromedp.Text(selector, &res).Do(ctx)
		if err != nil {
			Log().Errorf("%v", err)
			return "", err
		}
		break
	case "href":
		var nodes []*cdp.Node
		err := chromedp.Nodes(selector, &nodes).Do(ctx)
		if err != nil {
			Log().Errorf("%v", err)
			return "", err
		}
		if len(nodes) == 0 {
			err = errors.New("No nodes returned for selector")
			Log().Errorf("%v", err)
			return "", err
		}
		res = nodes[0].AttributeValue("href")
		break
	case "id":
		err := chromedp.Text(selector, &res, chromedp.ByID).Do(ctx)
		if err != nil {
			Log().Errorf("%v", err)
			return "", err
		}
		break
	case "dump":
		// by default, this will grab pretty much everything
		var tmp string

		err := chromedp.OuterHTML(`head`, &tmp, chromedp.ByQuery).Do(ctx)
		if err != nil {
			Log().Errorf("%v", err)
			return "", err
		}
		res += tmp

		err = chromedp.OuterHTML(`body`, &tmp, chromedp.ByQuery).Do(ctx)
		if err != nil {
			Log().Errorf("%v", err)
			return "", err
		}
		res += tmp
		break
	case "none":
	default:
		err := errors.New("For none or default we do nothing, but we shouldn't be here since for non-supported cases or none cases, we don't have an expected text to check - we are not taking any action")
		Log().ErrorF("%v", err)
		return "", err
	}

	return res, nil
}

func (s *pageSnaps) before(ctx context.Context) error {
	Log().Debugf("Performing before page snap steps for URL [%s]", s.targetURL)
	if s.checkLocation {
		Log().Debugf("Getting location for URL [%s]", s.targetURL)
		err := chromedp.Location(&s.currentURL).Do(ctx)
		if err != nil {
			return err
		}
		Log().Debugf("Successfully got location for URL [%s]", s.targetURL)
	}
	if s.dumpPageContents {
		Log().Debugf("Extracting page contents for URL [%s]", s.targetURL)
		res, err := extractData(ctx, "", "dump")
		if err != nil {
			return err
		}
		s.pageDump = res
		Log().Debugf("Successfully extracted page contents for URL [%s]", s.targetURL)
	}
	if s.dumpCaptcha {
		Log().Infof("Finding iframe for captcha using URI [%s] for URL [%s]", s.captchaIframeWaitSelector, s.targetURL)

		Log().Infof("Sleeping for [%d] seconds after sleep to allow captcha challenge to load", s.captchaClickSleep)
		chromedp.Sleep(time.Duration(s.captchaClickSleep) * time.Second).Do(ctx)

		err := chromedp.EvaluateAsDevTools(`document.querySelector('`+s.captchaIframeWaitSelector+`').contentWindow.document.body.outerHTML;`, &s.pageDump).Do(ctx)
		if err != nil {
			Log().Errorf("%v", err)
			return err
		}
		Log().Infof("Successfully loaded captcha for URL [%s]", s.targetURL)
	}

	Log().Debugf("Done with before page snap steps for URL [%s]", s.targetURL)
	return nil
}

func (s *pageSnaps) after(ctx context.Context, err error) error {
	Log().Debugf("Performing after page snap steps for URL [%s]", s.targetURL)
	if err != nil && (s.dumpPageContents || s.dumpCaptcha) && s.dumpOnError {
		if s.sendDumps {
			Log().Errorf("Dumping content for URL [%s] to redis", s.targetURL)
			go func() {
				s.dumps <- dumpData{URL: s.targetURL, ExtractText: s.pageDump}
			}()
		} else {
			Log().Errorf("Dumping content for URL [%s] to stdout:", s.targetURL)
			fmt.Printf("%s", s.pageDump)
		}
	}
	if err != nil && s.checkLocation {
		Log().Errorf("Logging the current URL location as [%s] for our original target [%s]", s.currentURL, s.targetURL)
	}

	Log().Debugf("Done with after page snap steps for URL [%s]", s.targetURL)
	return err
}

func getAgent(agents []string) string {
	rand.Seed(time.Now().UTC().UnixNano())
	rand.Shuffle(len(agents), func(i, j int) {
		agents[i], agents[j] = agents[j], agents[i]
	})
	index := rand.Intn(len(agents))
	return agents[index]
}

func setOpt(targetURL string) ([]func(*chromedp.ExecAllocator), error) {
	var agent string
	if len(gWorkingAgents[targetURL]) == 0 {
		gSelectedAgents[targetURL] = getAgent(gAgents)
		agent = gSelectedAgents[targetURL]
		Log().Infof("No working agent for URL [%s], so using selected user-agent [%s] for this attempt", targetURL, agent)
	} else {
		agent = gWorkingAgents[targetURL]
		Log().Infof("Last working agent was [%s] for URL [%s], so will continue using it", agent, targetURL)
	}

	runHeadless := viper.GetBool("headless")
	var opts []func(*chromedp.ExecAllocator)
	if !runHeadless {
		userDataDir := viper.GetString("user_data_dir")
		Log().Infof("Running without headless enabled, using user_data_dir [%s]", userDataDir)
		opts = []chromedp.ExecAllocatorOption{
			chromedp.UserAgent(agent),
			chromedp.NoFirstRun,
			chromedp.NoDefaultBrowserCheck,
			chromedp.Flag("user-data-dir", userDataDir),
			chromedp.Flag("disable-site-isolation-trials", true),
			chromedp.Flag("disable-web-security", true),
			chromedp.Flag("test-type", true),
		}
	} else {
		Log().Info("Running in headless mode")
		opts = []chromedp.ExecAllocatorOption{
			chromedp.UserAgent(agent),
			chromedp.Flag("headless", true),
			chromedp.Flag("hide-scrollbars", true),
			chromedp.Flag("mute-audio", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("allow-insecure-localhost", true),
			chromedp.Flag("ignore-certificate-errors", true),
			chromedp.Flag("disable-web-security", true),
		}
	}

	return opts, nil
}

func createChromeContext(opts []func(*chromedp.ExecAllocator)) (context.Context, context.CancelFunc) {
	var ctx context.Context
	var cancel context.CancelFunc

	ctx = context.Background()
	timeout := viper.GetInt("timeout")
	if timeout > 0 {
		Log().Infof("Timeout specified: %ds\n", timeout)
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	}

	ctx, _ = chromedp.NewExecAllocator(ctx, opts...)
	ctx, cancel = chromedp.NewContext(ctx)

	return ctx, cancel
}

func run(actions chromedp.Tasks, targetURL string) error {
	opts, err := setOpt(targetURL)
	if err != nil {
		return err
	}

	// perhaps add a check for an option that lets us reuse contexts
	// between calls - this may involved saving the first one we init
	// and reusing it in callers, but we'll leave this for now
	// as it suits most of the current use cases
	ctx, cancel := createChromeContext(opts)
	defer cancel()

	err = chromedp.Run(ctx, actions...)
	return err
}

func setupRedis(cmd *cobra.Command) {
	viper.BindPFlags(cmd.Flags())

	redisURL := viper.GetString("redis_url")
	redisPassword := viper.GetString("redis_password")
	redisKeyExpiration := viper.GetInt("redis_key_expiration")
	redisWriteTimeout := viper.GetInt("redis_write_timeout")

	Log().Infof("Dumps will be logged to redis instance running at [%s]", redisURL)
	Log().Infof("Redis key expiration set to [%d] seconds", redisKeyExpiration)
	Log().Infof("Redis write timeout set to [%d] seconds", redisWriteTimeout)

	go redisWorker(redisURL, redisPassword, redisKeyExpiration, redisWriteTimeout)
}

func redisWrite(client *redis.Client, data string, keyPrefix string, url string, redisKeyExpiration int) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	key := keyPrefix + "-" + timestamp + "-" + url
	err := client.Set(client.Context(), key, data, time.Duration(redisKeyExpiration)*time.Second).Err()
	if err == nil {
		Log().Infof("For key [%s] redis write was successful", key)
	} else {
		Log().Errorf("For key [%s] error encountered during write: [%v]", key, err)
	}
}

func redisWorker(redisURL string, redisPassword string, redisKeyExpiration int, redisWriteTimeout int) {
	client := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
	})
	client.WithTimeout(time.Duration(redisWriteTimeout))
	for {
		select {
		case d := <-gWaitErrorDumps:
			{
				redisWrite(client, d.ExtractText, "wait-errors", d.URL, redisKeyExpiration)
				break
			}
		case d := <-gDetectErrorDumps:
			{
				redisWrite(client, d.ExtractText, "detect-errors", d.URL, redisKeyExpiration)
				break
			}
		case d := <-gCaptchaDumps:
			{
				redisWrite(client, d.ExtractText, "catpcha-dumps", d.URL, redisKeyExpiration)
				break
			}
		}
	}
}

// Log creates a logger that we can use in the app
func Log() *logger.Logger {
	if gLog == nil {
		var err error
		if gLog, err = logger.New(0); err != nil {
			panic(err)
		}
		gLog.SetFormat("[%{level}] [%{time}] %{filename}:%{line}: %{message}")

		level := viper.GetString("log_level")
		switch level {
		case "DEBUG":
			gLog.Infof("Setting log level to [%s]", level)
			gLog.SetLogLevel(logger.DebugLevel)
			break
		case "INFO":
		case "":
		default:
			gLog.Info("Log level is set to [INFO] by default")
		}
	}

	return gLog
}

// CommonRootChecks does checks for flags for root commands
func CommonRootChecks(cmd *cobra.Command) error {
	viper.BindPFlags(cmd.Flags())

	if !viper.GetBool("headless") && len(viper.GetString("user_data_dir")) == 0 {
		return fmt.Errorf("If we are not running in headless mode, we need to specify a non-empty user_data_dir")
	}

	gAgents = viper.GetStringSlice("agents")
	if len(gAgents) == 0 {
		Log().Info("No user agents specified, setting to default")
		gAgents = DefaultUserAgents
	}
	Log().Infof("Running with [%d] user-agents: [%s]", len(gAgents), gAgents)

	if viper.GetBool("redis_dumps") && !viper.IsSet("redis_url") {
		return fmt.Errorf("We require a valid redis_url to dump to redis, specify one")
	}

	if viper.GetBool("detect_captcha_box") && (len(viper.GetString("captcha_wait_selector")) == 0 || len(viper.GetString("captcha_click_selector")) == 0 || len(viper.GetString("captcha_iframe_wait_selector")) == 0 ||
		!viper.GetBool("error_location")) {
		return fmt.Errorf("If we want to detect a captcha box, we must detect error_location as well to compare the current location to target location to be sure a captcha may exist and we must specify a non-empty captcha_wait_selector, captcha_click_selector, captcha_iframe_wait_selector, captcha_iframe_uri and captcha_challenge_wait_selector or leave defaults - pass accepted values for those flags to run with this flag")
	}

	return nil
}

// CommonWatchChecks checks if the common required flags for watch command are present - sub-commands check their own specific flags separately
func CommonWatchChecks(cmd *cobra.Command) error {
	viper.BindPFlags(cmd.Flags())

	urls := viper.GetStringSlice("urls")
	if len(urls) == 0 {
		return fmt.Errorf("We require a non-empty slice of URLs")
	}
	waitSelectors := viper.GetStringSlice("wait_selectors")
	if len(waitSelectors) == 0 {
		return fmt.Errorf("We require a non-empty slice of wait_selectors")
	}

	checkSelectors := viper.GetStringSlice("check_selectors")
	if len(checkSelectors) == 0 {
		return fmt.Errorf("We require a non-empty slice of check_selectors")
	}
	checkTypes := viper.GetStringSlice("check_types")
	if len(checkTypes) == 0 {
		return fmt.Errorf("We require a non-empty slice of check_types")
	}
	expectedTexts := viper.GetStringSlice("expected_texts")
	if len(expectedTexts) == 0 {
		return fmt.Errorf("We require a non-empty slice of expected_texts")
	}

	if len(urls) != len(waitSelectors) {
		return fmt.Errorf("Number of URLs and wait_selectors passed in must have the same length")
	}
	if len(urls) != len(checkSelectors) {
		return fmt.Errorf("Number of URLs and check_selectors passed in must have the same length")
	}
	if len(urls) != len(checkTypes) {
		return fmt.Errorf("Number of URLs and check_types passed in must have the same length")
	}
	if len(urls) != len(expectedTexts) {
		return fmt.Errorf("Number of URLs and expected_texts passed in must have the same length")
	}

	if viper.GetBool("detect_captcha_box") {
		captchaWaitSelectors := viper.GetStringSlice("captcha_wait_selectors")
		if len(captchaWaitSelectors) == 0 {
			return fmt.Errorf("We require a non-empty slice of captcha_wait_selectors")
		}
		captchaClickSelectors := viper.GetStringSlice("captcha_click_selectors")
		if len(captchaClickSelectors) == 0 {
			return fmt.Errorf("We require a non-empty slice of captcha_click_selectors")
		}
		captchaIframeWaitSelectors := viper.GetStringSlice("captcha_iframe_wait_selectors")
		if len(captchaClickSelectors) == 0 {
			return fmt.Errorf("We require a non-empty slice of captcha_iframe_wait_selectors")
		}

		if len(urls) != len(captchaWaitSelectors) {
			return fmt.Errorf("Number of URLs and captcha_wait_selectors passed in must have the same length")
		}
		if len(urls) != len(captchaClickSelectors) {
			return fmt.Errorf("Number of URLs and captcha_click_selectors passed in must have the same length")
		}
		if len(urls) != len(captchaIframeWaitSelectors) {
			return fmt.Errorf("Number of URLs and captcha_iframe_wait_selectors passed in must have the same length")
		}
	}

	return CommonRootChecks(cmd)
}

// PrintContent fetches HTML content
func PrintContent(cmd *cobra.Command) {
	viper.BindPFlags(cmd.Flags())

	u := viper.GetString("url")
	w := viper.GetString("wait_selector")

	Log().Infof("Fetching content from: [%s]", u)
	if len(w) != 0 {
		Log().Infof("Waiting on selector: [%s]", w)
	}

	t := viper.GetString("text_selector")
	if len(t) != 0 {
		Log().Infof("Will print text for: [%s]", t)
	}
	h := viper.GetString("href_selector")
	if len(h) != 0 {
		Log().Infof("Will dump data for href selector: [%s]", h)
	}
	id := viper.GetString("id_selector")
	if len(id) != 0 {
		Log().Infof("Will dump data for id selector: [%s]", id)
	}

	detectAccessDeniedOn := viper.GetBool("detect_access_denied")
	if detectAccessDeniedOn {
		Log().Info("Taking action against access denied")
	}

	detectCaptchaBoxOn := viper.GetBool("detect_captcha_box")
	captchaWaitSelector := viper.GetString("captcha_wait_selector")
	captchaClickSelector := viper.GetString("captcha_click_selector")
	captchaIframeWaitSelector := viper.GetString("captcha_iframe_wait_selector")
	captchaClickSleep := viper.GetInt("captcha_click_sleep")
	if detectCaptchaBoxOn {
		Log().Infof("Taking action against captcha boxes using default wait selector [%s], box selector [%s], iframe wait selector [%s] and captcha click seconds [%d]", captchaWaitSelector, captchaClickSelector, captchaIframeWaitSelector, captchaClickSleep)
	}

	errorDump := viper.GetBool("error_dump")
	errorLocation := viper.GetBool("error_location")
	if errorDump {
		Log().Info("Will dump out HTML page content on wait errors")
	}
	if errorLocation {
		Log().Info("Will log the current URL location on wait errors")
	}

	redisDumpOn := viper.GetBool("redis_dumps")
	if redisDumpOn {
		setupRedis(cmd)
	}

	fetchDumps := make(chan dumpData)
	actionGens := make([][]actionGenerator, 0)
	actionGens = append(actionGens, make([]actionGenerator, 0))

	actionGens[0] = append(actionGens[0], navigateActions{url: u})
	actionGens[0] = append(actionGens[0], detectActions{url: u, detectAccessDenied: detectAccessDeniedOn, detectCaptchaBox: detectCaptchaBoxOn, captchaWaitSelector: captchaWaitSelector, captchaClickSelector: captchaClickSelector, captchaIframeWaitSelector: captchaIframeWaitSelector, captchaClickSleep: captchaClickSleep, dumpOnError: errorDump, locationOnError: errorLocation, dumpToRedis: redisDumpOn})
	actionGens[0] = append(actionGens[0], waitActions{url: u, waitSelector: w, dumpOnError: errorDump, locationOnError: errorLocation, dumpToRedis: redisDumpOn})
	actionGens[0] = append(actionGens[0], dumpActions{postActionData: fetchDumps, textSelector: t, hrefSelector: h, idSelector: id, url: u})

	f := executors["fetch"].(*fetchExecutor)
	f.Init(actionGens, []string{u})
	f.Execute()
	if err := <-f.errs; err == nil {
		data := <-fetchDumps
		fmt.Printf(data.ExtractText)
	}
}

// EmailContent will watch content and take action if content is available
func EmailContent(cmd *cobra.Command) {
	viper.BindPFlags(cmd.Flags())

	subject := viper.GetString("subject")
	from := viper.GetString("from")
	to := viper.GetString("to")

	urls := viper.GetStringSlice("urls")
	waitSelectors := viper.GetStringSlice("wait_selectors")
	password := viper.GetString("email_password")

	Log().Infof("Using email subject: [%s]", subject)
	Log().Infof("Using from email: [%s]", from)
	Log().Infof("Using to email: [%s]", to)

	Log().Infof("Watching URLs: [%v]", urls)
	Log().Infof("Waiting on selectors: [%v]", waitSelectors)

	detectAccessDeniedOn := viper.GetBool("detect_access_denied")
	if detectAccessDeniedOn {
		Log().Info("Taking action against access denied")
	}

	detectCaptchaBoxOn := viper.GetBool("detect_captcha_box")
	captchaWaitSelector := viper.GetString("captcha_wait_selector")
	captchaClickSelector := viper.GetString("captcha_click_selector")
	captchaIframeWaitSelector := viper.GetString("captcha_iframe_wait_selector")
	captchaClickSleep := viper.GetInt("captcha_click_sleep")

	captchaOverrideWaitSelectors := viper.GetStringSlice("captcha_wait_selectors")
	captchaOverrideClickSelectors := viper.GetStringSlice("captcha_click_selectors")
	captchaOverrideIframeWaitSelectors := viper.GetStringSlice("captcha_iframe_wait_selectors")
	if detectCaptchaBoxOn {
		Log().Infof("Taking action against captcha boxes using default wait selector [%s], box selector [%s], iframe wait selector [%s] and captcha click seconds [%d]", captchaWaitSelector, captchaClickSelector, captchaIframeWaitSelector, captchaClickSleep)
		Log().Infof("Override captcha wait selectors: [%v]", captchaOverrideWaitSelectors)
		Log().Infof("Override captcha click selectors: [%v]", captchaOverrideClickSelectors)
		Log().Infof("Override captcha iframe wait selectors: [%v]", captchaOverrideIframeWaitSelectors)
	}

	detectNotifyPath := viper.GetBool("detect_notify_path")
	notifyPaths := viper.GetStringSlice("notify_paths")
	if detectNotifyPath {
		Log().Infof("Will detect notify URL Paths - desired paths: [%v]", notifyPaths)
	}

	errorDump := viper.GetBool("error_dump")
	errorLocation := viper.GetBool("error_location")

	if errorDump {
		Log().Info("Will dump out HTML page content on wait errors")
	}
	if errorLocation {
		Log().Info("Will log the current URL location on wait errors")
	}

	redisDumpOn := viper.GetBool("redis_dumps")
	if redisDumpOn {
		setupRedis(cmd)
	}

	checkSelectors := viper.GetStringSlice("check_selectors")
	checkTypes := viper.GetStringSlice("check_types")
	expectedTexts := viper.GetStringSlice("expected_texts")

	Log().Infof("Using check_selectors: [%v]", checkSelectors)
	Log().Infof("Using check_types: [%v]", checkTypes)
	Log().Infof("Using expected_texts: [%v]", expectedTexts)

	emailMetaData := make(chan emailData)
	postAction := emailWatchFunc{
		fromEmail:      from,
		toEmail:        to,
		toSubject:      subject,
		senderPassword: password,
	}
	go func() {
		for {
			data := <-emailMetaData
			postAction.sendEmail(data)
		}
	}()

	actionGens := make([][]actionGenerator, 0)
	for i := 0; i < len(urls); i++ {
		actionGens = append(actionGens, make([]actionGenerator, 0))
	}

	for i, u := range urls {
		var notifyPath string
		capWaitSelector := captchaWaitSelector
		capClickSelector := captchaClickSelector
		capIframeWaitSelector := captchaIframeWaitSelector

		overrideCapWaitSelector := captchaOverrideWaitSelectors[i]
		overrideCapClickSelector := captchaOverrideClickSelectors[i]
		overrideCapIframeWaitSelector := captchaOverrideIframeWaitSelectors[i]
		if len(overrideCapWaitSelector) != 0 {
			capWaitSelector = overrideCapWaitSelector
			Log().Infof("Using override captcha wait selector [%s] for URL [%s]", capWaitSelector, u)
		}
		if len(overrideCapClickSelector) != 0 {
			capClickSelector = overrideCapClickSelector
			Log().Infof("Using override captcha click selector [%s] for URL [%s]", capClickSelector, u)
		}
		if len(overrideCapIframeWaitSelector) != 0 {
			capIframeWaitSelector = overrideCapIframeWaitSelector
			Log().Infof("Using override iframe wait selector [%s] for URL [%s]", capIframeWaitSelector, u)
		}
		if len(notifyPaths) != 0 {
			notifyPath = notifyPaths[i]
		}

		actionGens[i] = append(actionGens[i], navigateActions{url: u})

		actionGens[i] = append(actionGens[i], detectActions{url: u, detectAccessDenied: detectAccessDeniedOn, detectCaptchaBox: detectCaptchaBoxOn, captchaWaitSelector: capWaitSelector, captchaClickSelector: capClickSelector, captchaIframeWaitSelector: capIframeWaitSelector, captchaClickSleep: captchaClickSleep, dumpOnError: errorDump, locationOnError: errorLocation, dumpToRedis: redisDumpOn, notifyPath: notifyPath, postActionEmail: emailMetaData, detectNotifyPath: detectNotifyPath})

		actionGens[i] = append(actionGens[i], waitActions{url: u, waitSelector: waitSelectors[i], dumpOnError: errorDump, locationOnError: errorLocation, dumpToRedis: redisDumpOn})

		e := emailActions{postActionData: emailMetaData, url: u}
		if checkSelectors != nil && expectedTexts != nil {
			e.checkSelector = checkSelectors[i]
			e.checkType = checkTypes[i]
			e.expectedText = expectedTexts[i]
		}
		actionGens[i] = append(actionGens[i], e)
	}

	e := executors["watch"].(*watchExecutor)
	e.Init(actionGens, urls)
	e.Execute() // blocks
}
