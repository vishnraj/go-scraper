package fetcher

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"time"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// DefaultInterval to wait (in seconds) when watching a selector
	DefaultInterval = 30

	// DefaultSubject to send email with
	DefaultSubject = "Go-Dynamic-Fetch Watcher"

	// DefaultUserAgent The default user agent to send request as
	DefaultUserAgent = "Moizilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36"
)

var (
	executors = map[string]actionExecutor{
		"fetch": &fetchExecutor{},
		"watch": &watchExecutor{},
	}
)

type actionGenerator interface {
	Generate(actions []chromedp.Action) []chromedp.Action
}

type actionExecutor interface {
	Init(cmd *cobra.Command, actionGens [][]actionGenerator)
	Execute(cmd *cobra.Command)
}

type watchFunc interface {
	Execute(metadata string)
}

type dumpData struct {
	ExtractText string
}

type emailData struct {
	URL string
}

type waitActions struct {
	url          string
	waitSelector string
}

type dumpActions struct {
	postActionData chan dumpData

	textSelector string
}

type emailActions struct {
	postActionData chan emailData

	url string
}

type fetchExecutor struct {
	actions [][]chromedp.Action
}

type watchExecutor struct {
	interval int
	urls     []string
	actions  [][]chromedp.Action
}

type emailWatchFunc struct {
	senderPassword string
	fromEmail      string
	toEmail        string
	toSubject      string
}

func (c waitActions) Generate(actions []chromedp.Action) []chromedp.Action {
	actions = append(actions, chromedp.Navigate(c.url))
	if len(c.waitSelector) != 0 {
		actions = append(actions, chromedp.WaitVisible(c.waitSelector, chromedp.ByQuery))
	}

	return actions
}

func (d dumpActions) Generate(actions []chromedp.Action) []chromedp.Action {
	if len(d.textSelector) != 0 {
		actions = append(actions,
			chromedp.ActionFunc(func(ctx context.Context) error {
				var res string
				err := chromedp.Text(d.textSelector, &res).Do(ctx)
				if err == nil {
					go func() {
						d.postActionData <- dumpData{ExtractText: res}
					}()
				} else {
					go func() {
						d.postActionData <- dumpData{}
					}()
				}

				return err
			}))
	} else {
		actions = append(actions,
			chromedp.ActionFunc(func(ctx context.Context) error {
				node, err := dom.GetDocument().Do(ctx)
				if err != nil {
					return err
				}

				res, err := dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
				if err == nil {
					go func() {
						d.postActionData <- dumpData{ExtractText: res}
					}()
				} else {
					go func() {
						d.postActionData <- dumpData{}
					}()
				}

				return err
			}))
	}

	return actions
}

func (e emailActions) Generate(actions []chromedp.Action) []chromedp.Action {
	actions = append(actions,
		chromedp.ActionFunc(func(ctx context.Context) error {
			go func() {
				e.postActionData <- emailData{URL: e.url}
			}()
			return nil
		}))

	return actions
}

func (f *fetchExecutor) Init(cmd *cobra.Command, actionGens [][]actionGenerator) {
	for _, gens := range actionGens {
		a := make([]chromedp.Action, 0)
		for _, g := range gens {
			a = g.Generate(a)
		}
		f.actions = append(f.actions, a)
	}
}

func (f fetchExecutor) Execute(cmd *cobra.Command) {
	for i, a := range f.actions {
		err := run(cmd, a)
		if err != nil {
			log.Printf("For URL [%d], received error [%v]", i, err)
		}
	}
}

func (w *watchExecutor) Init(cmd *cobra.Command, actionGens [][]actionGenerator) {
	viper.BindPFlags(cmd.Flags())

	w.interval = viper.GetInt("interval")
	log.Printf("Will check for updates every %d seconds\n", w.interval)

	for _, gens := range actionGens {
		a := make([]chromedp.Action, 0)
		for _, g := range gens {
			a = g.Generate(a)
		}
		w.actions = append(w.actions, a)
	}
}

func (w watchExecutor) Execute(cmd *cobra.Command) {
	for i, a := range w.actions {
		err := run(cmd, a)
		if err != nil {
			log.Printf("Data for %s was not available during this check - no email sent - received error %s\n", w.urls[i], err.Error())
		}
	}
	ticker := time.NewTicker(time.Duration(w.interval) * time.Second)
	for {
		select {
		case _ = <-ticker.C:
			for i, a := range w.actions {
				err := run(cmd, a)
				if err != nil {
					log.Printf("Data for %s was not available during this check - no email sent - received error %s\n", w.urls[i], err.Error())
				}
			}
		}
	}
}

