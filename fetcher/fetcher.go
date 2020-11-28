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
	DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36"
)

type watchFunc interface {
	Execute(metadata string)
}

type emailWatchFunc struct {
	senderPassword string
	fromEmail      string
	toEmail        string
	toSubject      string
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
		log.Println("Running without headless enabled")
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

func execute(cmd *cobra.Command, url string, waitSelector string, actionFunc chromedp.ActionFunc) error {
	opts, err := setOpt(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := createChromeContext(cmd, opts)
	defer cancel()

	actions := make([]chromedp.Action, 0)
	actions = append(actions, chromedp.Navigate(url))
	if len(waitSelector) != 0 {
		actions = append(actions, chromedp.WaitVisible(waitSelector, chromedp.ByQuery))
	}
	if actionFunc != nil {
		actions = append(actions, actionFunc)
	}

	err = chromedp.Run(ctx, actions...)
	return err
}

func checkAndPerformAction(cmd *cobra.Command, urls []string, selectors []string, postAction watchFunc) {
	for i := 0; i < len(urls); i++ {
		if err := execute(cmd,
			urls[i],
			selectors[i],
			nil,
		); err == nil {
			postAction.Execute(urls[i])
		} else {
			log.Printf("Data for %s was not available during this check - no email sent - received error %s\n", urls[i], err.Error())
		}
	}
}

func fetch(cmd *cobra.Command, url string, waitSelector string, textSelector string) (string, error) {
	var res string
	var actionFunc chromedp.ActionFunc

	if len(textSelector) != 0 {
		actionFunc = chromedp.ActionFunc(func(ctx context.Context) error {
			err := chromedp.Text(textSelector, &res).Do(ctx)
			return err
		})
	} else {
		actionFunc = chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return err
			}

			res, err = dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
			return err
		})
	}

	err := execute(cmd, url, waitSelector, actionFunc)
	if err != nil {
		return "", err
	}

	return res, nil
}

func watch(cmd *cobra.Command, urls []string, selectors []string, interval int, postAction watchFunc) {
	checkAndPerformAction(cmd, urls, selectors, postAction)

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	for {
		select {
		case _ = <-ticker.C:
			checkAndPerformAction(cmd, urls, selectors, postAction)
		}
	}
}

// PrintContent fetches HTML content
func PrintContent(cmd *cobra.Command) error {
	viper.BindPFlags(cmd.Flags())
	url := viper.GetString("url")
	waitSelector := viper.GetString("wait-selector")
	textSelector := viper.GetString("text-selector")

	log.Printf("Fetching content from: %s\n", url)

	if len(waitSelector) != 0 {
		log.Printf("Waiting on selector: %s\n", waitSelector)
	}
	if len(textSelector) != 0 {
		log.Printf("Will print text for %s\n", textSelector)
	}

	res, err := fetch(cmd, url, waitSelector, textSelector)
	if err == nil {
		fmt.Println(res)
	}
	return err
}

// EmailContent will watch content and take action if content is available
func EmailContent(cmd *cobra.Command) {
	viper.BindPFlags(cmd.Flags())
	urls := viper.GetStringSlice("urls")
	selectors := viper.GetStringSlice("selectors")

	subject := viper.GetString("subject")
	from := viper.GetString("from")
	to := viper.GetString("to")
	interval := viper.GetInt("interval")

	envPassword := viper.GetString("sender-password-env")
	viper.BindEnv(envPassword)
	password := viper.GetString(envPassword)

	log.Printf("Sending with subject %s\n", subject)
	log.Printf("Sending from email %s\n", from)
	log.Printf("Sending to email %s\n", to)
	log.Printf("Will check for updates every %d seconds\n", interval)

	// watch will just run until we choose to terminate
	// doesn't return content like fetch
	postAction := emailWatchFunc{
		fromEmail:      from,
		toEmail:        to,
		toSubject:      subject,
		senderPassword: password,
	}

	watch(cmd, urls, selectors, interval, postAction)
}
