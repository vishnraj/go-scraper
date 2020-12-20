package fetcher

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net/smtp"
	"time"

	"github.com/apsdehal/go-logger"
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

	gLog *logger.Logger
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
	errorDumps      chan dumpData
}

type dumpActions struct {
	postActionData chan dumpData

	textSelector string

	url string
}

type emailActions struct {
	postActionData chan emailData

	checkSelector string
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
	errorDumps  chan dumpData
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
			}
			return err
		}))
	return actions
}

func (w waitActions) Generate(actions chromedp.Tasks) chromedp.Tasks {
	if len(w.waitSelector) != 0 {
		actions = append(actions,
			chromedp.ActionFunc(func(ctx context.Context) error {
				err := chromedp.WaitVisible(w.waitSelector).Do(ctx)
				if err != nil {
					Log().Errorf("%v", err)
				}

				if err != nil && (w.dumpOnError || w.locationOnError) {
					select {
					case d := <-w.errorDumps:
						if w.dumpOnError {
							Log().Errorf("Dumping content for URL [%s]:", w.url)
							fmt.Printf("%s", d.ExtractText)
						}
						if w.locationOnError {
							Log().Errorf("Logging the current URL location as [%s] for our original target [%s]", d.URL, w.url)
						}
					default:
						Log().Errorf("No content to dump for wait failure for URL [%s]", w.url)
					}
				} else if err == nil && (w.dumpOnError || w.locationOnError) {
					// just read off the channel, so it's not there later
					select {
					case <-w.errorDumps:
					default:
					}
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
				err = chromedp.Text(d.textSelector, &res).Do(ctx)
				if err != nil {
					Log().Errorf("%v", err)
					return err
				}
			} else {
				// by default, this will grab pretty much everything
				var tmp string

				err = chromedp.OuterHTML(`head`, &tmp, chromedp.ByQuery).Do(ctx)
				if err != nil {
					Log().Errorf("%v", err)
					return err
				}
				res += tmp

				err = chromedp.OuterHTML(`body`, &tmp, chromedp.ByQuery).Do(ctx)
				if err != nil {
					Log().Errorf("%v", err)
					return err
				}
				res += tmp
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
				var res string
				err := chromedp.Text(e.checkSelector, &res).Do(ctx)
				if err != nil {
					Log().Errorf("%v", err)
					return err
				}

				if res == e.expectedText {
					go func() {
						e.postActionData <- emailData{URL: e.url, Text: res}
					}()
				} else {
					Log().Infof("Result found for URL [%s] was [%s] instead of [%s]", e.url, res, e.expectedText)
				}
			} else {
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
		err := run(a)
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
		err := run(a)
		if err != nil {
			Log().Errorf("Data for %s was not available during this check - received error %s\n", w.urls[i], err.Error())
		}
	}
	ticker := time.NewTicker(time.Duration(w.interval) * time.Second)
	for {
		select {
		case _ = <-ticker.C:
			for i, a := range w.actions {
				err := run(a)
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

func getAgent(agents []string) string {
	rand.Seed(time.Now().UTC().UnixNano())
	rand.Shuffle(len(agents), func(i, j int) {
		agents[i], agents[j] = agents[j], agents[i]
	})
	index := rand.Intn(len(agents))
	return agents[index]
}

func setOpt() ([]func(*chromedp.ExecAllocator), error) {
	agents := viper.GetStringSlice("agents")
	if len(agents) == 0 {
		agents = DefaultUserAgents
	}
	Log().Infof("Running with agents [%s]", agents)
	agent := getAgent(agents)
	Log().Infof("Selected with agent [%s]", agent)

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

func run(actions chromedp.Tasks) error {
	opts, err := setOpt()
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

// CommonWatchChecks checks if the common required flags for watch command are present - sub-commands check their own specific flags separately
func CommonWatchChecks(cmd *cobra.Command) error {
	viper.BindPFlags(cmd.Flags())

	urls := viper.GetStringSlice("urls")
	if urls == nil {
		return fmt.Errorf("We require a non-empty comma separated slice of URL(s)")
	}

	waitSelectors := viper.GetStringSlice("wait_selectors")
	if waitSelectors == nil {
		return fmt.Errorf("We require a non-empty comma separated slice of selector(s)")
	}

	if len(urls) == 0 || len(urls) != len(waitSelectors) {
		return fmt.Errorf("Number of URLs and selectors passed in must have the same length and be non-zero")
	}

	// these are optional
	if (viper.IsSet("check_selectors") && !viper.IsSet("expected_texts")) || (!viper.IsSet("check_selectors") && viper.IsSet("expected_texts")) {
		return fmt.Errorf("Specify both check_selectors and expected_texts or neither")
	} else if viper.IsSet("check_selectors") {
		checkSelectors := viper.GetStringSlice("check_selectors")
		expectedTexts := viper.GetStringSlice("expected_texts")
		if len(urls) != len(checkSelectors) || len(checkSelectors) != len(expectedTexts) {
			return fmt.Errorf("expected_texts and check_selectors must be the same length and match the number of URLs specified, invalid check_selectors length [%d], expected_texts length [%d] and URLs length is [%d]", len(checkSelectors), len(expectedTexts), len(urls))
		}
	}

	return nil
}

// PrintContent fetches HTML content
func PrintContent(cmd *cobra.Command) {
	viper.BindPFlags(cmd.Flags())

	u := viper.GetString("url")
	w := viper.GetString("wait_selector")

	waitErrorDump := viper.GetBool("wait_error_dump")
	waitErrorLocation := viper.GetBool("wait_error_location")

	Log().Infof("Fetching content from: %s", u)
	if len(w) != 0 {
		Log().Infof("Waiting on selector: %s", w)
	}

	t := viper.GetString("text_selector")

	if len(t) != 0 {
		Log().Infof("Will print text for %s", t)
	}

	if waitErrorDump {
		Log().Info("Will dump out HTML page content on wait errors")
	}
	if waitErrorLocation {
		Log().Info("Will log the current URL location on wait errors")
	}

	fetchDumps := make(chan dumpData)
	errorDumps := make(chan dumpData)
	actionGens := make([][]actionGenerator, 0)
	actionGens = append(actionGens, make([]actionGenerator, 0))

	actionGens[0] = append(actionGens[0], navigateActions{url: u})
	if waitErrorDump || waitErrorLocation {
		actionGens[0] = append(actionGens[0], dumpActions{postActionData: errorDumps, url: u})
	}
	actionGens[0] = append(actionGens[0], waitActions{url: u, waitSelector: w, dumpOnError: waitErrorDump, locationOnError: waitErrorLocation, errorDumps: errorDumps})
	actionGens[0] = append(actionGens[0], dumpActions{postActionData: fetchDumps, textSelector: t, url: u})

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

	Log().Infof("Sending with subject %s", subject)
	Log().Infof("Sending from email %s", from)
	Log().Infof("Sending to email %s", to)

	Log().Infof("Watching URLs %v", urls)
	Log().Infof("Waiting on selectors %v", waitSelectors)

	if waitErrorDump {
		Log().Info("Will dump out HTML page content on wait errors")
	}
	if waitErrorLocation {
		Log().Info("Will log the current URL location on wait errors")
	}

	var checkSelectors []string
	var expectedTexts []string
	if viper.IsSet("check_selectors") && viper.IsSet("expected_texts") {
		checkSelectors = viper.GetStringSlice("check_selectors")
		expectedTexts = viper.GetStringSlice("expected_texts")

		Log().Infof("Using check_selectors %v", checkSelectors)
		Log().Infof("Using expected_texts %v", expectedTexts)
	}

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
		errorDumps := make(chan dumpData)
		actionGens[i] = append(actionGens[i], navigateActions{url: u})
		if waitErrorDump || waitErrorLocation {
			actionGens[i] = append(actionGens[i], dumpActions{postActionData: errorDumps, url: u})
		}
		actionGens[i] = append(actionGens[i], waitActions{url: u, waitSelector: waitSelectors[i], dumpOnError: waitErrorDump, locationOnError: waitErrorLocation, errorDumps: errorDumps})
		e := emailActions{postActionData: emailMetaData, url: u}
		if checkSelectors != nil && expectedTexts != nil {
			e.checkSelector = checkSelectors[i]
			e.expectedText = expectedTexts[i]
		}
		actionGens[i] = append(actionGens[i], e)
	}

	e := executors["watch"].(*watchExecutor)
	e.Init(actionGens, urls)
	e.Execute() // blocks
}