func (e emailWatchFunc) Execute(metadata string) {
	smtpHost := "smtp.gmail.com"
	smtpPort := "465"

	auth := smtp.PlainAuth("", e.fromEmail, e.senderPassword, smtpHost)
	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         smtpHost,
	}

	conn, err := tls.Dial("tcp", smtpHost+":"+smtpPort, tlsconfig)
	if err != nil {
		log.Println(err)
		return
	}

	c, err := smtp.NewClient(conn, smtpHost)
	defer c.Quit()

	if err != nil {
		log.Println(err)
		return
	}
	if err = c.Auth(auth); err != nil {
		log.Println(err)
		return
	}
	if err = c.Mail(e.fromEmail); err != nil {
		log.Println(err)
		return
	}
	if err = c.Rcpt(e.toEmail); err != nil {
		log.Println(err)
		return
	}
	w, err := c.Data()
	if err != nil {
		log.Println(err)
		return
	}

	message := "To: " + e.toEmail + "\r\n" +
		"Subject: " + e.toSubject + "\r\n" +
		"\r\n" +
		metadata + "\r\n"

	_, err = w.Write([]byte(message))
	if err != nil {
		log.Println(err)
		return
	}
	err = w.Close()
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("Emailed %s successfully\n", e.toEmail)
}

func setOpt(cmd *cobra.Command) ([]func(*chromedp.ExecAllocator), error) {
	viper.BindPFlags(cmd.Flags())
	agent := viper.GetString("agent")

	runHeadless := viper.GetBool("headless")

	var opts []func(*chromedp.ExecAllocator)
	if !runHeadless {
		log.Println("Running without headless enabled")
		opts = []chromedp.ExecAllocatorOption{
			chromedp.UserAgent(agent),
			chromedp.NoFirstRun,
			chromedp.NoDefaultBrowserCheck,
		}
	} else {
		log.Println("Running in headless mode")
		opts = []chromedp.ExecAllocatorOption{
			chromedp.UserAgent(agent),
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
		}
	}

	return opts, nil
}

func createChromeContext(cmd *cobra.Command, opts []func(*chromedp.ExecAllocator)) (context.Context, context.CancelFunc) {
	var ctx context.Context
	var cancel context.CancelFunc

	viper.BindPFlags(cmd.Flags())
	ctx = context.Background()
	timeout := viper.GetInt("timeout")
	if timeout > 0 {
		log.Printf("Timeout specified: %ds\n", timeout)
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	}

	ctx, _ = chromedp.NewExecAllocator(ctx, opts...)
	ctx, cancel = chromedp.NewContext(ctx)

	return ctx, cancel
}

func run(cmd *cobra.Command, actions []chromedp.Action) error {
	opts, err := setOpt(cmd)
	if err != nil {
		return err
	}

	// perhaps add a check for an option that lets us reuse contexts
	// between calls - this may involved saving the first one we init
	// and reusing it in callers, but we'll leave this for now
	// as it suits most of the current use cases
	ctx, cancel := createChromeContext(cmd, opts)
	defer cancel()

	err = chromedp.Run(ctx, actions...)
	return err
}

func addWaitActions(urls []string, selectors []string, actionGens [][]actionGenerator) {
	for i, u := range urls {
		actionGens[i] = append(actionGens[i], waitActions{url: u, waitSelector: selectors[i]})
	}
}

// PrintContent fetches HTML content
func PrintContent(cmd *cobra.Command) {
	viper.BindPFlags(cmd.Flags())
	u := viper.GetString("url")
	w := viper.GetString("wait-selector")

	log.Printf("Fetching content from: %s\n", u)
	if len(w) != 0 {
		log.Printf("Waiting on selector: %s\n", w)
	}

	t := viper.GetString("text-selector")

	if len(t) != 0 {
		log.Printf("Will print text for %s\n", t)
	}

	p := make(chan dumpData)
	actionGens := make([][]actionGenerator, 0)
	actionGens = append(actionGens, make([]actionGenerator, 0))

	addWaitActions([]string{u}, []string{w}, actionGens)
	actionGens[0] = append(actionGens[0], dumpActions{postActionData: p, textSelector: t})

	executors["fetch"].Init(cmd, actionGens)
	executors["fetch"].Execute(cmd)

	data := <-p
	fmt.Printf(data.ExtractText)
}

// EmailContent will watch content and take action if content is available
func EmailContent(cmd *cobra.Command) {
	viper.BindPFlags(cmd.Flags())

	subject := viper.GetString("subject")
	from := viper.GetString("from")
	to := viper.GetString("to")

	urls := viper.GetStringSlice("urls")
	selectors := viper.GetStringSlice("wait-selectors")

	envPassword := viper.GetString("sender-password-env")
	viper.BindEnv(envPassword)
	password := viper.GetString(envPassword)

	log.Printf("Sending with subject %s\n", subject)
	log.Printf("Sending from email %s\n", from)
	log.Printf("Sending to email %s\n", to)

	log.Printf("Watching URLs %v\n", urls)
	log.Printf("Waiting on selectors %v\n", selectors)

	p := make(chan emailData)
	postAction := emailWatchFunc{
		fromEmail:      from,
		toEmail:        to,
		toSubject:      subject,
		senderPassword: password,
	}
	go func() {
		for {
			data := <-p
			postAction.Execute(data.URL)
		}
	}()

	actionGens := make([][]actionGenerator, 0)
	for i := 0; i < len(urls); i++ {
		actionGens = append(actionGens, make([]actionGenerator, 0))
	}

	addWaitActions(urls, selectors, actionGens)
	for i, u := range urls {
		actionGens[i] = append(actionGens[i], emailActions{postActionData: p, url: u})
	}

	executors["watch"].Init(cmd, actionGens)
	executors["watch"].Execute(cmd) // blocks
}
