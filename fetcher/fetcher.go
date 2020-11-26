package fetcher

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"
)

// DefaultUserAgent The default user agent to send request as
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36"

func setOpt(cmd *cobra.Command) ([]func(*chromedp.ExecAllocator), error) {
	f := cmd.Flags()
	agent, err := f.GetString("agent")
	if err != nil {
		return nil, err
	}

	runHeadless, err := f.GetBool("headless")
	if err != nil {
		return nil, err
	}

	var opts []func(*chromedp.ExecAllocator)
	if !runHeadless {
		opts = []chromedp.ExecAllocatorOption{
			chromedp.UserAgent(agent),
			chromedp.NoFirstRun,
			chromedp.NoDefaultBrowserCheck,
		}
	} else {
		opts = []chromedp.ExecAllocatorOption{
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
		}
	}

	return opts, nil
}

func fetch(ctx context.Context, url string, waitSelector string, textSelector string) (string, error) {
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

	actions := make([]chromedp.Action, 0)
	actions = append(actions, chromedp.Navigate(url))
	if len(waitSelector) != 0 {
		actions = append(actions, chromedp.WaitVisible(waitSelector, chromedp.ByQuery))
	}
	actions = append(actions, actionFunc)

	if err := chromedp.Run(ctx,
		actions...,
	); err != nil {
		return "", err
	}

	return res, nil
}

// PrintContent fetches HTML content
func PrintContent(cmd *cobra.Command) error {
	opts, err := setOpt(cmd)
	if err != nil {
		return err
	}

	f := cmd.Flags()
	url, _ := f.GetString("url")
	waitSelector, _ := f.GetString("wait-selector")
	textSelector, _ := f.GetString("text-selector")
	timeout, _ := f.GetInt("timeout")

	log.Printf("Fetching content from: %s\n", url)

	var ctx context.Context
	var cancel context.CancelFunc
	ctx = context.Background()
	if timeout > 0 {
		log.Printf("Timeout specified: %ds\n", timeout)
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	}

	ctx, cancel = chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	if len(waitSelector) != 0 {
		log.Printf("Waiting on selector: %s\n", waitSelector)
	}
	if len(textSelector) != 0 {
		log.Printf("Will print text for %s\n", textSelector)
	}

	res, err := fetch(ctx, url, waitSelector, textSelector)
	if err == nil {
		fmt.Println(res)
	}
	return err
}
