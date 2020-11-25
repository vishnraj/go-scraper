# Purpose
Command-line tool to interact with HTML content retrieved from dynamic web pages  
Uses the chromedp library:  
https://github.com/chromedp/chromedp

## General:
```
Allows you to request data from dynamic web pages and interact with it

Usage:
  go-dynamic-fetch [command]

Available Commands:
  fetch       Write the HTML content for the URL to stdout
  help        Help about any command

Flags:
  -a, --agent string    User agent to request as - if not specified the default is used (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36")
      --config string   config file (default is $HOME/.go-dynamic-fetch.yaml)
      --headless        Use headless shell
  -h, --help            help for go-dynamic-fetch
  -t, --timeout int     Timeout for context - if none is specified a default background context will be used (default -1)

Use "go-dynamic-fetch [command] --help" for more information about a command.
```

## Fetch:
```
Fetches all content from the URL in HTML format and writes it to stdout

Usage:
  go-dynamic-fetch fetch [flags]

Flags:
  -h, --help              help for fetch
  -s, --selector string   Selector for element to wait for - if not specified we do not wait and just dump static elements
  -u, --url string        URL that you are fetching HTML content for

Global Flags:
  -a, --agent string    User agent to request as - if not specified the default is used (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36")
      --config string   config file (default is $HOME/.go-dynamic-fetch.yaml)
      --headless        Use headless shell
  -t, --timeout int     Timeout for context - if none is specified a default background context will be used (default -1)
```

## Command-line args example:
Run from within headless-shell docker image to specify --headless  
Otherwise, it will need to run by calling into the chrome binary, which must
be installed on the machine you run this from
```
go-dynamic-fetch --headless -t "10" fetch -u "https://www.dsw.ca/en/ca/browse/sneakers/" -s ".result-list" | cascadia -i -o -c 'div.result-list__tiles' -p Name='div.product-tile__detail-text' -p Price='div.product-price' -d ','
2020/11/25 23:24:36 Fetching content from: https://www.dsw.ca/en/ca/browse/sneakers/
2020/11/25 23:24:36 Timeout specified: 10s
2020/11/25 23:24:36 Using selector: .result-list
Name,Price,
Summits Sneaker,   $69.99    ,
Ward Lo Sneaker,  $74.99 $59.98    ,
Kaptir Sneaker,   $109.99    ,
Online Only Dighton - Bricelyn Sneaker,   $74.99    ,
Anzarun Sneaker,   $79.99    ,
Women's Ward Sneaker,   $79.96    ,
```

## Useful command-line tools to run with:
https://github.com/suntong/cascadia

## Docker image to run with headless:
https://hub.docker.com/r/chromedp/headless-shell/