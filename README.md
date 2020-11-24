# Purpose
Command-line tool to fetch HTML Content from dynamic web pages using Go and chromedp

## General usage:
```
Fetch HTML content from an endpoint for dynamic web pages that use JS

Usage:
  go-dynamic-fetch [command]

Available Commands:
  fetch       Fetch the HTML content for the URL
  help        Help about any command

Flags:
      --config string   config file (default is $HOME/.go-dynamic-fetch.yaml)
      --headless        Use headless shell
  -h, --help            help for go-dynamic-fetch

Use "go-dynamic-fetch [command] --help" for more information about a command
```

## Fetch usage:
```
go-dynamic-fetch fetch --help
Fetches all content from the URL in HTML format

Usage:
  go-dynamic-fetch fetch [flags]

Flags:
  -a, --agent string      User agent to request as - if not specified the default is used (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36")
  -h, --help              help for fetch
  -s, --selector string   Selector for element to wait for - if not specified we do not wait and just dump static elements
  -t, --timeout int       Timeout for context - if none is specified a default background context will be used (default -1)
  -u, --url string        URL that you are fetching HTML content for

Global Flags:
      --config string   config file (default is $HOME/.go-dynamic-fetch.yaml)
      --headless        Use headless shell
```

## Example:
Run from within headless-shell docker image
```
go-dynamic-fetch --headless fetch -u "https://www.dsw.ca/en/ca/browse/sneakers/" -s ".result-list" -t "10" | cascadia -i -o -c 'div.result-list__tiles' -p Name='div.product-tile__detail-text' -p Price='div.product-price' -d ','
2020/11/24 05:18:04 Fetching content from: https://www.dsw.ca/en/ca/browse/sneakers/
2020/11/24 05:18:04 Timeout specified: 10s
2020/11/24 05:18:04 Using selector: .result-list
Name,Price,
Summits Sneaker,   $69.99    ,
Ward Lo Sneaker,  $74.99 $59.98    ,
Kaptir Sneaker,   $109.99    ,
Online Only Dighton - Bricelyn Sneaker,   $74.99    ,
Anzarun Sneaker,   $79.99    ,
Women's Ward Sneaker,   $79.96    ,
```

## Useful tools to run with:
https://github.com/suntong/cascadia