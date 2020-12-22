package fetcher

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"net/smtp"
	"strings"
	"time"

	"github.com/apsdehal/go-logger"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// DefaultInterval to wait (in seconds) when watching a selector
	DefaultInterval = 30

	// DefaultSubject to send email with
	DefaultSubject = "Go-Dynamic-Fetch Watcher"
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

type waitActions struct {
	url          string
	waitSelector string

	locationOnError bool
	dumpOnError     bool
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

func (n navigateActions) Generate(actions chromedp.Tasks) chromedp.Tasks {
	actions = append(actions,
		chromedp.ActionFunc(func(ctx context.Context) error {
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

func (w waitActions) Generate(actions chromedp.Tasks) chromedp.Tasks {
	if len(w.waitSelector) != 0 {
		actions = append(actions,
			chromedp.ActionFunc(func(ctx context.Context) error {
				var currentURL string
				var res string
				var err error

				if w.locationOnError {
					err = chromedp.Location(&currentURL).Do(ctx)
					if err != nil {
						Log().Errorf("%v", err)
						return err
					}
				}
				if w.dumpOnError {
					res, err = extractData(ctx, "", "dump")
					if err != nil {
						Log().Errorf("%v", err)
						return err
					}
				}

				err = chromedp.WaitVisible(w.waitSelector).Do(ctx)
				if err != nil {
					Log().Errorf("%v", err)
				}

				if err != nil && w.dumpOnError {
					Log().Errorf("Dumping content for URL [%s]:", w.url)
					fmt.Printf("%s", res)
				}
				if err != nil && w.locationOnError {
					Log().Errorf("Logging the current URL location as [%s] for our original target [%s]", currentURL, w.url)
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
		Log().Info("Running without headless enabled")
		opts = []chromedp.ExecAllocatorOption{
			chromedp.UserAgent(agent),
			chromedp.NoFirstRun,
			chromedp.NoDefaultBrowserCheck,
		}
	} else {
		Log().Info("Running in headless mode")
		opts = []chromedp.ExecAllocatorOption{
			chromedp.UserAgent(agent),
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("allow-insecure-localhost", true),
			chromedp.Flag("ignore-certificate-errors", true),
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

// Log creates a logger that we can use in the app
func Log() *logger.Logger {
	if gLog == nil {
		var err error
		if gLog, err = logger.New(0); err != nil {
			panic(err)
		}
		gLog.SetFormat("[%{level}] [%{time}] %{filename}:%{line}: %{message}")
	}

	return gLog
}

// CommonRootChecks does checks for flags for root commands
func CommonRootChecks(cmd *cobra.Command) error {
	viper.BindPFlags(cmd.Flags())

	gAgents = viper.GetStringSlice("agents")
	if len(gAgents) == 0 {
		Log().Info("No user agents specified, setting to default")
		gAgents = DefaultUserAgents
	}
	Log().Infof("Running with [%d] user-agents: [%s]", len(gAgents), gAgents)

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

	return CommonRootChecks(cmd)
}

// PrintContent fetches HTML content
func PrintContent(cmd *cobra.Command) {
	viper.BindPFlags(cmd.Flags())

	u := viper.GetString("url")
	w := viper.GetString("wait_selector")

	waitErrorDump := viper.GetBool("wait_error_dump")
	waitErrorLocation := viper.GetBool("wait_error_location")

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

	if waitErrorDump {
		Log().Info("Will dump out HTML page content on wait errors")
	}
	if waitErrorLocation {
		Log().Info("Will log the current URL location on wait errors")
	}

	fetchDumps := make(chan dumpData)
	actionGens := make([][]actionGenerator, 0)
	actionGens = append(actionGens, make([]actionGenerator, 0))

	actionGens[0] = append(actionGens[0], navigateActions{url: u})
	actionGens[0] = append(actionGens[0], waitActions{url: u, waitSelector: w, dumpOnError: waitErrorDump, locationOnError: waitErrorLocation})
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
	waitErrorDump := viper.GetBool("wait_error_dump")
	waitErrorLocation := viper.GetBool("wait_error_location")

	password := viper.GetString("email_password")

	Log().Infof("Using email subject: [%s]", subject)
	Log().Infof("Using from email: [%s]", from)
	Log().Infof("Using to email: [%s]", to)

	Log().Infof("Watching URLs: [%v]", urls)
	Log().Infof("Waiting on selectors: [%v]", waitSelectors)

	if waitErrorDump {
		Log().Info("Will dump out HTML page content on wait errors")
	}
	if waitErrorLocation {
		Log().Info("Will log the current URL location on wait errors")
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
		actionGens[i] = append(actionGens[i], navigateActions{url: u})
		actionGens[i] = append(actionGens[i], waitActions{url: u, waitSelector: waitSelectors[i], dumpOnError: waitErrorDump, locationOnError: waitErrorLocation})
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
