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

	opts := []chromedp.ExecAllocatorOption{
		chromedp.UserAgent(agent),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
	}

	return opts, nil
}

func noWaitFetch(ctx context.Context, url string) error {
	var res string

	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return err
			}
			res, err = dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
			return err
		}),
	); err != nil {
		return err
	}

	fmt.Println(res)

	return nil
}

func waitFetch(ctx context.Context, url string, selector string) error {
	var res string

	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return err
			}
			res, err = dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
			return err
		}),
	); err != nil {
		return err
	}

	fmt.Println(res)

	return nil
}

// GetContent fetches HTML content
func GetContent(cmd *cobra.Command) error {
	opts, err := setOpt(cmd)
	if err != nil {
		return err
	}

	f := cmd.Flags()
	url, _ := f.GetString("url")
	selector, _ := f.GetString("selector")
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

	if selector == "" {
		return noWaitFetch(ctx, url)
	}

	log.Printf("Using selector: %s\n", selector)
	return waitFetch(ctx, url, selector)
}
