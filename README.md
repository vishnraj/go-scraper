# Purpose
Command-line tool to fetch HTML Content from dynamic web pages using Go and chromedp

## General:
```
Allows you to request data from dynamic web pages and interact with it

Usage:
  go-dynamic-fetch [command]

Available Commands:
  fetch       Write the HTML content for the URL to stdout
  help        Help about any command

Flags:
  -a, --agent string      User agent to request as - if not specified the default is used (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36")
      --config string     config file (default is $HOME/.go-dynamic-fetch.yaml)
      --headless          Use headless shell
  -h, --help              help for go-dynamic-fetch
  -s, --selector string   Selector for element to wait for - if not specified we do not wait and just dump static elements
  -t, --timeout int       Timeout for context - if none is specified a default background context will be used (default -1)
  -u, --url string        URL that you are fetching HTML content for
```

## Fetch:
```
Fetches all content from the URL in HTML format and writes it to stdout

Usage:
  go-dynamic-fetch fetch [flags]

Flags:
  -h, --help   help for fetch

Global Flags:
  -a, --agent string      User agent to request as - if not specified the default is used (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36")
      --config string     config file (default is $HOME/.go-dynamic-fetch.yaml)
      --headless          Use headless shell
  -s, --selector string   Selector for element to wait for - if not specified we do not wait and just dump static elements
  -t, --timeout int       Timeout for context - if none is specified a default background context will be used (default -1)
  -u, --url string        URL that you are fetching HTML content for
```

## Command-line args example:
Run from within headless-shell docker image
```
go-dynamic-fetch -u "https://www.dsw.ca/en/ca/browse/sneakers/" -s ".result-list" -t "10" fetch | cascadia -i -o -c 'div.result-list__tiles' -p Name='div.product-tile__detail-text' -p Price='div.product-price' -d ','
2020/11/25 17:27:52 Fetching content from: https://www.dsw.ca/en/ca/browse/sneakers/
2020/11/25 17:27:52 Timeout specified: 10s
2020/11/25 17:27:52 Using selector: .result-list
Name,Price,
Summits Sneaker,   $69.99    ,
Ward Lo Sneaker,  $74.99 $59.98    ,
Kaptir Sneaker,   $109.99    ,
Online Only Dighton - Bricelyn Sneaker,   $74.99    ,
Anzarun Sneaker,   $79.99    ,
Women's Ward Sneaker,   $79.96    ,
Nampa - Wyola Sneaker,   $54.99    ,
Men's Atwood Sneaker,   $74.96    ,
Fruit Sneaker,   $49.99    ,
Summits Sneaker,   $69.99    ,
Men's Atwood Sneaker,   $69.99    ,
Men's Atwood Sneaker,   $74.96    
```

## Useful tools to run with:
https://github.com/suntong/cascadia